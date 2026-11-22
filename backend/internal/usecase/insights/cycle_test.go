package insights

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// cycleReader implementa só o que a tela do ciclo e o salário leem; o resto do InsightsReader entra em pânico se for usado.
type cycleReader struct {
	ports.InsightsReader
	credits []domain.SalaryCredit
}

func (cycleReader) MonthLines(context.Context, time.Time) ([]domain.MonthLine, error) { return nil, nil }
func (cycleReader) ExpenseKeyMonths(context.Context, time.Time, time.Time) ([]domain.ExpenseKeyMonth, error) {
	return nil, nil
}
func (cycleReader) ExpenseRules(context.Context) (map[string]domain.ExpenseClass, error) { return nil, nil }
func (r cycleReader) SalaryCredits(_ context.Context, from, to time.Time) ([]domain.SalaryCredit, error) {
	var out []domain.SalaryCredit
	for _, c := range r.credits {
		if !c.Date.Before(from) && !c.Date.After(to) {
			out = append(out, c)
		}
	}
	return out, nil
}

func d(y, m, day int) time.Time { return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC) }

func cycleInsights(credits []domain.SalaryCredit) *Insights {
	i := &Insights{reader: cycleReader{credits: credits}}
	parts := []domain.SalaryPart{{Day: 15, Amount: 5200}, {Day: 30, Amount: 4366.62}}
	i.SetSalarySchedule(func() []domain.SalaryPart { return parts })
	cyc := domain.NewCycles(domain.BuildCycles(parts, credits, d(2026, 10, 1), d(2027, 1, 1)))
	i.SetCycles(func() domain.Cycles { return cyc })
	return i
}

// Ciclo de novembro em 21/nov: a 1ª parte já caiu (13/nov), a de 30/nov ainda está a receber.
func TestMonthSheetUsesTheCyclesOwnSalary(t *testing.T) {
	i := cycleInsights([]domain.SalaryCredit{{Date: d(2026, 10, 15), Amount: 5200}, {Date: d(2026, 10, 30), Amount: 4366.62}, {Date: d(2026, 11, 13), Amount: 5200}})
	s, err := i.MonthSheet(context.Background(), d(2026, 11, 1), d(2026, 11, 21))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Pending) != 1 || !s.Pending[0].Date.Equal(d(2026, 11, 30)) || s.PendingTotal != 4366.62 {
		t.Errorf("a receber no ciclo de novembro = %+v (total %v), want só a parte de 30/nov", s.Pending, s.PendingTotal)
	}
}

// Em 5/dez o ciclo ainda é o de novembro (já recebido): a previsão precisa trazer as partes de 15/dez e 30/dez.
func TestSalaryDueEarlyInTheMonth(t *testing.T) {
	i := cycleInsights([]domain.SalaryCredit{{Date: d(2026, 11, 13), Amount: 5200}, {Date: d(2026, 11, 30), Amount: 4366.62}})
	lines, err := i.salaryDue(context.Background(), d(2026, 12, 5), d(2026, 12, 31))
	if err != nil {
		t.Fatal(err)
	}
	var got []time.Time
	for _, l := range lines {
		got = append(got, *l.Date)
	}
	if len(got) < 2 || !got[0].Equal(d(2026, 12, 15)) || !got[1].Equal(d(2026, 12, 30)) {
		t.Errorf("salário previsto a partir de 5/dez = %v, want 15/dez e 30/dez", got)
	}
}
