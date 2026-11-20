package planning

import (
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func TestBuildMonthSheet(t *testing.T) {
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	card := func(label, cat string, amount float64, synthetic string) domain.MonthLine {
		return domain.MonthLine{Key: label, Label: label, Category: cat, Kind: "EXPENSE", Amount: amount, AccountID: "c1", AccountName: "SANTANDER", Card: true, Synthetic: synthetic}
	}
	lines := []domain.MonthLine{
		{Key: "liquido", Label: "LIQUIDO DE VENCIMENTO", Category: "SALARY", Kind: "INCOME", Amount: 5200},
		{Key: "venda", Label: "Venda Notebook", Category: "OTHER", Kind: "INCOME", Amount: 4422.40},
		{Key: "financiamento", Label: "Financiamento do carro", Category: "TRANSPORT", Kind: "EXPENSE", Amount: 2086.68},
		{Key: "aluguel", Label: "Aluguel", Category: "HOUSING", Kind: "EXPENSE", Amount: 1100},
		{Key: "padaria", Label: "Padaria", Category: "FOOD", Kind: "EXPENSE", Amount: 30},
		{Key: "padaria", Label: "Padaria", Category: "FOOD", Kind: "EXPENSE", Amount: 20},
		{Key: "pix lavar", Label: "Pix lavar o carro", Category: "OTHER", Kind: "EXPENSE", Amount: 60},
		card("ifood", "FOOD", 500, ""),
		card("tiktok", "BILLS", 279.89, ""),
		card("tiktok", "BILLS", -279.89, ""),
		card("vivara", "SHOPPING", 669, "forecast"),
		card("ajuste", "OTHER", 279.89, "bill"),
		{Key: "cdb", Label: "CDB", Category: "INVESTMENT", Kind: "TRANSFER", Direction: "OUT", Amount: 1000},
		{Key: "cdb", Label: "CDB", Category: "INVESTMENT", Kind: "TRANSFER", Direction: "IN", Amount: 300},
	}
	classes := map[string]domain.ExpenseClass{"financiamento": domain.ClassInstallment, "aluguel": domain.ClassFixed, "padaria": domain.ClassVariable}
	salary := []domain.SalaryStatus{
		{Day: 15, Amount: 5200, Received: &domain.SalaryCredit{Amount: 5200}},
		{Day: 30, Expected: time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC), Amount: 4366.62},
	}

	s := BuildMonthSheet(nov, lines, classes, salary)

	if s.IncomeTotal != 9622.40 || len(s.Income) != 2 || s.Income[0].Label != salaryLabel {
		t.Errorf("entradas: %v %+v", s.IncomeTotal, s.Income)
	}
	if s.PendingTotal != 4366.62 || len(s.Pending) != 1 || s.Pending[0].Label != "Salário do dia 30" {
		t.Errorf("a receber: %+v", s.Pending)
	}
	if s.FixedTotal != 3186.68 || len(s.Fixed) != 2 || s.Fixed[0].Label != "Financiamento do carro" {
		t.Errorf("fixas: %v %+v", s.FixedTotal, s.Fixed)
	}
	if s.OtherTotal != 110 || len(s.Other) != 2 || s.Other[0].Label != "Pix lavar o carro" || s.Other[1].Count != 2 {
		t.Errorf("outras: %v %+v", s.OtherTotal, s.Other)
	}
	if len(s.Cards) != 1 {
		t.Fatalf("cartões: %+v", s.Cards)
	}
	c := s.Cards[0]
	// Compras 500 + 279,89 − estorno 279,89 + prevista 669 + ajuste 279,89.
	if c.Total != 1448.89 || c.Forecast != 669 || c.Adjustment != 279.89 || c.Refunds != -279.89 {
		t.Errorf("cartão: %+v", c)
	}
	if len(c.Categories) != 1 || c.Categories[0].Amount != 500 {
		t.Errorf("categorias do cartão (estorno zera Contas): %+v", c.Categories)
	}
	if s.ExpensesTotal != 4745.57 || s.Leftover != 4876.83 || s.LeftoverWithPending != 9243.45 || s.Invested != 700 {
		t.Errorf("totais: despesas %v sobra %v com a receber %v investido %v", s.ExpensesTotal, s.Leftover, s.LeftoverWithPending, s.Invested)
	}
}
