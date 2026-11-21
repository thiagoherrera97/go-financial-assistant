package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *VendorsRepo) VendorPlan(ctx context.Context) (domain.VendorPlan, error) {
	var p domain.VendorPlan
	err := r.db.Pool.QueryRow(ctx, `
		SELECT name, contribution_day, COALESCE(savings_box, ''), saved_manual FROM vendor_plan WHERE id
	`).Scan(&p.Name, &p.ContributionDay, &p.SavingsBox, &p.SavedManual)
	if err != nil {
		return p, fmt.Errorf("erro ao ler plano de fornecedores: %w", err)
	}
	return p, nil
}

func (r *VendorsRepo) SaveVendorPlan(ctx context.Context, p domain.VendorPlan) error {
	var box *string
	if p.SavingsBox != "" {
		box = &p.SavingsBox
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO vendor_plan (id, name, contribution_day, savings_box, saved_manual) VALUES (TRUE, $1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, contribution_day = EXCLUDED.contribution_day,
			savings_box = EXCLUDED.savings_box, saved_manual = EXCLUDED.saved_manual
	`, p.Name, p.ContributionDay, box, p.SavedManual)
	if err != nil {
		return fmt.Errorf("erro ao gravar plano de fornecedores: %w", err)
	}
	return nil
}

func (r *VendorsRepo) Vendors(ctx context.Context) ([]domain.Vendor, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT id, name, contracted, due_date, created_at FROM vendors ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler fornecedores: %w", err)
	}
	var vendors []domain.Vendor
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var v domain.Vendor
		if err := rows.Scan(&v.ID, &v.Name, &v.Contracted, &v.DueDate, &v.CreatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("erro ao escanear fornecedor: %w", err)
		}
		index[v.ID] = len(vendors)
		vendors = append(vendors, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao ler fornecedores: %w", err)
	}

	rows, err = r.db.Pool.Query(ctx, `
		SELECT vp.id, vp.vendor_id, vp.amount, vp.paid_on, vp.payment_id, COALESCE(p.description, '')
		FROM vendor_payments vp
		LEFT JOIN payments pay ON pay.id = vp.payment_id
		LEFT JOIN purchases p ON p.id = pay.purchase_id
		ORDER BY vp.paid_on, vp.created_at, vp.id
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler pagamentos de fornecedores: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.VendorPayment
		if err := rows.Scan(&p.ID, &p.VendorID, &p.Amount, &p.PaidOn, &p.TransactionID, &p.Description); err != nil {
			return nil, fmt.Errorf("erro ao escanear pagamento de fornecedor: %w", err)
		}
		if i, ok := index[p.VendorID]; ok {
			vendors[i].Payments = append(vendors[i].Payments, p)
		}
	}
	return vendors, rows.Err()
}

func (r *VendorsRepo) CreateVendor(ctx context.Context, v domain.Vendor, initial *domain.VendorPayment) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao gravar fornecedor: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO vendors (id, name, contracted, due_date) VALUES ($1, $2, $3, $4)`,
		v.ID, v.Name, v.Contracted, v.DueDate); err != nil {
		return fmt.Errorf("erro ao gravar fornecedor: %w", err)
	}
	if initial != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO vendor_payments (id, vendor_id, amount, paid_on) VALUES ($1, $2, $3, $4)`,
			initial.ID, v.ID, initial.Amount, initial.PaidOn); err != nil {
			return fmt.Errorf("erro ao gravar pagamento inicial: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("erro ao gravar fornecedor: %w", err)
	}
	return nil
}

func (r *VendorsRepo) UpdateVendor(ctx context.Context, v domain.Vendor) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `UPDATE vendors SET name = $2, contracted = $3, due_date = $4 WHERE id = $1`,
		v.ID, v.Name, v.Contracted, v.DueDate)
	if err != nil {
		return false, fmt.Errorf("erro ao alterar fornecedor: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *VendorsRepo) DeleteVendor(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM vendors WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("erro ao apagar fornecedor: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *VendorsRepo) AddVendorPayment(ctx context.Context, p domain.VendorPayment) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao gravar pagamento de fornecedor: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Trava o fornecedor para não gravar pagamento de um que está sendo apagado.
	var one int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM vendors WHERE id = $1 FOR UPDATE`, p.VendorID).Scan(&one); errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrVendorNotFound
	} else if err != nil {
		return fmt.Errorf("erro ao ler fornecedor: %w", err)
	}

	if p.TransactionID == nil {
		if _, err := tx.Exec(ctx, `INSERT INTO vendor_payments (id, vendor_id, amount, paid_on) VALUES ($1, $2, $3, $4)`,
			p.ID, p.VendorID, p.Amount, p.PaidOn); err != nil {
			return fmt.Errorf("erro ao gravar pagamento de fornecedor: %w", err)
		}
		return commit(ctx, tx.Commit)
	}

	// Ligado ao extrato: só saída não cancelada (gasto ou transferência de saída); valor e dia vêm da transação.
	tag, err := tx.Exec(ctx, `
		INSERT INTO vendor_payments (id, vendor_id, amount, paid_on, payment_id)
		SELECT $1, $2, pay.amount, `+txDate+`, pay.id
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE pay.id = $3 AND pay.status <> 'CANCELLED'
		  AND (p.kind = 'EXPENSE' OR (p.kind = 'TRANSFER' AND p.transfer_direction = 'OUT'))
		ON CONFLICT (payment_id) DO NOTHING
	`, p.ID, p.VendorID, *p.TransactionID)
	if err != nil {
		return fmt.Errorf("erro ao ligar transação ao fornecedor: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var linked bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vendor_payments WHERE payment_id = $1)`, *p.TransactionID).Scan(&linked); err != nil {
			return fmt.Errorf("erro ao ligar transação ao fornecedor: %w", err)
		}
		if linked {
			return domain.ErrVendorTxLinked
		}
		return domain.ErrVendorTxNotFound
	}
	return commit(ctx, tx.Commit)
}

func commit(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return fmt.Errorf("erro ao gravar pagamento de fornecedor: %w", err)
	}
	return nil
}

func (r *VendorsRepo) DeleteVendorPayment(ctx context.Context, vendorID, paymentID uuid.UUID) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM vendor_payments WHERE id = $1 AND vendor_id = $2`, paymentID, vendorID)
	if err != nil {
		return false, fmt.Errorf("erro ao apagar pagamento de fornecedor: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
