package db

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Lançamentos criados pelo próprio app nos cartões, marcados pelo external_id: parcelas previstas e o ajuste da fatura.
// São refeitos a cada sincronização (apagados e recriados), nunca editados.
const (
	syntheticPrefix = "finassist:"
	forecastPrefix  = syntheticPrefix + "forecast:"
	billGapPrefix   = syntheticPrefix + "bill:"
)

// notSynthetic filtra os lançamentos que vieram do banco ou foram anotados (fora os criados pelo app).
const notSynthetic = `(pay.external_id IS NULL OR pay.external_id NOT LIKE '` + syntheticPrefix + `%')`

var installmentDesc = regexp.MustCompile(`^(.*?)\s*\((\d+)/(\d+)\)$`)

// SaveCardBills grava (ou atualiza) as faturas fechadas do cartão informadas pelo banco.
func (r *LedgerRepo) SaveCardBills(ctx context.Context, accountID uuid.UUID, bills []ports.ExternalBill) error {
	for _, b := range bills {
		if _, err := r.db.Pool.Exec(ctx, `
			INSERT INTO card_bills (external_id, account_id, due_date, total_amount, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (external_id) DO UPDATE SET account_id = EXCLUDED.account_id, due_date = EXCLUDED.due_date,
				total_amount = EXCLUDED.total_amount, updated_at = NOW()
		`, b.ID, accountID, b.DueDate, b.Total); err != nil {
			return fmt.Errorf("erro ao salvar fatura do cartão: %w", err)
		}
	}
	return nil
}

// RebuildCardForecasts refaz os lançamentos que o app cria nos cartões, numa transação:
//   - parcelas previstas: as parcelas seguintes das compras parceladas que ainda não chegaram do banco, até a fatura
//     de daqui a domain.CardForecastMonthsAhead meses (domain.ForecastCardInstallments);
//   - ajuste da fatura: para cada fatura fechada informada pelo banco, o que ela cobrou além do que o app conhece
//     (reais + previstas) vira um lançamento "não detalhado no Open Finance".
//
// Tudo o que o app criou antes é apagado e recriado; o que veio do banco não é tocado.
func (r *LedgerRepo) RebuildCardForecasts(ctx context.Context, closeDaysBefore int, today time.Time) (forecasts, adjustments int, err error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// As notas escritas nesses lançamentos (ex.: o que compõe o ajuste da fatura) voltam depois de recriá-los: o
	// external_id de cada um é estável (id da fatura, chave da parcela), então a nota acha o mesmo lançamento.
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE synthetic_notes ON COMMIT DROP AS
		SELECT external_id, note FROM payments WHERE external_id LIKE '`+syntheticPrefix+`%' AND note IS NOT NULL`); err != nil {
		return 0, 0, fmt.Errorf("erro ao guardar as notas das previsões: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM purchases WHERE id IN (SELECT purchase_id FROM payments WHERE external_id LIKE '`+syntheticPrefix+`%')`); err != nil {
		return 0, 0, fmt.Errorf("erro ao apagar previsões antigas: %w", err)
	}

	dueDay, horizon, err := cardHorizons(ctx, tx, closeDaysBefore, today)
	if err != nil {
		return 0, 0, err
	}

	known, err := knownCardInstallments(ctx, tx)
	if err != nil {
		return 0, 0, err
	}
	for _, f := range domain.ForecastCardInstallments(known, dueDay, horizon) {
		desc := f.Description()
		raw := "[previsto pelo FinAssist: a parcela ainda não chegou do banco]"
		if err := insertSynthetic(ctx, tx, desc, f.Category, f.Amount, domain.PaymentStatusPending, raw, f.BillDue, f.AccountID, forecastPrefix+f.Key); err != nil {
			return 0, 0, err
		}
		forecasts++
	}

	adjustments, err = insertBillGaps(ctx, tx)
	if err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE payments pay SET note = s.note FROM synthetic_notes s WHERE pay.external_id = s.external_id`); err != nil {
		return 0, 0, fmt.Errorf("erro ao devolver as notas das previsões: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("erro ao gravar previsões: %w", err)
	}
	return forecasts, adjustments, nil
}

// cardHorizons dá, por cartão com vencimento conhecido, o dia do vencimento e até onde prever: a fatura que vence daqui a
// domain.CardForecastMonthsAhead meses, para dar para planejar. Cartão parado (sem lançamento real na fatura aberta nem
// na última fechada) não ganha parcela prevista.
func cardHorizons(ctx context.Context, tx pgx.Tx, closeDaysBefore int, today time.Time) (map[uuid.UUID]int, map[uuid.UUID]time.Time, error) {
	rows, err := tx.Query(ctx, `
		SELECT a.id, EXTRACT(DAY FROM a.due_date)::int,
		       CASE WHEN a.close_date IS NOT NULL AND a.close_date < a.due_date THEN (a.due_date - a.close_date) END,
		       (SELECT MAX(pay.bill_due_date) FROM payments pay WHERE pay.account_id = a.id AND `+notSynthetic+`)
		FROM accounts a WHERE a.type = 'CREDIT' AND a.due_date IS NOT NULL
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao ler cartões: %w", err)
	}
	defer rows.Close()
	dueDay, horizon := map[uuid.UUID]int{}, map[uuid.UUID]time.Time{}
	for rows.Next() {
		var id uuid.UUID
		var day int
		var bankClose *int
		var lastReal *time.Time
		if err := rows.Scan(&id, &day, &bankClose, &lastReal); err != nil {
			return nil, nil, fmt.Errorf("erro ao ler cartão: %w", err)
		}
		days := closeDaysBefore
		if bankClose != nil && *bankClose > 0 && *bankClose < 28 {
			days = *bankClose
		}
		if lastReal == nil {
			continue
		}
		open := domain.CardBillDue(today, day, days)
		if domain.MonthsBetween(*lastReal, open) > 1 {
			continue // nada real desde antes da última fatura fechada: o cartão parou
		}
		dueDay[id], horizon[id] = day, domain.CardForecastHorizon(today, day)
	}
	return dueDay, horizon, rows.Err()
}

// knownCardInstallments lê as parcelas de cartão que vieram do banco, com a fatura já calculada.
func knownCardInstallments(ctx context.Context, tx pgx.Tx) ([]domain.CardInstallment, error) {
	rows, err := tx.Query(ctx, `
		SELECT pay.account_id, p.description, p.category, pay.amount, pay.bill_due_date
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		JOIN accounts a ON a.id = pay.account_id
		WHERE a.type = 'CREDIT' AND p.kind = 'EXPENSE' AND pay.status != 'CANCELLED' AND pay.bill_due_date IS NOT NULL
		  AND p.description ~ '`+installmentSuffix+`' AND `+notSynthetic)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler parcelas do cartão: %w", err)
	}
	defer rows.Close()
	var out []domain.CardInstallment
	for rows.Next() {
		var (
			k    domain.CardInstallment
			desc string
		)
		if err := rows.Scan(&k.AccountID, &desc, &k.Category, &k.Amount, &k.BillDue); err != nil {
			return nil, fmt.Errorf("erro ao ler parcela do cartão: %w", err)
		}
		m := installmentDesc.FindStringSubmatch(desc)
		if m == nil {
			continue
		}
		k.Base = m[1]
		k.Number, _ = strconv.Atoi(m[2])
		k.Total, _ = strconv.Atoi(m[3])
		out = append(out, k)
	}
	return out, rows.Err()
}

// insertBillGaps lança, para cada fatura fechada do banco a partir do começo do histórico do cartão, a parte do total
// que o app não conhece (reais + previstas do mesmo cartão no mesmo mês de fatura).
func insertBillGaps(ctx context.Context, tx pgx.Tx) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT b.external_id, b.account_id, b.due_date, b.total_amount, a.name,
		       COALESCE((SELECT SUM(pay.amount) FROM payments pay JOIN purchases p ON p.id = pay.purchase_id
		                 WHERE pay.account_id = b.account_id AND p.kind = 'EXPENSE' AND pay.status != 'CANCELLED'
		                   AND DATE_TRUNC('month', pay.bill_due_date) = DATE_TRUNC('month', b.due_date)), 0)
		FROM card_bills b
		JOIN accounts a ON a.id = b.account_id
		WHERE b.due_date >= (SELECT MIN(pay.bill_due_date) FROM payments pay WHERE pay.account_id = b.account_id AND `+notSynthetic+`)
		ORDER BY b.due_date
	`)
	if err != nil {
		return 0, fmt.Errorf("erro ao ler faturas do cartão: %w", err)
	}
	type gap struct {
		id, name string
		account  uuid.UUID
		due      time.Time
		amount   float64
	}
	var gaps []gap
	for rows.Next() {
		var g gap
		var total, known float64
		if err := rows.Scan(&g.id, &g.account, &g.due, &total, &g.name, &known); err != nil {
			rows.Close()
			return 0, fmt.Errorf("erro ao ler fatura do cartão: %w", err)
		}
		if g.amount = domain.CardBillGap(total, known); g.amount != 0 {
			gaps = append(gaps, g)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("erro ao ler faturas do cartão: %w", err)
	}
	for _, g := range gaps {
		desc := fmt.Sprintf("Fatura %s %s · não detalhado no Open Finance", g.name, g.due.Format("02/01"))
		if g.amount < 0 {
			desc = fmt.Sprintf("Fatura %s %s · créditos não detalhados no Open Finance", g.name, g.due.Format("02/01"))
		}
		raw := "[ajuste do FinAssist: total da fatura informado pelo banco menos os lançamentos conhecidos]"
		if err := insertSynthetic(ctx, tx, desc, domain.CategoryOther, g.amount, domain.PaymentStatusPaid, raw, g.due, g.account, billGapPrefix+g.id); err != nil {
			return 0, err
		}
	}
	return len(gaps), nil
}

// insertSynthetic grava um lançamento criado pelo app num cartão: compra avulsa no crédito, pagamento com a fatura já
// definida (bill_due_date) e o external_id que o identifica como sintético.
func insertSynthetic(ctx context.Context, tx pgx.Tx, desc string, category domain.Category, amount float64, status domain.PaymentStatus, raw string, billDue time.Time, account uuid.UUID, externalID string) error {
	var p *domain.Purchase
	var err error
	if amount < 0 { // crédito da fatura: entra como estorno
		p, err = domain.NewCardRefund(-amount, &desc, category, raw)
	} else {
		p, err = domain.NewPurchase(amount, &desc, category, domain.PaymentMethodCreditCard, domain.PurchaseTypeSingle, raw)
	}
	if err != nil {
		return fmt.Errorf("lançamento do cartão inválido: %w", err)
	}
	if err := insertPurchase(ctx, tx, p); err != nil {
		return err
	}
	var paidAt *time.Time
	if status == domain.PaymentStatusPaid {
		paidAt = &billDue
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payments (id, purchase_id, amount, status, due_date, bill_due_date, paid_at, external_id, account_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $5, $6, $7, $8, NOW())
	`, uuid.New(), p.ID, amount, status, billDue, paidAt, externalID, account); err != nil {
		return fmt.Errorf("erro ao salvar lançamento do cartão: %w", err)
	}
	return nil
}
