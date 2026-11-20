package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func (r *LedgerRepo) UpsertAccount(ctx context.Context, a ports.ExternalAccount) (uuid.UUID, error) {
	query := `
		INSERT INTO accounts (external_id, item_id, type, name, last4, balance, credit_limit, available_credit_limit,
			institution_id, brand, close_date, due_date, minimum_payment, auto_invested_balance, reserved_balances, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW())
		ON CONFLICT (external_id) DO UPDATE SET
			item_id = EXCLUDED.item_id, type = EXCLUDED.type, name = EXCLUDED.name, last4 = EXCLUDED.last4,
			balance = EXCLUDED.balance, credit_limit = EXCLUDED.credit_limit,
			available_credit_limit = EXCLUDED.available_credit_limit,
			institution_id = COALESCE(EXCLUDED.institution_id, accounts.institution_id),
			brand = EXCLUDED.brand, close_date = EXCLUDED.close_date, due_date = EXCLUDED.due_date,
			minimum_payment = EXCLUDED.minimum_payment, auto_invested_balance = EXCLUDED.auto_invested_balance,
			reserved_balances = EXCLUDED.reserved_balances,
			updated_at = NOW()
		RETURNING id
	`
	reserved := a.Reserved
	if reserved == nil {
		reserved = []domain.ReservedBalance{}
	}
	var id uuid.UUID
	if err := r.db.Pool.QueryRow(ctx, query,
		a.ID, a.ItemID, a.Type, a.Name, a.Last4, a.Balance, a.CreditLimit, a.AvailableCreditLimit,
		a.InstitutionID, a.Brand, a.CloseDate, a.DueDate, a.MinimumPayment, a.AutoInvested, reserved,
	).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("erro ao salvar conta: %w", err)
	}
	return id, nil
}

// O logo é buscado de novo a cada 30 dias; se a busca falhou, tenta de novo depois de 1 dia.
func (r *LedgerRepo) UpsertInstitution(ctx context.Context, itemID string, inst ports.ExternalInstitution) (uuid.UUID, bool, error) {
	var id uuid.UUID
	var needsLogo bool
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO institutions (item_id, name, color, source_updated_at, updated_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, NOW())
		ON CONFLICT (item_id) DO UPDATE SET name = EXCLUDED.name, color = COALESCE(EXCLUDED.color, institutions.color),
			source_updated_at = COALESCE(EXCLUDED.source_updated_at, institutions.source_updated_at), updated_at = NOW()
		RETURNING id, (logo_checked_at IS NULL
			OR (logo IS NULL AND logo_checked_at < NOW() - INTERVAL '1 day')
			OR logo_checked_at < NOW() - INTERVAL '30 days')
	`, itemID, inst.Name, inst.Color, inst.UpdatedAt).Scan(&id, &needsLogo)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("erro ao salvar instituição: %w", err)
	}
	return id, needsLogo, nil
}

func (r *LedgerRepo) SaveInstitutionLogo(ctx context.Context, id uuid.UUID, data []byte, mime string) error {
	var err error
	if data == nil {
		_, err = r.db.Pool.Exec(ctx, `UPDATE institutions SET logo_checked_at = NOW() WHERE id = $1`, id)
	} else {
		_, err = r.db.Pool.Exec(ctx, `UPDATE institutions SET logo = $2, logo_mime = $3, logo_checked_at = NOW() WHERE id = $1`, id, data, mime)
	}
	if err != nil {
		return fmt.Errorf("erro ao salvar logo da instituição: %w", err)
	}
	return nil
}

func (r *LedgerRepo) RefreshExternal(ctx context.Context, tx ports.ExternalTransaction, accountID uuid.UUID) (bool, error) {
	// O CTE com UPDATE roda sempre; a consulta final só diz se a transação existia.
	query := `
		WITH linked AS (
			UPDATE payments SET account_id = COALESCE(account_id, $2),
				due_date = CASE WHEN $4 THEN $5::date ELSE due_date END,
				paid_at = CASE WHEN $4 AND paid_at IS NOT NULL THEN $5::timestamptz ELSE paid_at END,
				card_bill_id = COALESCE(NULLIF($6, ''), card_bill_id)
			WHERE external_id = $1 RETURNING purchase_id
		), recategorized AS (
			UPDATE purchases p SET category = $3
			WHERE p.id IN (SELECT purchase_id FROM linked) AND p.category = 'OTHER' AND $3 <> 'OTHER'
			  AND (p.kind <> 'EXPENSE' OR NOT EXISTS (SELECT 1 FROM category_rules r WHERE r.key = ` + expenseKey + `))
		)
		SELECT COUNT(*) FROM linked
	`
	var n int
	if err := r.db.Pool.QueryRow(ctx, query, tx.ID, accountID, tx.Category, tx.Installment, tx.Date, tx.BillID).Scan(&n); err != nil {
		return false, fmt.Errorf("erro ao verificar transação externa: %w", err)
	}
	return n > 0, nil
}

// RecategorizeExternalOther reclassifica as despesas do banco que estão em OTHER e ficaram fora da janela da sincronização
// (RefreshExternal só vê as transações relidas). A categoria do Pluggy está em raw_input ("[open finance: X | ...]") e é
// comparada por igualdade com o mapa. Regra por conta (category_rules, inclusive "manter em Outros") sempre vale. Um único
// UPDATE: atômico, idempotente e só troca a coluna category.
func (r *LedgerRepo) RecategorizeExternalOther(ctx context.Context, mapping map[string]domain.Category) (int64, error) {
	pluggy := make([]string, 0, len(mapping))
	target := make([]string, 0, len(mapping))
	for k, c := range mapping {
		if c == domain.CategoryOther {
			continue
		}
		pluggy, target = append(pluggy, strings.ToLower(strings.TrimSpace(k))), append(target, string(c))
	}
	if len(pluggy) == 0 {
		return 0, nil
	}
	query := `
		UPDATE purchases p SET category = m.category
		FROM unnest($1::text[], $2::text[]) AS m(pluggy, category)
		WHERE p.kind = 'EXPENSE' AND p.category = 'OTHER'
		  AND LOWER(BTRIM(SUBSTRING(p.raw_input FROM '^\[open finance: (.*?) \| '))) = m.pluggy
		  AND EXISTS (SELECT 1 FROM payments pay WHERE pay.purchase_id = p.id AND pay.external_id IS NOT NULL)
		  AND NOT EXISTS (SELECT 1 FROM category_rules r WHERE r.key = ` + expenseKey + `)
	`
	tag, err := r.db.Pool.Exec(ctx, query, pluggy, target)
	if err != nil {
		return 0, fmt.Errorf("erro ao reclassificar despesas externas em Outros: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *LedgerRepo) SaveExternal(ctx context.Context, purchase *domain.Purchase, payment *domain.Payment) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := insertPurchase(ctx, tx, purchase); err != nil {
		return err
	}

	query := `
		INSERT INTO payments
			(id, purchase_id, amount, status, due_date, paid_at, external_id, account_id, created_at, card_bill_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	if _, err := tx.Exec(ctx, query,
		payment.ID, payment.PurchaseID, payment.Amount, payment.Status,
		payment.DueDate, payment.PaidAt, payment.ExternalID, payment.AccountID, payment.CreatedAt, payment.CardBillID,
	); err != nil {
		return fmt.Errorf("erro ao salvar pagamento externo: %w", err)
	}

	// Regra de categoria da conta (escolhida pela pessoa) vence sempre numa despesa que chega do banco, seja qual for a
	// categoria vinda do Pluggy (mapeada ou não) e inclusive quando a regra é "manter em Outros" (OTHER).
	if _, err := tx.Exec(ctx, `
		UPDATE purchases p SET category = r.category
		FROM category_rules r
		WHERE p.id = $1 AND p.kind = 'EXPENSE' AND r.key = `+expenseKey, purchase.ID); err != nil {
		return fmt.Errorf("erro ao aplicar regra de categoria: %w", err)
	}

	return tx.Commit(ctx)
}

// ReconcileExternal vincula a transação do banco a um lançamento manual equivalente (mesmo tipo e valor; avulso até 3 dias
// de diferença, recorrente no mesmo mês), em vez de criar um segundo lançamento. Parcelados ficam de fora.
func (r *LedgerRepo) ReconcileExternal(ctx context.Context, tx ports.ExternalTransaction, accountID uuid.UUID) (bool, error) {
	query := `
		UPDATE payments SET external_id = $1, account_id = $6, status = 'PAID', paid_at = COALESCE(paid_at, $5::timestamptz)
		WHERE id = (
			SELECT pay.id
			FROM payments pay
			JOIN purchases p ON p.id = pay.purchase_id
			WHERE pay.external_id IS NULL
			  AND pay.status != 'CANCELLED'
			  AND p.kind = $2
			  AND ABS(pay.amount - $3) < 0.005
			  AND (
			        (p.type = 'SINGLE'
			         AND ABS(COALESCE(pay.due_date, pay.created_at::date) - $4::date) <= 3)
			     OR (p.type = 'RECURRING'
			         AND DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month)) = DATE_TRUNC('month', $4::date))
			  )
			ORDER BY pay.created_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
	`
	tag, err := r.db.Pool.Exec(ctx, query, tx.ID, tx.Kind, tx.Amount, tx.Date, tx.Date, accountID)
	if err != nil {
		return false, fmt.Errorf("erro ao conciliar transação externa: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *LedgerRepo) SaveInvestments(ctx context.Context, itemID string, positions []ports.ExternalInvestment, day time.Time) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	seen := make([]string, 0, len(positions))
	for _, p := range positions {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO investments (external_id, item_id, type, subtype, name, balance, amount, active, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, NOW())
			ON CONFLICT (external_id) DO UPDATE SET
				item_id = EXCLUDED.item_id, type = EXCLUDED.type, subtype = EXCLUDED.subtype, name = EXCLUDED.name,
				balance = EXCLUDED.balance, amount = EXCLUDED.amount, active = TRUE, updated_at = NOW()
			RETURNING id
		`, p.ID, itemID, p.Type, p.Subtype, p.Name, p.Balance, p.Amount).Scan(&id)
		if err != nil {
			return fmt.Errorf("erro ao salvar posição de investimento: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO investment_balances (investment_id, day, balance) VALUES ($1, $2::date, $3)
			ON CONFLICT (investment_id, day) DO UPDATE SET balance = EXCLUDED.balance
		`, id, day, p.Balance); err != nil {
			return fmt.Errorf("erro ao salvar saldo da posição: %w", err)
		}
		for _, m := range p.Movements {
			if _, err := tx.Exec(ctx, `
				INSERT INTO investment_movements (investment_id, external_id, day, amount) VALUES ($1, $2, $3::date, $4)
				ON CONFLICT (investment_id, external_id) DO UPDATE SET day = EXCLUDED.day, amount = EXCLUDED.amount
			`, id, m.ID, m.Day, m.Amount); err != nil {
				return fmt.Errorf("erro ao salvar movimentação do investimento: %w", err)
			}
		}
		seen = append(seen, p.ID)
	}

	// Posição que sumiu do Pluggy foi resgatada: zera o saldo daqui em diante para o histórico não carregá-la.
	if _, err := tx.Exec(ctx, `
		WITH gone AS (
			UPDATE investments SET active = FALSE, balance = 0, updated_at = NOW()
			WHERE item_id = $1 AND active AND external_id <> ALL($2::text[])
			RETURNING id
		)
		INSERT INTO investment_balances (investment_id, day, balance)
		SELECT id, $3::date, 0 FROM gone
		ON CONFLICT (investment_id, day) DO UPDATE SET balance = 0
	`, itemID, seen, day); err != nil {
		return fmt.Errorf("erro ao inativar posições resgatadas: %w", err)
	}
	return tx.Commit(ctx)
}

// ownCardPaymentReason marca o pagamento da fatura de um cartão seu feito por outra conta, ignorado pelo app.
const ownCardPaymentReason = "pagamento da fatura de um cartão seu (as compras do cartão já contam)"

// ReclassifyOwnMoves corrige o que o banco manda como gasto ou renda mas é dinheiro seu mudando de lugar. Roda ao fim de
// cada sincronização (idempotente) e vale também para o que já estava gravado:
//   - caixinha do Mercado Pago ("Dinheiro reservado ..." / "Dinheiro retirado ..."): vira transferência na categoria
//     Caixinha (nem gasto, nem renda, nem investimento);
//   - pagamento, por uma conta, da fatura de um cartão seu (a descrição traz o nome do cartão, ex.: "Pagamento de conta
//     SANTANDER UNIQUE VISA INFINITE"): é cancelado com o motivo, porque as compras do cartão já contam. Só vale para nome
//     de cartão com 10+ letras que não seja também o nome de uma conta corrente (evita "Mercado Pago" casar com tudo);
//   - e o contrário: "aplicação" que é pagamento ("Pagamento com Pix AIBR FIDC", que o banco classifica como fundo porque
//     quem recebe é um FIDC) volta a ser gasto, em Outros.
func (r *LedgerRepo) ReclassifyOwnMoves(ctx context.Context) (savings, cardPayments, fundPayments int64, err error) {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE purchases p SET
			kind = 'TRANSFER', category = 'RESERVE',
			transfer_direction = CASE WHEN p.kind = 'INCOME' THEN 'IN' WHEN p.kind = 'EXPENSE' THEN 'OUT' ELSE p.transfer_direction END
		WHERE (p.kind IN ('EXPENSE', 'INCOME') OR (p.kind = 'TRANSFER' AND p.category <> 'RESERVE')) AND p.type = 'SINGLE'
		  AND p.description ~* '^dinheiro (reservado|retirado)\M'
		  AND EXISTS (SELECT 1 FROM payments pay WHERE pay.purchase_id = p.id AND pay.external_id IS NOT NULL
		              AND pay.external_id NOT LIKE '`+syntheticPrefix+`%')
	`)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("erro ao reclassificar a caixinha: %w", err)
	}
	savings = tag.RowsAffected()

	tag, err = r.db.Pool.Exec(ctx, `
		WITH cards AS (
			SELECT DISTINCT c.name FROM accounts c
			WHERE c.type = 'CREDIT' AND LENGTH(BTRIM(c.name)) >= 10
			  AND NOT EXISTS (SELECT 1 FROM accounts b WHERE b.type = 'BANK' AND LOWER(b.name) = LOWER(c.name))
		), hit AS (
			SELECT pay.id, pay.purchase_id FROM payments pay
			JOIN purchases p ON p.id = pay.purchase_id
			JOIN accounts a ON a.id = pay.account_id
			WHERE a.type = 'BANK' AND p.kind = 'EXPENSE' AND pay.status != 'CANCELLED'
			  AND EXISTS (SELECT 1 FROM cards WHERE POSITION(LOWER(cards.name) IN LOWER(p.description)) > 0)
		), cancelled AS (
			UPDATE purchases p SET cancelled_at = COALESCE(p.cancelled_at, NOW()), cancellation_reason = $1
			WHERE p.id IN (SELECT purchase_id FROM hit)
		)
		UPDATE payments SET status = 'CANCELLED' WHERE id IN (SELECT id FROM hit)
	`, ownCardPaymentReason)
	if err != nil {
		return savings, 0, 0, fmt.Errorf("erro ao ignorar pagamento de fatura de cartão próprio: %w", err)
	}
	cardPayments = tag.RowsAffected()

	tag, err = r.db.Pool.Exec(ctx, `
		UPDATE purchases p SET kind = 'EXPENSE', category = 'OTHER', transfer_direction = NULL
		WHERE p.kind = 'TRANSFER' AND p.transfer_direction = 'OUT' AND p.type = 'SINGLE'
		  AND p.description ~* '^pagamento (com (qr )?pix|de boleto|de conta)\M'
		  AND EXISTS (SELECT 1 FROM payments pay WHERE pay.purchase_id = p.id AND pay.external_id IS NOT NULL
		              AND pay.external_id NOT LIKE '`+syntheticPrefix+`%')
	`)
	if err != nil {
		return savings, cardPayments, 0, fmt.Errorf("erro ao reclassificar pagamentos classificados como aplicação: %w", err)
	}
	return savings, cardPayments, tag.RowsAffected(), nil
}
