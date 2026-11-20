package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func (r *BudgetRepo) ExpenseKeyMonths(ctx context.Context, from, to time.Time) ([]domain.ExpenseKeyMonth, error) {
	query := `
		SELECT ` + expenseKey + ` AS key,
		       (ARRAY_AGG(` + cleanDescription + ` ORDER BY pay.created_at DESC))[1] AS label,
		       (ARRAY_AGG(p.category ORDER BY pay.created_at DESC))[1] AS category,
		       ` + paymentMonth + ` AS month,
		       SUM(pay.amount), COUNT(*),
		       EXTRACT(DAY FROM MAX(COALESCE(pay.due_date, pay.reference_month, pay.created_at::date)))::int,
		       BOOL_AND(pay.status = 'PAID'),
		       BOOL_OR(p.type = 'INSTALLMENT' OR COALESCE(p.description, '') ~* '(\(\s*\d+\s*/\s*\d+\s*\)|parc[a-z.]*\s*\d+\s*/\s*\d+)'),
		       BOOL_OR(p.type = 'RECURRING')
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'EXPENSE' AND pay.status != 'CANCELLED' AND p.description IS NOT NULL
		  AND ` + paymentMonth + ` BETWEEN $1::date AND $2::date
		GROUP BY 1, 4
		HAVING ` + expenseKey + ` <> ''
		ORDER BY 4, 5 DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas por conta: %w", err)
	}
	defer rows.Close()

	var result []domain.ExpenseKeyMonth
	for rows.Next() {
		var k domain.ExpenseKeyMonth
		if err := rows.Scan(&k.Key, &k.Label, &k.Category, &k.Month, &k.Total, &k.Count, &k.Day, &k.AllPaid, &k.Installment, &k.Recurring); err != nil {
			return nil, fmt.Errorf("erro ao escanear despesas por conta: %w", err)
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

func (r *BudgetRepo) ExpenseRules(ctx context.Context) (map[string]domain.ExpenseClass, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT key, class FROM expense_rules`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler regras de despesas: %w", err)
	}
	defer rows.Close()

	rules := map[string]domain.ExpenseClass{}
	for rows.Next() {
		var key, class string
		if err := rows.Scan(&key, &class); err != nil {
			return nil, fmt.Errorf("erro ao escanear regra de despesa: %w", err)
		}
		rules[key] = domain.ExpenseClass(class)
	}
	return rules, rows.Err()
}

func (r *BudgetRepo) SetExpenseRule(ctx context.Context, key string, class domain.ExpenseClass) error {
	var err error
	if class == "" {
		_, err = r.db.Pool.Exec(ctx, `DELETE FROM expense_rules WHERE key = $1`, key)
	} else {
		_, err = r.db.Pool.Exec(ctx, `
			INSERT INTO expense_rules (key, class, updated_at) VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO UPDATE SET class = EXCLUDED.class, updated_at = NOW()
		`, key, string(class))
	}
	if err != nil {
		return fmt.Errorf("erro ao gravar regra de despesa: %w", err)
	}
	return nil
}

func (r *BudgetRepo) KnownInstallments(ctx context.Context, from, to time.Time) (map[time.Time]float64, error) {
	query := `
		SELECT ` + paymentMonth + ` AS month, SUM(pay.amount)
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'EXPENSE' AND p.type = 'INSTALLMENT' AND pay.status != 'CANCELLED'
		  AND ` + paymentMonth + ` BETWEEN $1::date AND $2::date
		GROUP BY 1
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar parcelas futuras: %w", err)
	}
	defer rows.Close()

	result := map[time.Time]float64{}
	for rows.Next() {
		var m time.Time
		var total float64
		if err := rows.Scan(&m, &total); err != nil {
			return nil, fmt.Errorf("erro ao escanear parcelas futuras: %w", err)
		}
		result[m.UTC()] = total
	}
	return result, rows.Err()
}

func (r *BudgetRepo) IncomePayments(ctx context.Context, from, to time.Time) ([]domain.IncomePayment, error) {
	query := `
		SELECT CASE WHEN p.category = 'SALARY' THEN 'salario' ELSE ` + expenseKey + ` END AS key,
		       CASE WHEN p.category = 'SALARY' THEN 'Salário' ELSE p.description END, ` + paymentMonth + ` AS month, pay.amount
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'INCOME' AND pay.status != 'CANCELLED' AND p.description IS NOT NULL
		  AND ` + paymentMonth + ` BETWEEN $1::date AND $2::date
		  AND ` + expenseKey + ` <> ''
		ORDER BY 3, 4 DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar entradas de renda: %w", err)
	}
	defer rows.Close()

	var result []domain.IncomePayment
	for rows.Next() {
		var i domain.IncomePayment
		if err := rows.Scan(&i.Key, &i.Label, &i.Month, &i.Amount); err != nil {
			return nil, fmt.Errorf("erro ao escanear entrada de renda: %w", err)
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (r *BudgetRepo) SalaryCredits(ctx context.Context, from, to time.Time) ([]domain.SalaryCredit, error) {
	query := `
		SELECT ` + txDate + `::date AS day, pay.amount
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'INCOME' AND p.category = 'SALARY' AND pay.status != 'CANCELLED'
		  AND ` + txDate + `::date BETWEEN $1::date AND $2::date
		ORDER BY 1, 2 DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar créditos de salário: %w", err)
	}
	defer rows.Close()

	var result []domain.SalaryCredit
	for rows.Next() {
		var c domain.SalaryCredit
		if err := rows.Scan(&c.Date, &c.Amount); err != nil {
			return nil, fmt.Errorf("erro ao escanear crédito de salário: %w", err)
		}
		c.Date = c.Date.UTC()
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *BudgetRepo) MonthLines(ctx context.Context, month time.Time) ([]domain.MonthLine, error) {
	query := `
		SELECT ` + expenseKey + `, COALESCE(` + cleanDescription + `, ''), p.category, p.kind, COALESCE(p.transfer_direction, ''),
		       pay.amount, pay.status = 'PENDING', COALESCE(a.id::text, ''), COALESCE(a.name, ''),
		       COALESCE(a.type = 'CREDIT', p.payment_method = 'CREDIT_CARD'),
		       CASE WHEN pay.external_id LIKE '` + forecastPrefix + `%' THEN 'forecast'
		            WHEN pay.external_id LIKE '` + billGapPrefix + `%' THEN 'bill' ELSE '' END,
		       ` + txDate + `::date, COALESCE(pay.note, '')
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		LEFT JOIN accounts a ON a.id = pay.account_id
		WHERE pay.status != 'CANCELLED' AND ` + paymentMonth + ` = DATE_TRUNC('month', $1::date)::date
		ORDER BY pay.amount DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, month)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar lançamentos do mês: %w", err)
	}
	defer rows.Close()

	var result []domain.MonthLine
	for rows.Next() {
		var l domain.MonthLine
		if err := rows.Scan(&l.Key, &l.Label, &l.Category, &l.Kind, &l.Direction, &l.Amount, &l.Pending, &l.AccountID,
			&l.AccountName, &l.Card, &l.Synthetic, &l.Date, &l.Note); err != nil {
			return nil, fmt.Errorf("erro ao escanear lançamento do mês: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}
