package debts

import (
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func desmame() domain.RecurringFlow {
	return domain.RecurringFlow{In: true, Name: "Desmame do aluguel", Party: "Mãe", Amount: 400, Step: 100, MaxAmount: 1300, Day: 10,
		StartMonth: day(2026, 11, 1), PayeeMatch: "Monica Nunes de Andrade"}
}

func TestFlowAmountGrowsUntilCap(t *testing.T) {
	f := desmame()
	for _, c := range []struct {
		m    int
		want float64
		ok   bool
	}{{10, 0, false}, {11, 400, true}, {12, 500, true}} {
		if got, ok := FlowAmount(f, day(2026, timeMonth(c.m), 1)); got != c.want || ok != c.ok {
			t.Fatalf("mês %d: %v %v", c.m, got, ok)
		}
	}
	if got, _ := FlowAmount(f, day(2028, 1, 1)); got != 1300 {
		t.Fatalf("limite: %v", got)
	}
}

func TestFlowStatusInstallments(t *testing.T) {
	f := domain.RecurringFlow{In: true, Name: "Insulfilm", Amount: 113, Day: 10, StartMonth: day(2026, 10, 1), Installments: 10,
		PayeeMatch: "Monica Nunes de Andrade"}
	moves := []domain.MoneyMovement{
		{Date: day(2026, 10, 8), Amount: 113, Label: "Pix recebido Monica Nunes De Andrade", In: true},
		{Date: day(2026, 11, 10), Amount: 109, Label: "Pix recebido MONICA NUNES DE ANDRADE", In: true}, // outro valor
		{Date: day(2026, 11, 10), Amount: 113, Label: "Pix recebido MONICA NUNES DE ANDRADE", In: true},
		{Date: day(2026, 11, 10), Amount: 113, Label: "Pix enviado Monica Nunes de Andrade"}, // outro sentido
	}
	v := FlowStatus(f, moves, day(2026, 11, 21))
	if v.Done != 2 || !v.ThisMonthDone || v.Remaining != 8*113 || !v.NextDate.Equal(day(2026, 12, 10)) || len(v.Matches) != 2 {
		t.Fatalf("got %+v", v)
	}
	lines := FlowLines(f, v, day(2026, 11, 21), day(2027, 1, 14))
	if len(lines) != 2 || lines[0].Amount != 113 || lines[0].Label != "Insulfilm · 3/10" {
		t.Fatalf("lines %+v", lines)
	}
}

func TestFlowLinesLateAndOut(t *testing.T) {
	f := domain.RecurringFlow{Name: "Aluguel", Party: "irmão", Amount: 1300, Day: 10, StartMonth: day(2026, 1, 1)}
	v := FlowStatus(f, nil, day(2026, 11, 21))
	lines := FlowLines(f, v, day(2026, 11, 21), day(2026, 12, 14))
	if len(lines) != 2 || lines[0].Amount != -1300 || !lines[0].Date.Equal(day(2026, 11, 21)) || lines[1].Label != "Aluguel (irmão)" {
		t.Fatalf("lines %+v", lines)
	}
}

func TestFlowStatusVaryingBill(t *testing.T) {
	f := domain.RecurringFlow{Name: "Seguro do carro", Amount: 441.04, Day: 10, StartMonth: day(2026, 9, 1), PayeeMatch: "PORTOSEG"}
	moves := []domain.MoneyMovement{
		{Date: day(2026, 10, 9), Amount: 345, Label: "Pagamento de conta PORTOSEG SA"},
		{Date: day(2026, 11, 10), Amount: 441.04, Label: "Pagamento de conta PORTOSEG SA"},
	}
	if v := FlowStatus(f, moves, day(2026, 11, 21)); v.Done != 2 || !v.ThisMonthDone {
		t.Fatalf("got %+v", v)
	}
}
