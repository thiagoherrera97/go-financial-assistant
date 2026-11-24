package httpserver

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// debtFake guarda as dívidas dos testes da API.
type debtFake struct {
	debts     []domain.Debt
	hits      []domain.MoneyMovement
	days      []domain.BalanceDay
	bill      float64
	outflows  []domain.MoneyMovement
	flows     []domain.RecurringFlow
	cardBills []domain.CardBillSummary
	billLines []domain.CardBillLine
	billAsked time.Time
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
func (f *fakeReader) MovementsWith(context.Context, string, time.Time) ([]domain.MoneyMovement, error) {
	return f.debt.hits, f.err
}
func (f *fakeReader) BalanceDays(context.Context, uuid.UUID, time.Time) ([]domain.BalanceDay, error) {
	return f.debt.days, f.err
}
func (f *fakeReader) CardBillTotal(context.Context, uuid.UUID, time.Time) (float64, error) {
	return f.debt.bill, f.err
}
func (f *fakeReader) BankMovements(context.Context, time.Time, time.Time) ([]domain.MoneyMovement, error) {
	return f.debt.outflows, f.err
}
func (f *fakeReader) CardSpend(context.Context, uuid.UUID, time.Time, time.Time) (float64, error) {
	return f.debt.bill, f.err
}

func (f *fakeReader) RecurringFlows(context.Context) ([]domain.RecurringFlow, error) {
	return f.debt.flows, f.err
}
func (f *fakeReader) CreateRecurringFlow(_ context.Context, e domain.RecurringFlow) error {
	f.debt.flows = append(f.debt.flows, e)
	return f.err
}
func (f *fakeReader) UpdateRecurringFlow(_ context.Context, e domain.RecurringFlow) (bool, error) {
	for i := range f.debt.flows {
		if f.debt.flows[i].ID == e.ID {
			f.debt.flows[i] = e
			return true, f.err
		}
	}
	return false, f.err
}
func (f *fakeReader) DeleteRecurringFlow(_ context.Context, id uuid.UUID) (bool, error) {
	for i := range f.debt.flows {
		if f.debt.flows[i].ID == id {
			f.debt.flows = append(f.debt.flows[:i], f.debt.flows[i+1:]...)
			return true, f.err
		}
	}
	return false, f.err
}

func (f *fakeReader) CardBills(context.Context, uuid.UUID) ([]domain.CardBillSummary, error) {
	return f.debt.cardBills, f.err
}
func (f *fakeReader) CardBillLines(_ context.Context, _ uuid.UUID, due time.Time) ([]domain.CardBillLine, error) {
	f.debt.billAsked = due
	return f.debt.billLines, f.err
}
