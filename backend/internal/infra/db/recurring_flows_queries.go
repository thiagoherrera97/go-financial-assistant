package db

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func (r *DebtsRepo) RecurringFlows(ctx context.Context) ([]domain.RecurringFlow, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, direction = 'IN', name, party, amount, step, COALESCE(max_amount, 0), day, start_month, COALESCE(installments, 0),
		       payee_match, created_at
		FROM recurring_flows ORDER BY direction, day, created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler recorrências: %w", err)
	}
	defer rows.Close()
	var result []domain.RecurringFlow
	for rows.Next() {
		var f domain.RecurringFlow
		if err := rows.Scan(&f.ID, &f.In, &f.Name, &f.Party, &f.Amount, &f.Step, &f.MaxAmount, &f.Day, &f.StartMonth, &f.Installments,
			&f.PayeeMatch, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("erro ao escanear recorrência: %w", err)
		}
		f.StartMonth = f.StartMonth.UTC()
		result = append(result, f)
	}
	return result, rows.Err()
}

// flowArgs: zero em max_amount e installments vira nulo (sem limite, sem fim).
func flowArgs(f domain.RecurringFlow) (dir string, maxAmount *float64, installments *int) {
	dir = "OUT"
	if f.In {
		dir = "IN"
	}
	if f.MaxAmount > 0 {
		maxAmount = &f.MaxAmount
	}
	if f.Installments > 0 {
		installments = &f.Installments
	}
	return dir, maxAmount, installments
}

func (r *DebtsRepo) CreateRecurringFlow(ctx context.Context, f domain.RecurringFlow) error {
	dir, maxAmount, installments := flowArgs(f)
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO recurring_flows (id, direction, name, party, amount, step, max_amount, day, start_month, installments, payee_match)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, f.ID, dir, f.Name, f.Party, f.Amount, f.Step, maxAmount, f.Day, f.StartMonth, installments, f.PayeeMatch)
	if err != nil {
		return fmt.Errorf("erro ao gravar recorrência: %w", err)
	}
	return nil
}

func (r *DebtsRepo) UpdateRecurringFlow(ctx context.Context, f domain.RecurringFlow) (bool, error) {
	dir, maxAmount, installments := flowArgs(f)
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE recurring_flows SET direction = $2, name = $3, party = $4, amount = $5, step = $6, max_amount = $7, day = $8,
			start_month = $9, installments = $10, payee_match = $11
		WHERE id = $1
	`, f.ID, dir, f.Name, f.Party, f.Amount, f.Step, maxAmount, f.Day, f.StartMonth, installments, f.PayeeMatch)
	if err != nil {
		return false, fmt.Errorf("erro ao atualizar recorrência: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *DebtsRepo) DeleteRecurringFlow(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM recurring_flows WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("erro ao apagar recorrência: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
