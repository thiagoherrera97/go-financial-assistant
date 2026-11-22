package insights

import (
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/debts"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestExpectedIncomes(t *testing.T) {
	start, end, today := day(2026, 11, 13), day(2026, 12, 14), day(2026, 11, 21)
	salary := []domain.SalaryStatus{
		{Day: 15, Expected: day(2026, 11, 13), Amount: 5200, Received: &domain.SalaryCredit{Date: day(2026, 11, 13), Amount: 5200}},
		{Day: 30, Expected: day(2026, 11, 30), Amount: 4366.62},
	}
	mom := domain.RecurringFlow{In: true, Name: "Dívida antiga", Party: "Mãe", Amount: 300, Day: 5, StartMonth: day(2026, 1, 1)}
	late := domain.RecurringFlow{In: true, Name: "Reembolso", Amount: 150, Day: 20, StartMonth: day(2026, 1, 1)}
	paid := domain.RecurringFlow{In: true, Name: "Aluguel do quarto", Amount: 800, Day: 15, StartMonth: day(2026, 1, 1)}
	rent := domain.RecurringFlow{In: false, Name: "Aluguel", Amount: 2000, Day: 20, StartMonth: day(2026, 1, 1)}
	flows := []FlowItem{
		{Flow: mom},
		{Flow: late},
		{Flow: paid, View: debts.FlowView{Matches: []domain.MoneyMovement{{Date: day(2026, 11, 16), Amount: 800, In: true}}}},
		{Flow: rent},
	}

	got := expectedIncomes(salary, flows, start, end, today)

	// Mãe, dia 5: a de novembro cai antes do ciclo (começa 13/11); a de dezembro entra. Aluguel do quarto já chegou;
	// Aluguel é saída. Reembolso (20/11) passou da data sem chegar: atrasado.
	want := []struct {
		label string
		late  bool
	}{
		{"Reembolso", true},
		{"Salário · 2ª parte", false},
		{"Dívida antiga (Mãe)", false},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for n, w := range want {
		if got[n].Label != w.label || got[n].Late != w.late {
			t.Errorf("%d: got %+v, want %+v", n, got[n], w)
		}
	}
	if got[1].Amount != 4366.62 || got[1].Source != "SALARY" || got[2].Source != "RECURRING" || got[2].Amount != 300 {
		t.Errorf("valores: %+v", got)
	}
}

func TestExpectedIncomes_SinglePartLabel(t *testing.T) {
	got := expectedIncomes([]domain.SalaryStatus{{Day: 5, Expected: day(2026, 11, 5), Amount: 3000}}, nil, day(2026, 11, 5), day(2026, 12, 4), day(2026, 11, 1))
	if len(got) != 1 || got[0].Label != "Salário" || got[0].Late {
		t.Fatalf("got %+v", got)
	}
}
