package domain

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCardBillStates(t *testing.T) {
	total := 100.0
	bills := []CardBillSummary{
		{Due: day("2026-10-10"), BankTotal: &total},
		{Due: day("2026-11-10"), BankTotal: &total},
		{Due: day("2026-12-10")},
		{Due: day("2027-01-10")},
	}
	got := CardBillStates(bills, time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC))
	want := []CardBillState{CardBillClosed, CardBillClosed, CardBillOpen, CardBillFuture}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("fatura %d: got %s, want %s", i, got[i], want[i])
		}
	}

	// Já venceu e o banco ainda não mandou o total: continua aberta, e a seguinte também é aberta (a do ciclo atual).
	late := []CardBillSummary{{Due: day("2026-11-10")}, {Due: day("2026-12-10")}, {Due: day("2027-01-10")}}
	got = CardBillStates(late, day("2026-11-24"))
	want = []CardBillState{CardBillOpen, CardBillOpen, CardBillFuture}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("atrasada %d: got %s, want %s", i, got[i], want[i])
		}
	}

	// O vencimento de hoje ainda é a aberta.
	if s := CardBillStates([]CardBillSummary{{Due: day("2026-11-24")}}, time.Date(2026, 11, 24, 23, 0, 0, 0, time.UTC)); s[0] != CardBillOpen {
		t.Errorf("vence hoje: got %s", s[0])
	}
}

func TestCardBillSummaryTotals(t *testing.T) {
	bank := 1000.0
	b := CardBillSummary{BankTotal: &bank, Detailed: 850.10, Forecast: 100.20, Gap: 49.70}
	if b.Known() != 950.30 {
		t.Errorf("Known = %v", b.Known())
	}
	if b.Total() != 1000 {
		t.Errorf("Total = %v", b.Total())
	}
	if b.Difference() != 49.70 {
		t.Errorf("Difference = %v", b.Difference())
	}
	b.Detailed = 899.90 // diferença de 10 centavos: dentro da tolerância
	if b.Difference() != 0 {
		t.Errorf("Difference dentro da tolerância = %v", b.Difference())
	}
	if (CardBillSummary{Detailed: 10}).Difference() != 0 {
		t.Error("sem total do banco não há diferença")
	}
}
