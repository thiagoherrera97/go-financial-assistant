// Package migrate aplica as migrations SQL embutidas no binário ao subir o app.
//
// Garantias: uma instância por vez (advisory lock), cada migration numa transação junto com o seu registro em
// schema_migrations (ou ela inteira ou nada), e backup do banco antes de aplicar qualquer pendente. Os scripts precisam ser
// idempotentes: num banco criado pelo initdb (sem schema_migrations) a primeira subida os reaplica, sem efeito, e registra.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrBackup indica que havia migrations pendentes e o backup antes delas falhou. Nada foi aplicado e o banco está como estava.
var ErrBackup = errors.New("backup antes das migrations falhou")

// lockName identifica o advisory lock das migrations (hashtext vira o número do lock).
const lockName = "finassist.schema_migrations"

type Migration struct {
	Version  string // nome do arquivo sem .sql
	SQL      string
	Checksum string
}

type Result struct {
	Applied []string // versões aplicadas agora, em ordem
	Backup  string   // caminho do backup feito antes (vazio se não houve)
}

type Runner struct {
	Pool  *pgxpool.Pool
	Files []Migration
	// Backup grava um backup e devolve o caminho. tag identifica o conjunto de migrations pendentes (PendingTag): o mesmo
	// conjunto reaproveita o backup que já existe, para um app que reinicia em laço com uma migration falhando não encher o
	// disco. nil não faz backup (só testes).
	Backup func(ctx context.Context, tag string) (string, error)
	Logger *slog.Logger
}

// Load lê os scripts *.sql de fsys, em ordem de nome. Recusa o que não pode rodar dentro de uma transação.
func Load(fsys fs.FS) ([]Migration, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := make([]Migration, 0, len(names))
	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("erro ao ler %s: %w", name, err)
		}
		if concurrently.MatchString(stripComments(string(raw))) {
			return nil, fmt.Errorf("%s: CONCURRENTLY não roda dentro de transação", name)
		}
		sum := sha256.Sum256(raw)
		out = append(out, Migration{Version: strings.TrimSuffix(name, ".sql"), SQL: string(raw), Checksum: hex.EncodeToString(sum[:])})
	}
	return out, nil
}

var (
	concurrently = regexp.MustCompile(`(?i)\bconcurrently\b`)
	lineComment  = regexp.MustCompile(`--[^\n]*`)
	blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

func stripComments(sql string) string {
	return lineComment.ReplaceAllString(blockComment.ReplaceAllString(sql, ""), "")
}

// Run aplica as migrations pendentes. Com o lock tomado: cria schema_migrations, vê o que falta, faz o backup (só se
// houver pendentes e o banco não estiver vazio) e aplica uma a uma. A falha de uma interrompe tudo, com o arquivo no erro.
func (r *Runner) Run(ctx context.Context) (Result, error) {
	var res Result
	conn, err := r.Pool.Acquire(ctx)
	if err != nil {
		return res, fmt.Errorf("erro ao obter conexão para as migrations: %w", err)
	}
	locked := false
	defer func() { r.release(ctx, conn, locked) }()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1))`, lockName); err != nil {
		return res, fmt.Errorf("erro ao tomar o lock das migrations: %w", err)
	}
	locked = true

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT        PRIMARY KEY,
			checksum   TEXT        NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return res, fmt.Errorf("erro ao criar schema_migrations: %w", err)
	}

	applied, err := r.appliedVersions(ctx, conn)
	if err != nil {
		return res, err
	}
	var pending []Migration
	for _, m := range r.Files {
		sum, done := applied[m.Version]
		switch {
		case !done:
			pending = append(pending, m)
		case sum != m.Checksum:
			// O script mudou depois de aplicado. Não reaplica: mudança de esquema nova vai numa migration nova.
			r.Logger.Warn("migration já aplicada foi alterada; não será reaplicada", "migration", m.Version)
		}
	}
	if len(pending) == 0 {
		return res, nil
	}

	if r.Backup != nil {
		empty, err := r.isEmpty(ctx, conn)
		if err != nil {
			return res, err
		}
		if !empty {
			path, err := r.Backup(ctx, PendingTag(pending))
			if err != nil {
				return res, fmt.Errorf("%w: %w", ErrBackup, err)
			}
			res.Backup = path
			r.Logger.Info("backup antes das migrations gravado", "arquivo", path)
		}
	}

	for _, m := range pending {
		if err := r.apply(ctx, conn, m); err != nil {
			return res, fmt.Errorf("migration %s: %w", m.Version, err)
		}
		res.Applied = append(res.Applied, m.Version)
		r.Logger.Info("migration aplicada", "migration", m.Version)
	}
	return res, nil
}

func (r *Runner) appliedVersions(ctx context.Context, conn *pgxpool.Conn) (map[string]string, error) {
	rows, err := conn.Query(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler schema_migrations: %w", err)
	}
	defer rows.Close()
	applied := map[string]string{}
	for rows.Next() {
		var v, c string
		if err := rows.Scan(&v, &c); err != nil {
			return nil, fmt.Errorf("erro ao ler schema_migrations: %w", err)
		}
		applied[v] = c
	}
	return applied, rows.Err()
}

// isEmpty diz se o banco ainda não tem nenhuma tabela (fora a de controle): não há o que proteger com um backup.
func (r *Runner) isEmpty(ctx context.Context, conn *pgxpool.Conn) (bool, error) {
	var n int
	err := conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name <> 'schema_migrations'`).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("erro ao verificar se o banco está vazio: %w", err)
	}
	return n == 0, nil
}

// apply roda o script e registra a versão na mesma transação: se um falhar, nada fica.
func (r *Runner) apply(ctx context.Context, conn *pgxpool.Conn, m Migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // no-op depois do commit
	// Sem argumentos o pgx usa o protocolo simples, que aceita o script com vários comandos.
	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`, m.Version, m.Checksum); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// release solta o lock e devolve a conexão ao pool. Se o unlock falhar numa conexão viva, o lock (que é da sessão) ficaria
// preso a uma conexão reaproveitada: nesse caso a conexão é fechada, o que o libera.
func (r *Runner) release(ctx context.Context, conn *pgxpool.Conn, locked bool) {
	if locked {
		if _, err := conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(hashtext($1))`, lockName); err != nil {
			r.Logger.Warn("não foi possível soltar o lock das migrations; fechando a conexão", "error", err)
			_ = conn.Hijack().Close(context.WithoutCancel(ctx))
			return
		}
	}
	conn.Release()
}

// PendingTag resume o conjunto de migrations pendentes em 8 caracteres: sha256 da lista ordenada de "versão:checksum".
// Corrigir uma migration muda o checksum e, portanto, a tag.
func PendingTag(pending []Migration) string {
	lines := make([]string, len(pending))
	for i, m := range pending {
		lines[i] = m.Version + ":" + m.Checksum
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])[:8]
}
