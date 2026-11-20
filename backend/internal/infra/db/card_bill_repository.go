package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// installmentSuffix reconhece a parcela do Open Finance: a sincronização acrescenta " (n/m)" à descrição.
const installmentSuffix = `\(\d+/\d+\)$`

// RecomputeCardBills calcula a fatura (bill_due_date) de cada despesa em cartão de crédito do Open Finance cujo cartão tem
// vencimento conhecido. O dia do vencimento é o da conta; o fechamento é o informado pelo banco quando houver (vencimento
// menos fechamento) e, senão, closeDaysBefore dias antes do vencimento. Parcelas usam o mês em que a sincronização já as
// datou (o da fatura); compras à vista, a regra de fechamento. Grava só o que mudou e devolve quantas linhas mudaram.
func (r *LedgerRepo) RecomputeCardBills(ctx context.Context, closeDaysBefore int) (int64, error) {
	query := `
		SELECT pay.id, COALESCE(pay.due_date, pay.created_at::date), p.description ~ '` + installmentSuffix + `',
		       EXTRACT(DAY FROM a.due_date)::int,
		       CASE WHEN a.close_date IS NOT NULL AND a.close_date < a.due_date THEN (a.due_date - a.close_date) END,
		       pay.bill_due_date
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		JOIN accounts a ON a.id = pay.account_id
		WHERE a.type = 'CREDIT' AND a.due_date IS NOT NULL AND p.kind = 'EXPENSE' AND ` + notSynthetic + `
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("erro ao ler compras no cartão: %w", err)
	}
	var ids []uuid.UUID
	var dues []time.Time
	for rows.Next() {
		var (
			id          uuid.UUID
			day         time.Time
			installment bool
			dueDay      int
			bankClose   *int
			current     *time.Time
		)
		if err := rows.Scan(&id, &day, &installment, &dueDay, &bankClose, &current); err != nil {
			rows.Close()
			return 0, fmt.Errorf("erro ao ler compra no cartão: %w", err)
		}
		closeDays := closeDaysBefore
		if bankClose != nil && *bankClose > 0 && *bankClose < 28 {
			closeDays = *bankClose
		}
		want := domain.CardBillDue(day, dueDay, closeDays)
		if installment {
			want = domain.CardInstallmentBillDue(day, dueDay)
		}
		if current == nil || !current.Equal(want) {
			ids, dues = append(ids, id), append(dues, want)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("erro ao ler compras no cartão: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE payments SET bill_due_date = u.due
		FROM unnest($1::uuid[], $2::date[]) AS u(id, due)
		WHERE payments.id = u.id
	`, ids, dues)
	if err != nil {
		return 0, fmt.Errorf("erro ao gravar a fatura das compras no cartão: %w", err)
	}
	return tag.RowsAffected(), nil
}
