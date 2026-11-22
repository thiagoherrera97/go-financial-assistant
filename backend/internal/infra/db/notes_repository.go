package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// NotesRepo acha transações pelo valor e pelo dia, para a nota mandada pelo chat, e guarda a nota que chegou antes da
// transação.
type NotesRepo struct{ db *DB }

func NewNotesRepo(db *DB) *NotesRepo { return &NotesRepo{db: db} }

// NoteCandidates devolve as transações (não canceladas, fora as criadas pelo app) com esse valor, de qualquer sinal, com
// data de from a to (dias, inclusive), da mais recente para a mais antiga.
func (r *NotesRepo) NoteCandidates(ctx context.Context, amount float64, from, to time.Time) ([]domain.NoteCandidate, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT pay.id, `+txDate+`::date, COALESCE(`+cleanDescription+`, ''), pay.amount, p.kind,
		       COALESCE(a.name, ''), COALESCE(pay.note, '')
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		LEFT JOIN accounts a ON a.id = pay.account_id
		WHERE pay.status != 'CANCELLED' AND `+notSynthetic+`
		  AND ABS(ABS(pay.amount) - $1) < 0.005
		  AND `+txDate+`::date BETWEEN $2::date AND $3::date
		ORDER BY 2 DESC, pay.created_at DESC
		LIMIT 8
	`, amount, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao procurar a transação da nota: %w", err)
	}
	defer rows.Close()
	var out []domain.NoteCandidate
	for rows.Next() {
		var c domain.NoteCandidate
		if err := rows.Scan(&c.PaymentID, &c.Date, &c.Description, &c.Amount, &c.Kind, &c.AccountName, &c.Note); err != nil {
			return nil, fmt.Errorf("erro ao ler a transação da nota: %w", err)
		}
		c.Date = c.Date.UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetTransactionNote grava a nota da transação (vazia apaga).
func (r *NotesRepo) SetTransactionNote(ctx context.Context, id uuid.UUID, note string) (bool, error) {
	return (&TransactionsRepo{db: r.db}).SetTransactionNote(ctx, id, note)
}

// SavePendingNote guarda a nota de uma transação que ainda não chegou do banco.
func (r *NotesRepo) SavePendingNote(ctx context.Context, amount float64, day time.Time, note string) error {
	if _, err := r.db.Pool.Exec(ctx, `INSERT INTO pending_notes (id, amount, day, note) VALUES ($1, $2, $3, $4)`,
		uuid.New(), amount, day, note); err != nil {
		return fmt.Errorf("erro ao guardar a nota pendente: %w", err)
	}
	return nil
}

// ApplyPendingNotes coloca cada nota pendente na transação que chegou: mesmo valor, até PendingNoteMatchDays dias do dia
// dito e ainda sem nota. Só aplica quando há exatamente uma candidata (na dúvida, a nota continua esperando). Notas mais
// velhas que PendingNoteMaxAge são descartadas. Devolve quantas foram aplicadas.
func (r *LedgerRepo) ApplyPendingNotes(ctx context.Context) (int, error) {
	if _, err := r.db.Pool.Exec(ctx, `DELETE FROM pending_notes WHERE created_at < $1`, time.Now().Add(-domain.PendingNoteMaxAge)); err != nil {
		return 0, fmt.Errorf("erro ao limpar notas pendentes: %w", err)
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT n.id, n.note, (
			SELECT ARRAY_AGG(pay.id) FROM payments pay
			WHERE pay.status != 'CANCELLED' AND `+notSynthetic+` AND COALESCE(pay.note, '') = ''
			  AND ABS(ABS(pay.amount) - n.amount) < 0.005
			  AND ABS(`+txDate+`::date - n.day) <= $1
		)
		FROM pending_notes n ORDER BY n.created_at
	`, domain.PendingNoteMatchDays)
	if err != nil {
		return 0, fmt.Errorf("erro ao ler notas pendentes: %w", err)
	}
	type hit struct {
		note    uuid.UUID
		text    string
		payment uuid.UUID
	}
	var hits []hit
	for rows.Next() {
		var h hit
		var matches []uuid.UUID
		if err := rows.Scan(&h.note, &h.text, &matches); err != nil {
			rows.Close()
			return 0, fmt.Errorf("erro ao ler nota pendente: %w", err)
		}
		if len(matches) == 1 {
			h.payment = matches[0]
			hits = append(hits, h)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("erro ao ler notas pendentes: %w", err)
	}
	applied := 0
	for _, h := range hits {
		tx, err := r.db.Pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("erro ao aplicar nota pendente: %w", err)
		}
		tag, err := tx.Exec(ctx, `UPDATE payments SET note = $2 WHERE id = $1 AND COALESCE(note, '') = ''`, h.payment, h.text)
		if err == nil && tag.RowsAffected() == 1 {
			_, err = tx.Exec(ctx, `DELETE FROM pending_notes WHERE id = $1`, h.note)
			if err == nil {
				applied++
			}
		}
		if err != nil {
			tx.Rollback(ctx) //nolint:errcheck
			return applied, fmt.Errorf("erro ao aplicar nota pendente: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("erro ao aplicar nota pendente: %w", err)
		}
	}
	if _, err := inheritNotes(ctx, r.db.Pool, ""); err != nil {
		return applied, err
	}
	return applied, nil
}

// learnedNoteShare é a parte mínima (%) das notas de um comércio e valor que precisa ser a mesma para ela ser herdada.
const learnedNoteShare = 60

// inheritNotes dá nota a quem nunca teve (nota nula; apagada pela pessoa fica vazia e é respeitada): a transação herda a
// nota mais usada nas outras do mesmo tipo, comércio (expenseKey: some número e parcela) e valor, quando ela (sem
// diferenciar maiúsculas) é pelo menos learnedNoteShare% das notas dali (o estacionamento do trabalho, com uma ou outra
// nota diferente; as parcelas que chegam depois). Abaixo disso há dúvida, e nada é herdado. key vazia olha todos os
// comércios. Devolve quantas ganharam nota.
func inheritNotes(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, key string) (int64, error) {
	tag, err := q.Exec(ctx, `
		WITH tx AS (
			SELECT pay.id, pay.note, p.kind, ROUND(ABS(pay.amount)::numeric, 2) AS amount, `+expenseKey+` AS key
			FROM payments pay JOIN purchases p ON p.id = pay.purchase_id
			WHERE pay.status != 'CANCELLED'
		), noted AS (
			SELECT kind, key, amount, note,
			       COUNT(*) OVER (PARTITION BY kind, key, amount, LOWER(note)) AS same,
			       COUNT(*) OVER (PARTITION BY kind, key, amount, note) AS exact,
			       COUNT(*) OVER (PARTITION BY kind, key, amount) AS total
			FROM tx WHERE note <> '' AND key <> '' AND ($1 = '' OR key = $1)
		), learned AS (
			SELECT DISTINCT ON (kind, key, amount) kind, key, amount, note FROM noted
			WHERE same * 100 >= total * `+fmt.Sprint(learnedNoteShare)+`
			ORDER BY kind, key, amount, same DESC, exact DESC, note
		)
		UPDATE payments pay SET note = l.note
		FROM tx JOIN learned l ON l.kind = tx.kind AND l.key = tx.key AND l.amount = tx.amount
		WHERE pay.id = tx.id AND tx.note IS NULL
	`, key)
	if err != nil {
		return 0, fmt.Errorf("erro ao repetir as notas aprendidas: %w", err)
	}
	return tag.RowsAffected(), nil
}
