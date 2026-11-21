package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
	"strings"
)

const debtColumns = `id, kind, name, creditor, note, original_amount, due_day, account_id, free_days,
	COALESCE(installment_amount, 0), COALESCE(installments, 0), COALESCE(first_due, '0001-01-01'), payee_match, paid_manual, created_at`

func (r *DebtsRepo) Debts(ctx context.Context) ([]domain.Debt, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT `+debtColumns+` FROM debts ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler dívidas: %w", err)
	}
	defer rows.Close()
	var result []domain.Debt
	for rows.Next() {
		var d domain.Debt
		if err := rows.Scan(&d.ID, &d.Kind, &d.Name, &d.Creditor, &d.Note, &d.OriginalAmount, &d.DueDay, &d.AccountID, &d.FreeDays,
			&d.InstallmentAmount, &d.Installments, &d.FirstDue, &d.PayeeMatch, &d.PaidManual, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("erro ao escanear dívida: %w", err)
		}
		d.FirstDue = d.FirstDue.UTC()
		if d.Kind != domain.DebtInstallments {
			d.FirstDue = time.Time{}
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// debtArgs grava só os campos do tipo da dívida; os demais ficam nulos.
func debtArgs(d domain.Debt) (amount *float64, count *int, first *time.Time) {
	if d.Kind == domain.DebtInstallments {
		return &d.InstallmentAmount, &d.Installments, &d.FirstDue
	}
	return nil, nil, nil
}

func (r *DebtsRepo) CreateDebt(ctx context.Context, d domain.Debt) error {
	amount, count, first := debtArgs(d)
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO debts (id, kind, name, creditor, note, original_amount, due_day, account_id, free_days,
			installment_amount, installments, first_due, payee_match, paid_manual)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, d.ID, string(d.Kind), d.Name, d.Creditor, d.Note, d.OriginalAmount, d.DueDay, d.AccountID, d.FreeDays,
		amount, count, first, d.PayeeMatch, d.PaidManual)
	if err != nil {
		return fmt.Errorf("erro ao gravar dívida: %w", err)
	}
	return nil
}

func (r *DebtsRepo) UpdateDebt(ctx context.Context, d domain.Debt) (bool, error) {
	amount, count, first := debtArgs(d)
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE debts SET kind = $2, name = $3, creditor = $4, note = $5, original_amount = $6, due_day = $7, account_id = $8,
			free_days = $9, installment_amount = $10, installments = $11, first_due = $12, payee_match = $13, paid_manual = $14
		WHERE id = $1
	`, d.ID, string(d.Kind), d.Name, d.Creditor, d.Note, d.OriginalAmount, d.DueDay, d.AccountID, d.FreeDays,
		amount, count, first, d.PayeeMatch, d.PaidManual)
	if err != nil {
		return false, fmt.Errorf("erro ao atualizar dívida: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *DebtsRepo) DeleteDebt(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM debts WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("erro ao apagar dívida: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// bankOutflows são as saídas de conta corrente (Pix, TED, boleto), sem as transferências entre contas próprias.
const bankOutflows = `
	SELECT ` + txDate + `::date, pay.amount, COALESCE(p.description, '')
	FROM payments pay
	JOIN purchases p ON p.id = pay.purchase_id
	JOIN accounts a ON a.id = pay.account_id AND a.type = 'BANK'
	WHERE p.kind = 'EXPENSE' AND pay.status != 'CANCELLED' AND pay.amount > 0`

func (r *DebtsRepo) PaymentsMatching(ctx context.Context, match string, from time.Time) ([]domain.MoneyMovement, error) {
	if match == "" {
		return nil, nil
	}
	want := domain.Fold(match)
	rows, err := r.movements(ctx, bankOutflows+` AND `+txDate+`::date >= $1::date ORDER BY 1`, from)
	if err != nil {
		return nil, err
	}
	var result []domain.MoneyMovement
	for _, m := range rows {
		// O filtro é em Go para ignorar acentos e maiúsculas do mesmo jeito que no resto do app.
		if strings.Contains(domain.Fold(m.Label), want) {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *DebtsRepo) AccountOutflows(ctx context.Context, accountID uuid.UUID, from, to time.Time) ([]domain.MoneyMovement, error) {
	return r.movements(ctx, bankOutflows+` AND pay.account_id = $1 AND `+txDate+`::date BETWEEN $2::date AND $3::date ORDER BY 1`,
		accountID, from, to)
}

func (r *DebtsRepo) movements(ctx context.Context, query string, args ...any) ([]domain.MoneyMovement, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler saídas da conta: %w", err)
	}
	defer rows.Close()
	var result []domain.MoneyMovement
	for rows.Next() {
		var m domain.MoneyMovement
		if err := rows.Scan(&m.Date, &m.Amount, &m.Label); err != nil {
			return nil, fmt.Errorf("erro ao escanear saída da conta: %w", err)
		}
		m.Date = m.Date.UTC()
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *DebtsRepo) BalanceDays(ctx context.Context, accountID uuid.UUID, from time.Time) ([]domain.BalanceDay, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT day, min_balance, last_balance FROM account_balance_days
		WHERE account_id = $1 AND day >= $2::date ORDER BY day
	`, accountID, from)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler saldos diários: %w", err)
	}
	defer rows.Close()
	var result []domain.BalanceDay
	for rows.Next() {
		var d domain.BalanceDay
		if err := rows.Scan(&d.Day, &d.Min, &d.Last); err != nil {
			return nil, fmt.Errorf("erro ao escanear saldo diário: %w", err)
		}
		d.Day = d.Day.UTC()
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *DebtsRepo) CardBillTotal(ctx context.Context, cardID uuid.UUID, due time.Time) (float64, error) {
	var total float64
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM payments
		WHERE account_id = $1 AND bill_due_date = $2::date AND status != 'CANCELLED'
	`, cardID, due).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("erro ao somar a fatura: %w", err)
	}
	return total, nil
}
