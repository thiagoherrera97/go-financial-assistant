package domain

import "testing"

func TestMonthTotals_OtherShare(t *testing.T) {
	cases := []struct {
		name string
		m    MonthTotals
		want *float64
	}{
		{"mês sem despesas", MonthTotals{Income: 500}, nil},
		{"despesa zero", MonthTotals{Expense: 0, OtherExpense: 0}, nil},
		{"nada em Outros", MonthTotals{Expense: 100}, ptr(0)},
		{"30%", MonthTotals{Expense: 100, OtherExpense: 30}, ptr(0.3)},
		{"tudo em Outros", MonthTotals{Expense: 80, OtherExpense: 80}, ptr(1)},
		{"nunca passa de 1 nem fica abaixo de 0", MonthTotals{Expense: 10, OtherExpense: 50}, ptr(1)},
	}
	for _, tc := range cases {
		got := tc.m.OtherShare()
		switch {
		case tc.want == nil && got != nil, tc.want != nil && got == nil, tc.want != nil && *got != *tc.want:
			t.Errorf("%s: got %v, esperado %v", tc.name, got, tc.want)
		}
	}
}

func ptr(v float64) *float64 { return &v }
