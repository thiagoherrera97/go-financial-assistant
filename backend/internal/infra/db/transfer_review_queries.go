package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgUniqueViolation é o SQLSTATE de chave duplicada.
const pgUniqueViolation = "23505"

// transferEligible: pagamento vindo do banco (external_id), ainda contando, despesa ou renda, que o Pluggy classificou como
// transferência. O lançamento manual não tem external_id e nunca entra. O kind TRANSFER é de investimento e fica de fora.
const transferEligible = `pay.external_id IS NOT NULL AND pay.status <> 'CANCELLED' AND p.kind IN ('EXPENSE', 'INCOME')
	AND p.raw_input LIKE '[open finance: Transfer%'`

const transferItemColumns = `pay.id, ` + txDate + `, pay.amount, p.kind, COALESCE(a.name || ' ' || a.last4, ''),
	` + cleanDescription + `, p.category`

func (r *TransferReviewRepo) PendingTransfers(ctx context.Context, limit int) ([]domain.TransferReviewItem, int, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT `+transferItemColumns+`, COUNT(*) OVER ()
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		LEFT JOIN accounts a ON a.id = pay.account_id
		WHERE `+transferEligible+` AND NOT EXISTS (SELECT 1 FROM transfer_decisions d WHERE d.payment_id = pay.id)
		ORDER BY `+txDate+` DESC, pay.created_at DESC, pay.id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao listar transferências a revisar: %w", err)
	}
	defer rows.Close()
	var (
		items []domain.TransferReviewItem
		total int
	)
	for rows.Next() {
		var it domain.TransferReviewItem
		if err := rows.Scan(&it.PaymentID, &it.Date, &it.Amount, &it.Kind, &it.Account, &it.Description, &it.Category, &total); err != nil {
			return nil, 0, fmt.Errorf("erro ao ler transferência a revisar: %w", err)
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// DecidedTransfers não repete o filtro de elegibilidade: a transferência "entre contas minhas" está CANCELLED e precisa aparecer.
func (r *TransferReviewRepo) DecidedTransfers(ctx context.Context, limit int) ([]domain.TransferReviewItem, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT `+transferItemColumns+`, d.decision, d.decided_at
		FROM transfer_decisions d
		JOIN payments pay ON pay.id = d.payment_id
		JOIN purchases p ON p.id = pay.purchase_id
		LEFT JOIN accounts a ON a.id = pay.account_id
		ORDER BY d.decided_at DESC, pay.id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar transferências decididas: %w", err)
	}
	defer rows.Close()
	var items []domain.TransferReviewItem
	for rows.Next() {
		var it domain.TransferReviewItem
		if err := rows.Scan(&it.PaymentID, &it.Date, &it.Amount, &it.Kind, &it.Account, &it.Description, &it.Category, &it.Decision, &it.DecidedAt); err != nil {
			return nil, fmt.Errorf("erro ao ler transferência decidida: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *TransferReviewRepo) DecideTransfer(ctx context.Context, paymentID uuid.UUID, decision domain.TransferDecision, category string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao decidir transferência: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// A trava no pagamento evita duas decisões ao mesmo tempo sobre ele; a na compra, que outra tela (regra de categoria)
	// troque a categoria entre a leitura do previous_category e o UPDATE.
	var purchaseID uuid.UUID
	var status, prevCategory, kind string
	err = tx.QueryRow(ctx, `
		SELECT p.id, pay.status, p.category, p.kind
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE pay.id = $1 AND `+transferEligible+` AND NOT EXISTS (SELECT 1 FROM transfer_decisions d WHERE d.payment_id = pay.id)
		FOR UPDATE OF pay, p
	`, paymentID).Scan(&purchaseID, &status, &prevCategory, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTransferNotPending
	}
	if err != nil {
		return fmt.Errorf("erro ao ler a transferência: %w", err)
	}

	if decision == domain.TransferPayment && kind == "INCOME" {
		return domain.ErrTransferPaymentOnIncome
	}
	var applied *string
	if c := decision.AppliedCategory(category); c != "" {
		applied = &c
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO transfer_decisions (payment_id, decision, previous_status, previous_category, applied_category)
		VALUES ($1, $2, $3, $4, $5)
	`, paymentID, string(decision), status, prevCategory, applied); err != nil {
		// Duas decisões ao mesmo tempo: a trava não revê o NOT EXISTS (em READ COMMITTED a subconsulta usa o snapshot do
		// início), então a segunda chega aqui e bate na chave primária. Para ela o pagamento já foi decidido.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return domain.ErrTransferNotPending
		}
		return fmt.Errorf("erro ao gravar a decisão: %w", err)
	}

	switch decision {
	case domain.TransferOwn:
		_, err = tx.Exec(ctx, `UPDATE payments SET status = 'CANCELLED' WHERE id = $1`, paymentID)
	case domain.TransferPerson:
		_, err = tx.Exec(ctx, `UPDATE purchases SET category = 'PEOPLE' WHERE id = $1`, purchaseID)
	case domain.TransferPayment:
		_, err = tx.Exec(ctx, `UPDATE purchases SET category = $2 WHERE id = $1`, purchaseID, category)
	default:
		return fmt.Errorf("decisão inválida: %q", decision)
	}
	if err != nil {
		return fmt.Errorf("erro ao aplicar a decisão: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *TransferReviewRepo) UndoTransferDecision(ctx context.Context, paymentID uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao desfazer a decisão: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Trava a decisão, o pagamento e a compra: nada muda o status ou a categoria entre a leitura e a restauração.
	var (
		purchaseID                    uuid.UUID
		decision, prevStatus, prevCat string
		applied                       *string
	)
	err = tx.QueryRow(ctx, `
		SELECT d.decision, d.previous_status, d.previous_category, d.applied_category, p.id
		FROM transfer_decisions d
		JOIN payments pay ON pay.id = d.payment_id
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE d.payment_id = $1
		FOR UPDATE OF d, pay, p
	`, paymentID).Scan(&decision, &prevStatus, &prevCat, &applied, &purchaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrTransferNoDecision
	}
	if err != nil {
		return fmt.Errorf("erro ao ler a decisão: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM transfer_decisions WHERE payment_id = $1`, paymentID); err != nil {
		return fmt.Errorf("erro ao apagar a decisão: %w", err)
	}

	// Só desfaz o que a decisão fez e ninguém mudou depois: OWN só mexe no status, e só se ele ainda é CANCELLED;
	// PERSON e PAYMENT só mexem na categoria, e só se ela ainda é a aplicada. Ex.: a limpeza do OWN_NAMES cancelou depois
	// um "para pessoa": desfazer não o reativa.
	switch domain.TransferDecision(decision) {
	case domain.TransferOwn:
		if _, err := tx.Exec(ctx, `UPDATE payments SET status = $2 WHERE id = $1 AND status = 'CANCELLED'`, paymentID, prevStatus); err != nil {
			return fmt.Errorf("erro ao restaurar o status: %w", err)
		}
	default:
		if applied != nil {
			if _, err := tx.Exec(ctx, `UPDATE purchases SET category = $2 WHERE id = $1 AND category = $3`, purchaseID, prevCat, *applied); err != nil {
				return fmt.Errorf("erro ao restaurar a categoria: %w", err)
			}
		}
	}
	return tx.Commit(ctx)
}
