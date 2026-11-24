package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// cardBillOrigin classifica a linha da fatura (aliases pay e p): ajuste, prevista, trocada, do banco ou manual.
const cardBillOrigin = `CASE
	WHEN pay.external_id LIKE '` + billGapPrefix + `%' THEN 'GAP'
	WHEN pay.external_id LIKE '` + forecastPrefix + `%' THEN 'FORECAST'
	WHEN pay.status = 'CANCELLED' THEN 'REPLACED'
	WHEN pay.external_id IS NOT NULL THEN 'BANK'
	ELSE 'MANUAL' END`

// cardBillLinesFrom são as linhas de fatura do cartão $1: despesas com fatura calculada, contando as parcelas trocadas por
// lançamento manual (o banco cobrou). A fatura é o mês do vencimento, como no ajuste da fatura (insertBillGaps).
const cardBillLinesFrom = `
	FROM payments pay
	JOIN purchases p ON p.id = pay.purchase_id`

const cardBillLinesWhere = `
	WHERE pay.account_id = $1 AND pay.bill_due_date IS NOT NULL AND p.kind = 'EXPENSE' AND ` + activeOrReplaced

// CardBills resume as faturas do cartão, da mais antiga à mais nova: as que têm lançamento no app e as fechadas pelo
// banco a partir do começo do histórico do cartão no app (fora as zeradas sem lançamento: cartão sem uso).
func (r *DebtsRepo) CardBills(ctx context.Context, cardID uuid.UUID) ([]domain.CardBillSummary, error) {
	rows, err := r.db.Pool.Query(ctx, `
		WITH lines AS (
			SELECT DATE_TRUNC('month', pay.bill_due_date)::date AS m, pay.bill_due_date AS due, pay.amount,
			       `+cardBillOrigin+` AS origin
			`+cardBillLinesFrom+cardBillLinesWhere+`
		), sums AS (
			SELECT m, MAX(due) AS due,
			       COALESCE(SUM(amount) FILTER (WHERE origin IN ('BANK', 'MANUAL', 'REPLACED')), 0) AS detailed,
			       COALESCE(SUM(amount) FILTER (WHERE origin = 'FORECAST'), 0) AS forecast,
			       COALESCE(SUM(amount) FILTER (WHERE origin = 'GAP'), 0) AS gap,
			       COUNT(*) FILTER (WHERE origin <> 'GAP') AS n
			FROM lines GROUP BY m
		), bills AS (
			SELECT DATE_TRUNC('month', due_date)::date AS m, MAX(due_date) AS due, SUM(total_amount) AS total
			FROM card_bills WHERE account_id = $1 GROUP BY 1
		)
		SELECT COALESCE(b.due, s.due), b.total, COALESCE(s.detailed, 0), COALESCE(s.forecast, 0), COALESCE(s.gap, 0),
		       COALESCE(s.n, 0)
		FROM sums s
		FULL JOIN bills b ON b.m = s.m
		WHERE COALESCE(b.m, s.m) >= (SELECT MIN(m) FROM sums)
		  AND NOT (s.m IS NULL AND b.total = 0)
		ORDER BY 1
	`, cardID)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler as faturas do cartão: %w", err)
	}
	defer rows.Close()
	var out []domain.CardBillSummary
	for rows.Next() {
		var b domain.CardBillSummary
		if err := rows.Scan(&b.Due, &b.BankTotal, &b.Detailed, &b.Forecast, &b.Gap, &b.Count); err != nil {
			return nil, fmt.Errorf("erro ao ler fatura do cartão: %w", err)
		}
		b.Due = b.Due.UTC()
		out = append(out, b)
	}
	return out, rows.Err()
}

// CardBillLines devolve as linhas da fatura do cartão que vence no mês de due, da compra mais recente à mais antiga
// (parcelas pelo dia da compra, quando se acha a parcela 1); o ajuste vai por último.
func (r *DebtsRepo) CardBillLines(ctx context.Context, cardID uuid.UUID, due time.Time) ([]domain.CardBillLine, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT pay.id, `+txDate+`, first.day, COALESCE(p.description, ''), p.category, pay.amount, pay.installment_number,
		       `+cardBillOrigin+` AS origin, pay.status = 'PENDING', COALESCE(pay.note, '')
		`+cardBillLinesFrom+`
		LEFT JOIN LATERAL (`+installmentPurchaseDay+`) first ON true
		`+cardBillLinesWhere+`
		  AND DATE_TRUNC('month', pay.bill_due_date) = DATE_TRUNC('month', $2::date)
		ORDER BY COALESCE(pay.external_id LIKE '`+billGapPrefix+`%', false), COALESCE(first.day, `+txDate+`) DESC, pay.bank_time DESC NULLS LAST, pay.created_at DESC, pay.id
	`, cardID, due)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler a fatura do cartão: %w", err)
	}
	defer rows.Close()
	var out []domain.CardBillLine
	for rows.Next() {
		var l domain.CardBillLine
		var origin string
		if err := rows.Scan(&l.ID, &l.Date, &l.PurchaseDate, &l.Description, &l.Category, &l.Amount, &l.Installment,
			&origin, &l.Pending, &l.Note); err != nil {
			return nil, fmt.Errorf("erro ao ler linha da fatura: %w", err)
		}
		l.Origin = domain.CardBillOrigin(origin)
		out = append(out, l)
	}
	return out, rows.Err()
}
