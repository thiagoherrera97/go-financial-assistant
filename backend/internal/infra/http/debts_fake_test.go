package httpserver

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// debtFake guarda as dívidas dos testes da API.
type debtFake struct {
	debts    []domain.Debt
	hits     []domain.MoneyMovement
	days     []domain.BalanceDay
	bill     float64
	outflows []domain.MoneyMovement
}

func (f *fakeReader) Debts(context.Context) ([]domain.Debt, error) { return f.debt.debts, f.err }
func (f *fakeReader) CreateDebt(_ context.Context, d domain.Debt) error {
	f.debt.debts = append(f.debt.debts, d)
	return f.err
}
func (f *fakeReader) UpdateDebt(_ context.Context, d domain.Debt) (bool, error) {
	for i := range f.debt.debts {
		if f.debt.debts[i].ID == d.ID {
			f.debt.debts[i] = d
			return true, f.err
		}
	}
	return false, f.err
}
func (f *fakeReader) DeleteDebt(_ context.Context, id uuid.UUID) (bool, error) {
	for i := range f.debt.debts {
		if f.debt.debts[i].ID == id {
			f.debt.debts = append(f.debt.debts[:i], f.debt.debts[i+1:]...)
			return true, f.err
		}
	}
	return false, f.err
}
func (f *fakeReader) PaymentsMatching(context.Context, string, time.Time) ([]domain.MoneyMovement, error) {
	return f.debt.hits, f.err
}
func (f *fakeReader) BalanceDays(context.Context, uuid.UUID, time.Time) ([]domain.BalanceDay, error) {
	return f.debt.days, f.err
}
func (f *fakeReader) CardBillTotal(context.Context, uuid.UUID, time.Time) (float64, error) {
	return f.debt.bill, f.err
}
func (f *fakeReader) AccountOutflows(context.Context, uuid.UUID, time.Time, time.Time) ([]domain.MoneyMovement, error) {
	return f.debt.outflows, f.err
}
