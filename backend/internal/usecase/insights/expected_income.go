package insights

import (
	"context"
	"sort"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/debts"
)

// ExpectedIncome é uma receita que já se sabe que vai cair no ciclo e ainda não chegou do banco: uma parte do salário
// ou uma entrada recorrente cadastrada. Quando o crédito real chega na sincronização, ela deixa de ser prevista.
type ExpectedIncome struct {
	Source string // "SALARY" ou "RECURRING"
	Label  string
	Date   time.Time
	Amount float64
	Late   bool // a data passou e o crédito ainda não chegou
}

// ExpectedIncomes lista as receitas previstas do ciclo key que ainda não chegaram. Ciclos já encerrados não têm
// previstas: o que não veio, não veio.
func (i *Insights) ExpectedIncomes(ctx context.Context, key, now time.Time) ([]ExpectedIncome, error) {
	today := civilDay(now)
	start, end := i.Cycles().Bounds(key)
	if end.Before(today) {
		return nil, nil
	}
	salary, err := i.salaryFor(ctx, key)
	if err != nil {
		return nil, err
	}
	flows, err := i.RecurringFlows(ctx, now)
	if err != nil {
		return nil, err
	}
	return expectedIncomes(salary.Parts, flows, start, end, today), nil
}

func expectedIncomes(salary []domain.SalaryStatus, flows []FlowItem, start, end, today time.Time) []ExpectedIncome {
	var out []ExpectedIncome
	for n, s := range salary {
		if s.Received != nil {
			continue
		}
		label := "Salário"
		if len(salary) > 1 {
			label = "Salário · " + ordinal(n+1) + " parte"
		}
		out = append(out, ExpectedIncome{Source: "SALARY", Label: label, Date: s.Expected, Amount: s.Amount, Late: s.Expected.Before(today)})
	}
	out = append(out, expectedFlows(flows, true, start, end, today)...)
	sort.SliceStable(out, func(a, b int) bool { return out[a].Date.Before(out[b].Date) })
	return out
}

// ExpectedExpense é uma saída recorrente cadastrada (aluguel, seguro, financiamento) que cai no ciclo e ainda não saiu
// do banco. Quando o débito real chega na sincronização, ela deixa de ser prevista.
type ExpectedExpense = ExpectedIncome

// ExpectedExpenses lista as saídas recorrentes previstas do ciclo key que ainda não saíram. Como nas receitas, ciclo
// encerrado não tem previstas.
func (i *Insights) ExpectedExpenses(ctx context.Context, key, now time.Time) ([]ExpectedExpense, error) {
	today := civilDay(now)
	start, end := i.Cycles().Bounds(key)
	if end.Before(today) {
		return nil, nil
	}
	flows, err := i.RecurringFlows(ctx, now)
	if err != nil {
		return nil, err
	}
	out := expectedFlows(flows, false, start, end, today)
	sort.SliceStable(out, func(a, b int) bool { return out[a].Date.Before(out[b].Date) })
	return out, nil
}

// expectedFlows: as ocorrências dos fluxos recorrentes de um sentido (in = entradas) que caem entre start e end e ainda
// não têm a transação correspondente naquele mês.
func expectedFlows(flows []FlowItem, in bool, start, end, today time.Time) []ExpectedIncome {
	var out []ExpectedIncome
	for _, it := range flows {
		f := it.Flow
		if f.In != in {
			continue
		}
		done := map[time.Time]bool{}
		for _, m := range it.View.Matches {
			done[MonthStart(m.Date)] = true
		}
		// Um ciclo pega o fim de um mês e o começo do outro: olha as ocorrências dos dois.
		for m := MonthStart(start); !m.After(end); m = m.AddDate(0, 1, 0) {
			amount, ok := debts.FlowAmount(f, m)
			date := domain.DebtDueIn(m, f.Day)
			if !ok || done[m] || date.Before(start) || date.After(end) {
				continue
			}
			label := f.Name
			if f.Party != "" {
				label += " (" + f.Party + ")"
			}
			out = append(out, ExpectedIncome{Source: "RECURRING", Label: label, Date: date, Amount: amount, Late: date.Before(today)})
		}
	}
	return out
}

func ordinal(n int) string {
	return [...]string{"", "1ª", "2ª", "3ª", "4ª", "5ª", "6ª"}[min(n, 6)]
}
