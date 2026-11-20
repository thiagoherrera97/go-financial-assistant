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
// datou (o da fatura); compras à vista, a regra de fechamento. Quando o banco informou a fatura da transação (billId) e ela
// está entre as faturas lidas, vale o vencimento dela, sem estimativa. Num cartão em que o banco informa a fatura das
// transações, compra à vista sem fatura informada ainda não entrou em fatura fechada: se a estimativa a puser na última
// fatura fechada (compra pendente perto do fechamento), ela vai para a seguinte. Grava só o que mudou e devolve quantas
// linhas mudaram.
func (r *LedgerRepo) RecomputeCardBills(ctx context.Context, closeDaysBefore int) (int64, error) {
	query := `
		SELECT pay.id, COALESCE(pay.due_date, pay.created_at::date), p.description ~ '` + installmentSuffix + `',
		       EXTRACT(DAY FROM a.due_date)::int,
		       CASE WHEN a.close_date IS NOT NULL AND a.close_date < a.due_date THEN (a.due_date - a.close_date) END,
		       pay.bill_due_date, cb.due_date, lb.last_due
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		JOIN accounts a ON a.id = pay.account_id
		LEFT JOIN card_bills cb ON cb.external_id = pay.card_bill_id AND cb.account_id = pay.account_id
		LEFT JOIN LATERAL (
			SELECT MAX(b.due_date) AS last_due FROM card_bills b
			WHERE b.account_id = a.id
			  AND EXISTS (SELECT 1 FROM payments q WHERE q.account_id = a.id AND q.card_bill_id IS NOT NULL)
		) lb ON TRUE
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
			bankBill    *time.Time
			lastBill    *time.Time // última fatura fechada, só em cartão cujo banco informa a fatura das transações
		)
		if err := rows.Scan(&id, &day, &installment, &dueDay, &bankClose, &current, &bankBill, &lastBill); err != nil {
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
		switch {
		case bankBill != nil:
			want = bankBill.UTC()
		case !installment && lastBill != nil && sameMonth(want, *lastBill):
			want = domain.CardInstallmentBillDue(lastBill.UTC().AddDate(0, 0, -lastBill.Day()+1).AddDate(0, 1, 0), dueDay)
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

func sameMonth(a, b time.Time) bool {
	a, b = a.UTC(), b.UTC()
	return a.Year() == b.Year() && a.Month() == b.Month()
}
