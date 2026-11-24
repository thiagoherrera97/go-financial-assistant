package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pluggyCategory é a categoria que o banco mandou, gravada em raw_input ("[open finance: X | ...]"); vazia nos manuais.
const pluggyCategory = `COALESCE(LOWER(BTRIM(SUBSTRING(p.raw_input FROM '^\[open finance: (.*?) \| '))), '')`

// RefreshSubcategories preenche a subcategoria e o essencial/supérfluo de todas as despesas: o que você corrigiu à mão fica,
// depois vale a regra do estabelecimento e por fim a detecção pela descrição e pela categoria do banco. Só grava o que muda,
// então rodar de novo não custa nada; quando a categoria muda (regra, sincronização), a subcategoria acompanha na próxima vez.
func (r *ClassifyRepo) RefreshSubcategories(ctx context.Context) (int64, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT p.id, p.category, `+pluggyCategory+`, COALESCE(p.description, ''),
		       COALESCE(p.subcategory, ''), p.essential, p.subcategory_manual, p.essential_manual,
		       COALESCE(cr.subcategory, ''), cr.essential
		FROM purchases p
		LEFT JOIN category_rules cr ON cr.key = `+expenseKey+` AND cr.category = p.category
		WHERE p.kind = 'EXPENSE'
	`)
	if err != nil {
		return 0, fmt.Errorf("erro ao ler despesas para subcategorizar: %w", err)
	}
	var (
		ids       []uuid.UUID
		subs      []*string
		essential []*bool
		manual    []bool
	)
	for rows.Next() {
		var (
			id                   uuid.UUID
			category             domain.Category
			pluggy, desc, cur    string
			curEss, ruleEss      *bool
			subManual, essManual bool
			ruleSub              string
		)
		if err := rows.Scan(&id, &category, &pluggy, &desc, &cur, &curEss, &subManual, &essManual, &ruleSub, &ruleEss); err != nil {
			rows.Close()
			return 0, fmt.Errorf("erro ao escanear despesa para subcategorizar: %w", err)
		}
		sub, ess, keepManual := resolveSubcategory(category, pluggy, desc, domain.Subcategory(cur), curEss, subManual, essManual,
			domain.Subcategory(ruleSub), ruleEss)
		if string(sub) == cur && sameBool(ess, curEss) && keepManual == subManual {
			continue
		}
		var s *string
		if sub != "" {
			v := string(sub)
			s = &v
		}
		ids, subs, essential, manual = append(ids, id), append(subs, s), append(essential, ess), append(manual, keepManual)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("erro ao ler despesas para subcategorizar: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE purchases p SET subcategory = u.sub, essential = u.essential, subcategory_manual = u.manual
		FROM unnest($1::uuid[], $2::text[], $3::boolean[], $4::boolean[]) AS u(id, sub, essential, manual)
		WHERE p.id = u.id
	`, ids, subs, essential, manual)
	if err != nil {
		return 0, fmt.Errorf("erro ao gravar subcategorias: %w", err)
	}
	return tag.RowsAffected(), nil
}

// resolveSubcategory decide a subcategoria e o essencial de uma despesa. A subcategoria à mão só vale se ainda for da categoria
// (se a categoria mudou depois, ela deixa de ser manual). O essencial à mão vale sempre.
func resolveSubcategory(category domain.Category, pluggy, desc string, cur domain.Subcategory, curEss *bool, subManual,
	essManual bool, ruleSub domain.Subcategory, ruleEss *bool) (domain.Subcategory, *bool, bool) {
	var sub domain.Subcategory
	switch {
	case subManual && cur.BelongsTo(category):
		sub = cur
	case ruleSub.BelongsTo(category):
		sub, subManual = ruleSub, false
	default:
		sub, subManual = domain.Classify(category, pluggy, desc), false
	}
	var ess *bool
	switch {
	case essManual:
		ess = curEss
	case ruleEss != nil:
		ess = ruleEss
	default:
		info, _ := sub.Info()
		ess = info.Essential
	}
	return sub, ess, subManual
}

func sameBool(a, b *bool) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}

// SetClassification corrige a despesa do pagamento id. Com ApplyToAll grava a regra do estabelecimento e aplica a categoria às
// outras despesas dele (tirando as correções à mão delas). found falso se o pagamento não existe ou não é despesa; others é
// quantas outras despesas mudaram.
func (r *ClassifyRepo) SetClassification(ctx context.Context, id uuid.UUID, c domain.Classification) (found bool, others int64, err error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("erro ao corrigir a classificação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var purchaseID uuid.UUID
	var key string
	err = tx.QueryRow(ctx, `
		SELECT p.id, `+expenseKey+` FROM payments pay JOIN purchases p ON p.id = pay.purchase_id
		WHERE pay.id = $1 AND p.kind = 'EXPENSE'
	`, id).Scan(&purchaseID, &key)
	if err == pgx.ErrNoRows {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("erro ao ler a despesa: %w", err)
	}
	var sub *string
	if c.Subcategory != "" {
		v := string(c.Subcategory)
		sub = &v
	}
	if _, err := tx.Exec(ctx, `
		UPDATE purchases SET category = $2, subcategory = $3::text, subcategory_manual = $3::text IS NOT NULL,
			essential = $4::boolean, essential_manual = $4::boolean IS NOT NULL
		WHERE id = $1
	`, purchaseID, string(c.Category), sub, c.Essential); err != nil {
		return false, 0, fmt.Errorf("erro ao corrigir a classificação: %w", err)
	}
	// As outras parcelas da mesma compra (inclusive as previstas) seguem a mesma classificação, como a nota.
	if !c.ApplyToAll {
		if _, err := tx.Exec(ctx, `
			UPDATE purchases p SET category = src.category, subcategory = src.subcategory,
				subcategory_manual = src.subcategory_manual, essential = src.essential, essential_manual = src.essential_manual
			FROM purchases src, payments pay, payments spay
			WHERE src.id = $1 AND spay.purchase_id = src.id AND pay.purchase_id = p.id AND p.id <> src.id
			  AND p.kind = 'EXPENSE' AND pay.status != 'CANCELLED'
			  AND src.description ~ '`+installmentSuffix+`'
			  AND SUBSTRING(p.description FROM '`+installmentTotal+`') = SUBSTRING(src.description FROM '`+installmentTotal+`')
			  AND `+expenseKey+` = `+strings.ReplaceAll(expenseKey, "p.description", "src.description")+`
			  AND ROUND(pay.amount::numeric, 2) = ROUND(spay.amount::numeric, 2)
		`, purchaseID); err != nil {
			return false, 0, fmt.Errorf("erro ao classificar as outras parcelas: %w", err)
		}
	}
	if c.ApplyToAll && key != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO category_rules (key, category, subcategory, essential) VALUES ($1, $2, $3, $4)
			ON CONFLICT (key) DO UPDATE SET category = EXCLUDED.category, subcategory = EXCLUDED.subcategory,
				essential = EXCLUDED.essential, updated_at = NOW()
		`, key, string(c.Category), sub, c.Essential); err != nil {
			return false, 0, fmt.Errorf("erro ao gravar a regra do estabelecimento: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE purchases p SET category = $2, subcategory_manual = FALSE, essential_manual = FALSE
			WHERE p.kind = 'EXPENSE' AND p.id <> $3 AND `+expenseKey+` = $1
		`, key, string(c.Category), purchaseID)
		if err != nil {
			return false, 0, fmt.Errorf("erro ao corrigir as outras despesas do estabelecimento: %w", err)
		}
		others = tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return false, 0, fmt.Errorf("erro ao corrigir a classificação: %w", err)
	}
	if _, err := r.RefreshSubcategories(ctx); err != nil {
		return true, others, err
	}
	return true, others, nil
}

// SpendBySubcategory soma as despesas do mês por subcategoria e essencial/supérfluo, com a média dos três meses anteriores.
func (r *ClassifyRepo) SpendBySubcategory(ctx context.Context, month time.Time) ([]domain.SubcategorySpend, error) {
	rows, err := r.db.Pool.Query(ctx, `
		WITH e AS (
			SELECT `+paymentMonth+` AS month, p.category, COALESCE(p.subcategory, '') AS sub, p.essential, pay.amount,
			       `+isInstallment+` AS installment
			FROM payments pay
			JOIN purchases p ON p.id = pay.purchase_id
			WHERE pay.status != 'CANCELLED' AND p.kind = 'EXPENSE'
			  AND `+paymentMonth+` BETWEEN ($1::date - INTERVAL '3 months')::date AND $1::date
		)
		SELECT category, sub, essential,
		       COUNT(*) FILTER (WHERE month = $1::date),
		       COALESCE(SUM(amount) FILTER (WHERE month = $1::date), 0),
		       COALESCE(SUM(amount) FILTER (WHERE month < $1::date), 0) / 3,
		       COALESCE(SUM(amount) FILTER (WHERE month = $1::date AND installment), 0)
		FROM e
		GROUP BY 1, 2, 3
		HAVING COUNT(*) FILTER (WHERE month = $1::date) > 0
		ORDER BY 5 DESC, 1, 2
	`, month)
	if err != nil {
		return nil, fmt.Errorf("erro ao somar despesas por subcategoria: %w", err)
	}
	defer rows.Close()
	var out []domain.SubcategorySpend
	for rows.Next() {
		var s domain.SubcategorySpend
		if err := rows.Scan(&s.Category, &s.Subcategory, &s.Essential, &s.Count, &s.Total, &s.PrevAverage, &s.Installments); err != nil {
			return nil, fmt.Errorf("erro ao escanear despesas por subcategoria: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// installmentTotal tira o total de parcelas da marcação no fim da descrição ("VIVARA SJV (4/10)" → "10").
const installmentTotal = `/(\d+)\)$`

// inheritInstallmentClassification passa a classificação corrigida à mão de uma parcela para as outras parcelas da mesma
// compra que ainda não foram corrigidas: as previstas que o app recria e as reais que chegam do banco depois. Irmãs =
// mesmo estabelecimento, mesmo valor e mesmo total de parcelas. Havendo mais de uma corrigida, vale a mais recente.
func inheritInstallmentClassification(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}) error {
	if _, err := q.Exec(ctx, `
		WITH s AS (
			SELECT p.id, `+expenseKey+` AS key, ROUND(pay.amount::numeric, 2) AS amount,
			       SUBSTRING(p.description FROM '`+installmentTotal+`') AS total, `+txDate+` AS day,
			       p.category, p.subcategory, p.subcategory_manual, p.essential, p.essential_manual
			FROM purchases p JOIN payments pay ON pay.purchase_id = p.id
			WHERE p.kind = 'EXPENSE' AND pay.status != 'CANCELLED' AND p.description ~ '`+installmentSuffix+`'
		), src AS (
			SELECT DISTINCT ON (key, amount, total) * FROM s
			WHERE subcategory_manual OR essential_manual
			ORDER BY key, amount, total, day DESC
		)
		UPDATE purchases p SET
			category = CASE WHEN src.subcategory_manual THEN src.category ELSE p.category END,
			subcategory = CASE WHEN src.subcategory_manual THEN src.subcategory ELSE p.subcategory END,
			subcategory_manual = src.subcategory_manual,
			essential = CASE WHEN src.essential_manual THEN src.essential ELSE p.essential END,
			essential_manual = src.essential_manual
		FROM s JOIN src ON src.key = s.key AND src.amount = s.amount AND src.total = s.total
		WHERE p.id = s.id AND NOT (s.subcategory_manual OR s.essential_manual)
	`); err != nil {
		return fmt.Errorf("erro ao repetir a classificação nas outras parcelas: %w", err)
	}
	return nil
}
