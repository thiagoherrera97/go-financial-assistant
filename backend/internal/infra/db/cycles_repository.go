package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// CycleRepo guarda os ciclos do salário (tabela cycles), lidos pelo SQL em cycle_of.
type CycleRepo struct{ db *DB }

func NewCycleRepo(db *DB) *CycleRepo { return &CycleRepo{db: db} }

// ReplaceCycles troca todos os ciclos de uma vez, numa transação: nenhuma consulta vê a tabela pela metade.
func (r *CycleRepo) ReplaceCycles(ctx context.Context, starts []domain.CycleStart) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao gravar ciclos: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM cycles`); err != nil {
		return fmt.Errorf("erro ao gravar ciclos: %w", err)
	}
	for _, s := range starts {
		if _, err := tx.Exec(ctx, `INSERT INTO cycles (month, starts_on) VALUES ($1, $2)`, s.Key, s.Start); err != nil {
			return fmt.Errorf("erro ao gravar ciclos: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// LoadCycles lê os ciclos gravados, em ordem.
func (r *CycleRepo) LoadCycles(ctx context.Context) ([]domain.CycleStart, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT month, starts_on FROM cycles ORDER BY starts_on`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler ciclos: %w", err)
	}
	defer rows.Close()
	var out []domain.CycleStart
	for rows.Next() {
		var s domain.CycleStart
		if err := rows.Scan(&s.Key, &s.Start); err != nil {
			return nil, fmt.Errorf("erro ao ler ciclos: %w", err)
		}
		s.Key, s.Start = s.Key.UTC(), s.Start.UTC()
		out = append(out, s)
	}
	return out, rows.Err()
}

// OldestPaymentDate é o dia do lançamento mais antigo (ok=false sem lançamentos): de onde os ciclos precisam começar.
func (r *CycleRepo) OldestPaymentDate(ctx context.Context) (time.Time, bool, error) {
	var d *time.Time
	if err := r.db.Pool.QueryRow(ctx, `SELECT MIN(`+txDate+`)::date FROM payments pay`).Scan(&d); err != nil {
		return time.Time{}, false, fmt.Errorf("erro ao ler o lançamento mais antigo: %w", err)
	}
	if d == nil {
		return time.Time{}, false, nil
	}
	return d.UTC(), true, nil
}
