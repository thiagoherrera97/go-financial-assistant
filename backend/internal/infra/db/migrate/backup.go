package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PgDump grava o backup que antecede as migrations: pg_dump no formato custom, em Dir/pre-migration-<UTC>.dump com
// permissão 600. O prefixo difere do finassist-* do serviço de backup para a retenção dele nunca apagá-lo.
type PgDump struct {
	Bin string           // padrão: pg_dump no PATH
	Dir string           // pasta dos backups (padrão "backups")
	URL string           // DATABASE_URL: nunca é registrada nem passada na linha de comando
	Now func() time.Time // padrão: relógio real
	// Logger registra o reaproveitamento de um backup existente (opcional).
	Logger *slog.Logger
}

// reuseWindow é por quanto tempo um backup da mesma tag vale como o backup "de antes": só o laço de reinício (uma migration que
// falha e o Docker que sobe o app de novo) cabe aqui. Um dump mais antigo pode ser de antes de o app ter rodado e gravado dados
// (ex.: voltou-se à imagem antiga por dias), e então não protege nada: faz-se um novo.
const reuseWindow = time.Hour

// Run faz o backup e devolve o caminho. tag (PendingTag) vai no nome: pre-migration-<UTC>-<tag>.dump. Se já existe um backup com
// essa tag, ele é reaproveitado e nenhum outro é feito (o app reinicia em laço quando uma migration falha, e cada subida faria um
// dump novo). Grava num .tmp e só renomeia se o pg_dump terminou: nunca fica um arquivo pela metade, e o .tmp não conta.
func (b PgDump) Run(ctx context.Context, tag string) (string, error) {
	if tag == "" || strings.ContainsAny(tag, `/\*?[`) {
		return "", errors.New("tag de backup inválida")
	}
	bin := b.Bin
	if bin == "" {
		var err error
		if bin, err = exec.LookPath("pg_dump"); err != nil {
			return "", errors.New("pg_dump não encontrado (a imagem do app traz o postgresql16-client)")
		}
	}
	dir := b.Dir
	if dir == "" {
		dir = "backups"
	}
	now := time.Now
	if b.Now != nil {
		now = b.Now
	}
	env, password, err := pgEnv(b.URL)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("pasta de backup %q indisponível: %w", dir, err)
	}

	// O padrão não casa com o .tmp (que começa com ponto e termina em .tmp). Vale só arquivo não vazio e recente (reuseWindow).
	sameTag, _ := filepath.Glob(filepath.Join(dir, "pre-migration-*-"+tag+".dump"))
	sort.Strings(sameTag)
	for _, path := range sameTag {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		if age := now().Sub(info.ModTime()); age >= 0 && age < reuseWindow {
			if b.Logger != nil {
				b.Logger.Info("backup já existe para estas migrations", "arquivo", path)
			}
			return path, nil
		}
	}

	stamp := now().UTC().Format("20060102-150405")
	tmp := filepath.Join(dir, ".pre-migration-"+stamp+"-"+tag+".tmp")
	out := filepath.Join(dir, "pre-migration-"+stamp+"-"+tag+".dump")

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("não foi possível criar o arquivo de backup em %q: %w", dir, err)
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(tmp)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "-Fc", "--no-owner")
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = f, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pg_dump falhou: %w: %s", err, redact(stderr.String(), password))
	}
	if err := f.Sync(); err != nil {
		return "", fmt.Errorf("erro ao gravar o backup: %w", err)
	}
	if info, err := f.Stat(); err != nil || info.Size() == 0 {
		return "", errors.New("o pg_dump não gerou conteúdo")
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, out); err != nil {
		return "", fmt.Errorf("erro ao finalizar o backup: %w", err)
	}
	ok = true
	// Fica um único arquivo por tag: os antigos da MESMA tag saem (nunca os de outras tags, nunca o novo).
	for _, old := range sameTag {
		if old != out {
			os.Remove(old)
		}
	}
	return out, nil
}

// pgEnv monta o ambiente do pg_dump a partir da DATABASE_URL, para a senha ir por PGPASSWORD e não pela linha de comando.
func pgEnv(databaseURL string) (env []string, password string, err error) {
	u, err := url.Parse(databaseURL)
	if err != nil || u.Host == "" {
		return nil, "", errors.New("DATABASE_URL inválida para o backup")
	}
	env = []string{"PATH=" + os.Getenv("PATH"), "PGHOST=" + u.Hostname(), "PGDATABASE=" + strings.TrimPrefix(u.Path, "/")}
	if p := u.Port(); p != "" {
		env = append(env, "PGPORT="+p)
	}
	if u.User != nil {
		env = append(env, "PGUSER="+u.User.Username())
		if pw, ok := u.User.Password(); ok {
			password = pw
			env = append(env, "PGPASSWORD="+pw)
		}
	}
	if mode := u.Query().Get("sslmode"); mode != "" {
		env = append(env, "PGSSLMODE="+mode)
	}
	return env, password, nil
}

// redact tira a senha de qualquer texto que vá para o log.
func redact(text, password string) string {
	if password != "" {
		text = strings.ReplaceAll(text, password, "***")
	}
	return strings.TrimSpace(text)
}
