package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/jackc/pgx/v5"
)

type paymentDetailModel struct {
	Description       *string    `db:"description"`
	Category          string     `db:"category"`
	PaymentMethod     string     `db:"payment_method"`
	Amount            float64    `db:"amount"`
	Status            string     `db:"status"`
	PurchaseType      string     `db:"purchase_type"`
	PurchaseKind      string     `db:"purchase_kind"`
	TransferDirection *string    `db:"transfer_direction"`
	InstallmentNumber *int       `db:"installment_number"`
	DueDate           *time.Time `db:"due_date"`
	ReferenceMonth    *time.Time `db:"reference_month"`
	CreatedAt         time.Time  `db:"created_at"`
}

func (m paymentDetailModel) toPort() ports.PaymentDetail {
	dir := ""
	if m.TransferDirection != nil {
		dir = *m.TransferDirection
	}
	return ports.PaymentDetail{
		Description:       m.Description,
		Category:          m.Category,
		PaymentMethod:     m.PaymentMethod,
		Amount:            m.Amount,
		Status:            m.Status,
		PurchaseType:      m.PurchaseType,
		PurchaseKind:      m.PurchaseKind,
		TransferDirection: dir,
		InstallmentNumber: m.InstallmentNumber,
		DueDate:           m.DueDate,
		ReferenceMonth:    m.ReferenceMonth,
		CreatedAt:         m.CreatedAt,
	}
}

func (r *LedgerRepo) ExistsPaymentByDateAndAmount(ctx context.Context, date time.Time, amount float64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM payments
			WHERE ABS(amount - $2) < 0.001
			  AND due_date IS NOT NULL
			  AND DATE_TRUNC('day', due_date) = DATE_TRUNC('day', $1::date)
			  AND status != 'CANCELLED'
		)
	`
	var exists bool
	if err := r.db.Pool.QueryRow(ctx, query, date, amount).Scan(&exists); err != nil {
		return false, fmt.Errorf("erro ao verificar duplicata: %w", err)
	}
	return exists, nil
}

func (r *LedgerRepo) SaveRecurringPaymentIfMissing(ctx context.Context, payment *domain.Payment) (bool, error) {
	if payment.ReferenceMonth == nil {
		return false, errors.New("pagamento recorrente sem mês de referência")
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// A trava na recorrente serializa duas execuções sobre ela (subida e meia-noite, ou duas instâncias): a segunda espera
	// a primeira terminar e então já vê o pagamento. Também pega um cancelamento feito depois da lista de ativas.
	var active bool
	err = tx.QueryRow(ctx, `SELECT is_active FROM purchases WHERE id = $1 AND type = 'RECURRING' FOR UPDATE`, payment.PurchaseID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("erro ao travar a recorrente: %w", err)
	}

	// Qualquer pagamento do mês conta, de qualquer status: um CANCELLED foi uma decisão da pessoa.
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE purchase_id = $1 AND reference_month = $2)`,
		payment.PurchaseID, payment.ReferenceMonth).Scan(&exists); err != nil {
		return false, fmt.Errorf("erro ao verificar pagamento do mês: %w", err)
	}
	if exists {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payments
			(id, purchase_id, amount, status, installment_number, due_date, reference_month, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, payment.ID, payment.PurchaseID, payment.Amount, payment.Status,
		payment.InstallmentNumber, payment.DueDate, payment.ReferenceMonth, payment.PaidAt, payment.CreatedAt,
	); err != nil {
		return false, fmt.Errorf("erro ao salvar pagamento recorrente: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("erro ao salvar pagamento recorrente: %w", err)
	}
	return true, nil
}

// purchaseColumns são as colunas de purchaseModel, na ordem da struct: pgx.RowToStructByName exige todas, e faltar uma
// (ex.: kind) faz a leitura falhar com "cannot find field" sempre que há linha.
const purchaseColumns = `id, description, category, payment_method, kind, transfer_direction, type, total_amount,
		       installment_count, installment_amount, day_of_month, is_active,
		       cancelled_at, cancellation_reason, raw_input, created_at`

func (r *LedgerRepo) FindActiveRecurring(ctx context.Context) ([]domain.Purchase, error) {
	query := `
		SELECT ` + purchaseColumns + `
		FROM purchases
		WHERE type = 'RECURRING' AND is_active = TRUE
		ORDER BY description
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar despesas recorrentes ativas: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[purchaseModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear compras: %w", err)
	}

	result := make([]domain.Purchase, len(models))
	for i, m := range models {
		result[i] = m.toDomain()
	}
	return result, nil
}

func (r *LedgerRepo) FindByDescription(ctx context.Context, description string) ([]domain.Purchase, error) {
	query := `
		SELECT ` + purchaseColumns + `
		FROM purchases
		WHERE type = 'RECURRING' AND is_active = TRUE AND description ILIKE $1
		ORDER BY description
	`
	rows, err := r.db.Pool.Query(ctx, query, "%"+description+"%")
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar compras por descrição: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[purchaseModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear compras: %w", err)
	}

	result := make([]domain.Purchase, len(models))
	for i, m := range models {
		result[i] = m.toDomain()
	}
	return result, nil
}

func (r *LedgerRepo) FindPaymentsByMonth(ctx context.Context, month time.Time) ([]ports.PaymentSummary, error) {
	query := `
		SELECT p.category, SUM(pay.amount) AS total
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month, pay.created_at)) = DATE_TRUNC('month', $1::date)
		  AND pay.status != 'CANCELLED'
		  AND p.kind = 'EXPENSE'
		GROUP BY p.category
		ORDER BY total DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, month)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas do mês: %w", err)
	}
	defer rows.Close()

	var result []ports.PaymentSummary
	for rows.Next() {
		var s ports.PaymentSummary
		if err := rows.Scan(&s.Category, &s.Total); err != nil {
			return nil, fmt.Errorf("erro ao escanear resumo: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *LedgerRepo) FindPaymentDetailsByMonth(ctx context.Context, month time.Time) ([]ports.PaymentDetail, error) {
	query := `
		SELECT
		    p.description,
		    p.category,
		    p.payment_method,
		    pay.amount,
		    pay.status,
		    p.type AS purchase_type,
		    p.kind AS purchase_kind,
		    p.transfer_direction,
		    pay.installment_number,
		    pay.due_date,
		    pay.reference_month,
		    pay.created_at
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month, pay.created_at)) = DATE_TRUNC('month', $1::date)
		  AND pay.status != 'CANCELLED'
		ORDER BY COALESCE(pay.due_date, pay.reference_month, pay.created_at)
	`
	rows, err := r.db.Pool.Query(ctx, query, month)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar detalhes do mês: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[paymentDetailModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear detalhes: %w", err)
	}

	result := make([]ports.PaymentDetail, len(models))
	for i, m := range models {
		result[i] = m.toPort()
	}
	return result, nil
}
