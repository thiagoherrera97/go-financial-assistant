package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// transactionWhere monta a cláusula WHERE (e os argumentos) do filtro, compartilhada pela lista e
// pelos grupos. Só texto fixo entra no SQL; os valores vão sempre como parâmetros.
func transactionWhere(f domain.TransactionFilter) (string, []any) {
	var (
		where []string
		args  []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	where = append(where, "pay.status != 'CANCELLED'")
	switch {
	case f.Day != nil:
		add(txDate+" = ?::date", *f.Day)
	case f.Month != nil:
		add(paymentMonth+" = ?::date", *f.Month)
	}
	if f.Kind != "" {
		add("p.kind = ?", f.Kind)
	}
	if f.ExcludeKind != "" {
		add("p.kind != ?", f.ExcludeKind)
	}
	if f.Category != "" {
		add("p.category = ?", f.Category)
	}
	switch f.Subcategory {
	case "":
	case "none":
		where = append(where, "p.subcategory IS NULL")
	default:
		add("p.subcategory = ?", f.Subcategory)
	}
	switch f.Essential {
	case "true":
		where = append(where, "p.essential")
	case "false":
		where = append(where, "NOT p.essential")
	case "unknown":
		where = append(where, "p.kind = 'EXPENSE' AND p.essential IS NULL")
	}
	if f.HideRefunded {
		where = append(where, "pay.amount > 0 AND NOT EXISTS ("+refundOf+")")
	}
	if f.PaymentMethod != "" {
		add("p.payment_method = ?", f.PaymentMethod)
	}
	if f.AccountID != nil {
		add("pay.account_id = ?", *f.AccountID)
	}
	if f.Search != "" {
		add(`(p.description ILIKE ? ESCAPE '\' OR pay.note ILIKE ? ESCAPE '\')`, "%"+escapeLike(f.Search)+"%")
	}
	return strings.Join(where, " AND "), args
}

// refundOf acha o estorno de uma despesa (pay/p): mesmo valor com sinal trocado, mesma descrição e até 45 dias de
// distância. É o caso da pré-autorização (recarga, hotel, posto) que o banco cobra e devolve logo depois.
var refundOf = `
	SELECT 1 FROM payments rp JOIN purchases r ON r.id = rp.purchase_id
	WHERE r.kind = 'EXPENSE' AND rp.status != 'CANCELLED' AND rp.amount < 0
	  AND ABS(rp.amount + pay.amount) < 0.005 AND r.description = p.description
	  AND ABS(` + strings.ReplaceAll(txDate, "pay.", "rp.") + ` - ` + txDate + `) <= 45`

func (r *TransactionsRepo) Transactions(ctx context.Context, f domain.TransactionFilter) ([]domain.Transaction, int, error) {
	where, args := transactionWhere(f)
	args = append(args, f.Limit, f.Offset)
	order := "date DESC, pay.bank_time DESC NULLS LAST, pay.created_at DESC, pay.id"
	if f.SortByAmount {
		order = "ABS(pay.amount) DESC, date DESC, pay.id"
	}

	// COUNT(*) OVER () traz o total sem uma segunda consulta; sem linhas, o total é 0.
	query := fmt.Sprintf(`
		SELECT pay.id, `+txDate+` AS date,
		       COALESCE(p.description, ''), p.category, COALESCE(p.subcategory, ''), p.essential, p.payment_method, p.kind,
		       COALESCE(p.transfer_direction, ''), p.type, pay.status, pay.amount,
		       pay.installment_number, a.id, COALESCE(a.name || ' ' || a.last4, ''),
		       pay.external_id IS NOT NULL, COALESCE(pay.note, ''), pay.bank_time, COUNT(*) OVER ()
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		LEFT JOIN accounts a ON a.id = pay.account_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, order, len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao listar transações: %w", err)
	}
	defer rows.Close()

	var (
		result []domain.Transaction
		total  int
	)
	for rows.Next() {
		var t domain.Transaction
		if err := rows.Scan(&t.ID, &t.Date, &t.Description, &t.Category, &t.Subcategory, &t.Essential, &t.PaymentMethod, &t.Kind,
			&t.TransferDirection, &t.Type, &t.Status, &t.Amount, &t.InstallmentNumber,
			&t.AccountID, &t.AccountName, &t.FromOpenFinance, &t.Note, &t.BankTime, &total); err != nil {
			return nil, 0, fmt.Errorf("erro ao escanear transação: %w", err)
		}
		result = append(result, t)
	}
	return result, total, rows.Err()
}

func (r *TransactionsRepo) TransactionGroups(ctx context.Context, f domain.TransactionFilter, by domain.GroupBy) ([]domain.TransactionGroup, error) {
	// A chave do grupo vem de fragmentos fixos, nunca do cliente.
	var key, order string
	switch by {
	case domain.GroupByCategory:
		// Toda receita vai para um grupo só (Entradas), na frente; as categorias ficam só com gastos e investimentos.
		key = "CASE WHEN p.kind = 'INCOME' THEN '" + domain.IncomeGroupKey + "' ELSE p.category END"
		order = "(" + key + ") = '" + domain.IncomeGroupKey + "' DESC, expense DESC, income DESC, transfer DESC, key"
	case domain.GroupByDay:
		key, order = "TO_CHAR("+txDate+", 'YYYY-MM-DD')", "key DESC"
	default:
		return nil, fmt.Errorf("agrupamento inválido: %q", by)
	}
	where, args := transactionWhere(f)

	query := fmt.Sprintf(`
		SELECT %s AS key, COUNT(*),
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'EXPENSE'), 0)  AS expense,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'INCOME'), 0)   AS income,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'TRANSFER'), 0) AS transfer,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'TRANSFER' AND p.transfer_direction = 'OUT'), 0) AS applied,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'TRANSFER' AND p.transfer_direction = 'IN'), 0) AS redeemed
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE %s
		GROUP BY 1
		ORDER BY %s
	`, key, where, order)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao agrupar transações: %w", err)
	}
	defer rows.Close()

	var result []domain.TransactionGroup
	for rows.Next() {
		var g domain.TransactionGroup
		if err := rows.Scan(&g.Key, &g.Count, &g.Expense, &g.Income, &g.Transfer, &g.Applied, &g.Redeemed); err != nil {
			return nil, fmt.Errorf("erro ao escanear grupo: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

// SetTransactionNote grava a nota da transação. Nota apagada fica vazia (não nula), para a nota aprendida não voltar
// nela. Numa parcela ("(3/10)"), a nota vale para as outras parcelas da mesma compra; e as transações sem nota do mesmo
// comércio e valor herdam a nota quando ela é a única já usada ali (ver inheritNotes).
func (r *TransactionsRepo) SetTransactionNote(ctx context.Context, id uuid.UUID, note string) (bool, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("erro ao gravar a nota: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var key string
	err = tx.QueryRow(ctx, `
		UPDATE payments pay SET note = $2 FROM purchases p
		WHERE pay.id = $1 AND p.id = pay.purchase_id
		RETURNING `+expenseKey, id, note).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("erro ao gravar a nota: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE payments pay SET note = $2
		FROM purchases p, payments src JOIN purchases sp ON sp.id = src.purchase_id
		WHERE src.id = $1 AND p.id = pay.purchase_id AND pay.id <> src.id AND pay.status != 'CANCELLED'
		  AND sp.description ~ '\(\d+/\d+\)\s*$'
		  AND SUBSTRING(p.description FROM '/(\d+)\)\s*$') = SUBSTRING(sp.description FROM '/(\d+)\)\s*$')
		  AND `+expenseKey+` = `+strings.ReplaceAll(expenseKey, "p.description", "sp.description")+`
		  AND ABS(pay.amount - src.amount) < 0.005
	`, id, note); err != nil {
		return false, fmt.Errorf("erro ao gravar a nota nas outras parcelas: %w", err)
	}
	if _, err := inheritNotes(ctx, tx, key); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("erro ao gravar a nota: %w", err)
	}
	return true, nil
}

func (r *TransactionsRepo) DeleteManualTransaction(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		DELETE FROM purchases p
		USING payments pay
		WHERE pay.id = $1 AND pay.purchase_id = p.id AND p.type = 'SINGLE'
		  AND NOT EXISTS (SELECT 1 FROM payments q WHERE q.purchase_id = p.id AND q.external_id IS NOT NULL)
	`, id)
	if err != nil {
		return false, fmt.Errorf("erro ao apagar o lançamento: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
