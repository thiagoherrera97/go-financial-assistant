package chat

import (
	"strings"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"
)

func TestFormatQueryReply_Category(t *testing.T) {
	out := &ledger.ExpenseOutput{
		Type:          "QUERY",
		QueryMonth:    "Novembro 2026",
		QueryCategory: "Alimentação",
		QueryTotal:    200,
		QueryCount:    4,
		QueryItems: []ledger.CategoryItem{
			{Description: "Restaurante X", Total: 120, Count: 1},
			{Description: "iFood", Total: 75, Count: 2},
		},
	}

	got := formatReply(out)

	for _, want := range []string{"Alimentação em Novembro 2026", "Total: R$ 200.00 (4 lançamentos)", "Restaurante X: R$ 120.00\n", "iFood: R$ 75.00 (2x)"} {
		if !strings.Contains(got, want) {
			t.Errorf("resposta deveria conter %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Resumo de") {
		t.Errorf("consulta por categoria não deveria mostrar o resumo do mês, got:\n%s", got)
	}
}

func TestFormatQueryReply_CategoryEmpty(t *testing.T) {
	out := &ledger.ExpenseOutput{Type: "QUERY", QueryMonth: "Novembro 2026", QueryCategory: "Saúde", QueryEmpty: true}

	if got := formatReply(out); got != "📊 Sem gastos com Saúde em Novembro 2026." {
		t.Errorf("resposta inesperada: %q", got)
	}
}

func TestFormatQueryReply_CategorySingular(t *testing.T) {
	out := &ledger.ExpenseOutput{Type: "QUERY", QueryMonth: "Novembro 2026", QueryCategory: "Saúde", QueryTotal: 30, QueryCount: 1}

	if got := formatReply(out); !strings.Contains(got, "(1 lançamento)") {
		t.Errorf("esperava singular, got:\n%s", got)
	}
}
