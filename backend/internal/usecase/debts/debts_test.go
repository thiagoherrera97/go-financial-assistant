package debts

import (
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
func ptr(t time.Time) *time.Time               { return &t }

func macMini() domain.Debt {
	return domain.Debt{Kind: domain.DebtInstallments, Name: "Mac mini", OriginalAmount: 3200, InstallmentAmount: 320,
		Installments: 10, FirstDue: day(2026, 12, 10), DueDay: 10, PayeeMatch: "Maria"}
}

func TestInstallmentsNothingPaid(t *testing.T) {
	v := Installments(macMini(), nil, day(2026, 11, 21))
	if v.Paid != 0 || v.Remaining != 3200 || v.Done || v.Overdue {
		t.Fatalf("got %+v", v)
	}
	if !v.NextDue.Equal(day(2026, 12, 10)) {
		t.Fatalf("next due %v", v.NextDue)
	}
}

func TestInstallmentsCountsPixAndManual(t *testing.T) {
	d := macMini()
	d.PaidManual = 1
	hits := []domain.MoneyMovement{
		{Date: day(2027, 1, 9), Amount: 320, Label: "PIX ENVIADO Maria"},
		{Date: day(2027, 1, 20), Amount: 50, Label: "PIX ENVIADO Maria"},             // não é parcela
		{Date: day(2027, 1, 20), Amount: 800, Label: "PIX ENVIADO Maria"},            // não é múltiplo da parcela
		{Date: day(2027, 1, 21), Amount: 320, Label: "PIX RECEBIDO Maria", In: true}, // recebido não abate
		{Date: day(2026, 6, 1), Amount: 320, Label: "PIX ENVIADO Maria"},             // antes da dívida
		{Date: day(2027, 2, 9), Amount: 640, Label: "PIX ENVIADO Maria"},             // duas parcelas de uma vez
	}
	v := Installments(d, hits, day(2027, 2, 21))
	if v.Paid != 4 || v.Remaining != 1920 || len(v.Payments) != 2 {
		t.Fatalf("got %+v", v)
	}
	if !v.NextDue.Equal(day(2027, 4, 10)) || v.Overdue {
		t.Fatalf("next due %v overdue %v", v.NextDue, v.Overdue)
	}
}

func TestInstallmentsOverdueAndDone(t *testing.T) {
	v := Installments(macMini(), nil, day(2026, 12, 11))
	if !v.Overdue {
		t.Fatal("parcela de 10/12 sem Pix em 11/12 deveria estar atrasada")
	}
	d := macMini()
	d.PaidManual = 10
	if v := Installments(d, nil, day(2027, 12, 1)); !v.Done || v.NextDue != nil || v.Remaining != 0 {
		t.Fatalf("got %+v", v)
	}
}

func TestUsageStreakAndMonths(t *testing.T) {
	days := []domain.BalanceDay{
		{Day: day(2026, 11, 9), Min: 100}, {Day: day(2026, 11, 10), Min: -800}, {Day: day(2026, 11, 11), Min: -1200},
		{Day: day(2026, 11, 12), Min: -1000},
	}
	u := Usage(-900, days, day(2026, 11, 12))
	if u.InLimit != 900 || u.DaysUsed != 3 || !u.Since.Equal(day(2026, 11, 10)) {
		t.Fatalf("got %+v", u)
	}
	if len(u.Months) != 1 || u.Months[0].Days != 3 || u.Months[0].Worst != 1200 {
		t.Fatalf("months %+v", u.Months)
	}
	if u := Usage(50, days, day(2026, 11, 13)); u.InLimit != 0 || u.Since != nil || u.DaysUsed != 0 {
		t.Fatalf("positiva: %+v", u)
	}
}

func TestBuildForecastDipAndExit(t *testing.T) {
	f := BuildForecast(ForecastInput{
		Today: day(2026, 11, 21), Dip: day(2026, 12, 10), Balance: 1000, FreeDays: 10,
		Salaries: []Line{
			{Label: "Salário dia 30", Amount: 4366.62, Date: ptr(day(2026, 11, 30))},
			{Label: "Salário dia 15", Amount: 5200, Date: ptr(day(2026, 12, 15))},
			{Label: "Salário dia 30", Amount: 4366.62, Date: ptr(day(2026, 12, 30))},
		},
		Bills: []Line{{Label: "Fatura", Amount: -6000, Date: ptr(day(2026, 12, 10))}, {Label: "Mac mini", Amount: -320}},
		Usual: &Line{Label: "costume", Amount: -2500},
	})
	// 1000 + 4366,62 - 6000 - 320 - 2500 = -3453,38
	if f.Balance != -3453.38 || f.InLimit != 3453.38 {
		t.Fatalf("balance %v inLimit %v", f.Balance, f.InLimit)
	}
	if f.ExitDate == nil || !f.ExitDate.Equal(day(2026, 12, 15)) || f.Days != 5 || !f.WithinFree {
		t.Fatalf("exit %v days %d free %v", f.ExitDate, f.Days, f.WithinFree)
	}
	if len(f.Lines) != 4 {
		t.Fatalf("lines %+v", f.Lines)
	}
}

func TestBuildForecastStaysPositive(t *testing.T) {
	f := BuildForecast(ForecastInput{Today: day(2026, 11, 21), Dip: day(2026, 12, 10), Balance: 8000, FreeDays: 10,
		Bills: []Line{{Label: "Fatura", Amount: -6000}}})
	if f.InLimit != 0 || f.ExitDate != nil || !f.WithinFree {
		t.Fatalf("got %+v", f)
	}
}

func TestBuildForecastBeyondFreeDays(t *testing.T) {
	f := BuildForecast(ForecastInput{Today: day(2026, 11, 21), Dip: day(2026, 12, 10), Balance: 0, FreeDays: 10,
		Salaries: []Line{{Label: "Salário dia 30", Amount: 9000, Date: ptr(day(2026, 12, 30))}},
		Bills:    []Line{{Label: "Fatura", Amount: -5000}}})
	if f.Days != 20 || f.WithinFree {
		t.Fatalf("got %+v", f)
	}
}

func TestNextDip(t *testing.T) {
	if d := NextDip(day(2026, 11, 21), 10); !d.Equal(day(2026, 12, 10)) {
		t.Fatal(d)
	}
	if d := NextDip(day(2026, 11, 10), 10); !d.Equal(day(2026, 11, 10)) {
		t.Fatal(d)
	}
	if d := NextDip(day(2027, 1, 31), 31); !d.Equal(day(2027, 1, 31)) {
		t.Fatal(d)
	}
	if d := NextDip(day(2027, 2, 1), 31); !d.Equal(day(2027, 2, 28)) {
		t.Fatal(d)
	}
}

func TestMatchesPayeeSeveralNames(t *testing.T) {
	names := "Monica Nunes de Andrade; Monica Nunes Herrera"
	for _, label := range []string{"PIX ENVIADO   MÔNICA  NUNES DE ANDRADE", "Pix recebido Monica Nunes De Andrade", "TED RECEBIDA   Monica Nunes Herrera"} {
		if !domain.MatchesPayee(label, names) {
			t.Fatalf("deveria reconhecer %q", label)
		}
	}
	if domain.MatchesPayee("PIX ENVIADO Monica Silva", names) || domain.MatchesPayee("PIX", " ; ") {
		t.Fatal("não deveria reconhecer")
	}
}
