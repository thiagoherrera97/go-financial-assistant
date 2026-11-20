package ledger

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func strp(s string) *string { return &s }

func TestProcessQuery_Category_FiltersAndRanks(t *testing.T) {
	summaryCalled := false
	repo := &mockLedger{
		findPaymentsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentSummary, error) {
			summaryCalled = true
			return nil, nil
		},
		findPaymentDetailsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentDetail, error) {
			return []ports.PaymentDetail{
				{Description: strp("iFood"), Category: "FOOD", Amount: 40, Status: "PAID", PurchaseKind: "EXPENSE"},
				{Description: strp("Restaurante X"), Category: "FOOD", Amount: 120, Status: "PAID", PurchaseKind: "EXPENSE"},
				{Description: strp("ifood "), Category: "FOOD", Amount: 35, Status: "PENDING", PurchaseKind: "EXPENSE"},
				{Description: strp("Cancelado"), Category: "FOOD", Amount: 999, Status: "CANCELLED", PurchaseKind: "EXPENSE"},
				{Description: strp("Uber"), Category: "TRANSPORT", Amount: 50, Status: "PAID", PurchaseKind: "EXPENSE"},
				{Description: strp("Estorno"), Category: "FOOD", Amount: 20, Status: "PAID", PurchaseKind: "INCOME"},
				{Description: nil, Category: "FOOD", Amount: 5, Status: "PAID", PurchaseKind: "EXPENSE"},
			}, nil
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{
				Type:      ports.ExpenseTypeQuery,
				QueryInfo: &ports.QueryInfo{Month: 11, Year: 2026, Category: "FOOD"},
			}, nil
		},
	}

	out, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "quanto gastei com comida"})
	if err != nil {
		t.Fatalf("esperava sem erro, got: %v", err)
	}
	if summaryCalled {
		t.Error("consulta por categoria não deveria usar o resumo do mês inteiro")
	}
	if out.QueryCategory != "Alimentação" {
		t.Errorf("categoria esperada Alimentação, got %q", out.QueryCategory)
	}
	if out.QueryMonth != "Novembro 2026" {
		t.Errorf("mês esperado Novembro 2026, got %q", out.QueryMonth)
	}
	if out.QueryTotal != 200 {
		t.Errorf("total esperado 200 (sem cancelado, sem entrada, sem outra categoria), got %.2f", out.QueryTotal)
	}
	if out.QueryCount != 4 {
		t.Errorf("esperava 4 lançamentos, got %d", out.QueryCount)
	}
	if out.QueryEmpty {
		t.Error("QueryEmpty não deveria ser true")
	}
	if len(out.QueryItems) != 3 {
		t.Fatalf("esperava 3 estabelecimentos, got %d: %+v", len(out.QueryItems), out.QueryItems)
	}
	if out.QueryItems[0].Description != "Restaurante X" || out.QueryItems[0].Total != 120 {
		t.Errorf("primeiro deveria ser Restaurante X com 120, got %+v", out.QueryItems[0])
	}
	if out.QueryItems[1].Description != "iFood" || out.QueryItems[1].Total != 75 || out.QueryItems[1].Count != 2 {
		t.Errorf("segundo deveria juntar iFood (75, 2x) ignorando caixa e espaço, got %+v", out.QueryItems[1])
	}
	if out.QueryItems[2].Description != "Sem descrição" {
		t.Errorf("lançamento sem descrição deveria aparecer como 'Sem descrição', got %+v", out.QueryItems[2])
	}
}

func TestProcessQuery_Category_LimitsTopItems(t *testing.T) {
	var details []ports.PaymentDetail
	for i, name := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		details = append(details, ports.PaymentDetail{Description: strp(name), Category: "MARKET", Amount: float64(10 + i), Status: "PAID", PurchaseKind: "EXPENSE"})
	}
	repo := &mockLedger{
		findPaymentDetailsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentDetail, error) { return details, nil },
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{Type: ports.ExpenseTypeQuery, QueryInfo: &ports.QueryInfo{Category: "MARKET"}}, nil
		},
	}

	out, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "quanto gastei com mercado"})
	if err != nil {
		t.Fatalf("esperava sem erro, got: %v", err)
	}
	if len(out.QueryItems) != queryTopItems {
		t.Fatalf("esperava %d estabelecimentos, got %d", queryTopItems, len(out.QueryItems))
	}
	if out.QueryItems[0].Description != "G" {
		t.Errorf("o maior gasto deveria vir primeiro, got %+v", out.QueryItems[0])
	}
	if out.QueryCount != 7 || out.QueryTotal != 7*10+21 {
		t.Errorf("total e contagem deveriam incluir todos os lançamentos, got %.2f / %d", out.QueryTotal, out.QueryCount)
	}
}

func TestProcessQuery_Category_Empty(t *testing.T) {
	repo := &mockLedger{
		findPaymentDetailsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentDetail, error) {
			return []ports.PaymentDetail{
				{Description: strp("Uber"), Category: "TRANSPORT", Amount: 50, Status: "PAID", PurchaseKind: "EXPENSE"},
			}, nil
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{Type: ports.ExpenseTypeQuery, QueryInfo: &ports.QueryInfo{Category: "HEALTH"}}, nil
		},
	}

	out, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "quanto gastei com farmácia"})
	if err != nil {
		t.Fatalf("esperava sem erro, got: %v", err)
	}
	if !out.QueryEmpty || out.QueryCategory != "Saúde" {
		t.Errorf("esperava consulta vazia de Saúde, got empty=%v categoria=%q", out.QueryEmpty, out.QueryCategory)
	}
}
