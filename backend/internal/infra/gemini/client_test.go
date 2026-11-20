package gemini

import (
	"testing"

	"google.golang.org/genai"
)

func makeResponse(text string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: text},
					},
				},
			},
		},
	}
}

func TestParseResponse_Success(t *testing.T) {
	raw := `{"amount": 49.90, "description": "Almoço", "category": "FOOD", "confidence": 0.95}`
	analysis, err := parseResponse(makeResponse(raw))
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if analysis.Amount == nil || *analysis.Amount != 49.90 {
		t.Errorf("amount esperado 49.90, got %v", analysis.Amount)
	}
	if analysis.Description == nil || *analysis.Description != "Almoço" {
		t.Errorf("description incorreta: %v", analysis.Description)
	}
	if analysis.Category == nil || *analysis.Category != "FOOD" {
		t.Errorf("category incorreta: %v", analysis.Category)
	}
	if analysis.Confidence != 0.95 {
		t.Errorf("confidence esperada 0.95, got %v", analysis.Confidence)
	}
	if analysis.RawResponse != raw {
		t.Errorf("rawResponse não preservado")
	}
}

func TestParseResponse_AmountNull(t *testing.T) {
	raw := `{"amount": null, "description": "?", "category": "OTHER", "confidence": 0.1}`
	analysis, err := parseResponse(makeResponse(raw))
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if analysis.Amount != nil {
		t.Errorf("amount deveria ser nil, got %v", *analysis.Amount)
	}
}

func TestParseResponse_NilResponse(t *testing.T) {
	_, err := parseResponse(nil)
	if err == nil {
		t.Fatal("esperava erro para resposta nil")
	}
}

func TestParseResponse_EmptyCandidates(t *testing.T) {
	_, err := parseResponse(&genai.GenerateContentResponse{Candidates: nil})
	if err == nil {
		t.Fatal("esperava erro para candidates vazio")
	}
}

func TestParseResponse_NilContent(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: nil},
		},
	}
	_, err := parseResponse(resp)
	if err == nil {
		t.Fatal("esperava erro para content nil")
	}
}

func TestParseResponse_EmptyParts(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{Parts: []*genai.Part{}}},
		},
	}
	_, err := parseResponse(resp)
	if err == nil {
		t.Fatal("esperava erro para parts vazio")
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	_, err := parseResponse(makeResponse("não é json"))
	if err == nil {
		t.Fatal("esperava erro de JSON inválido")
	}
}

func TestToAnalysis(t *testing.T) {
	amount := 100.0
	desc := "Farmácia"
	cat := "HEALTH"
	raw := `{"amount":100,"description":"Farmácia","category":"HEALTH","confidence":0.8}`

	g := &geminiResponse{
		Amount:      &amount,
		Description: &desc,
		Category:    &cat,
		Confidence:  0.8,
	}

	analysis := g.toAnalysis(raw)

	if analysis.Amount != &amount {
		t.Error("amount não foi preservado")
	}
	if analysis.Description != &desc {
		t.Error("description não foi preservada")
	}
	if analysis.Category != &cat {
		t.Error("category não foi preservada")
	}
	if analysis.Confidence != 0.8 {
		t.Errorf("confidence esperada 0.8, got %v", analysis.Confidence)
	}
	if analysis.RawResponse != raw {
		t.Errorf("rawResponse não preservado")
	}
}

func TestParseResponse_Installment(t *testing.T) {
	raw := `{
		"type": "INSTALLMENT",
		"amount": 1200.0,
		"description": "iPhone 15",
		"category": "SHOPPING",
		"payment_method": "CREDIT_CARD",
		"confidence": 0.97,
		"installments": {"total": 12, "amount_per_installment": 100.0}
	}`
	analysis, err := parseResponse(makeResponse(raw))
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if analysis.Type != "INSTALLMENT" {
		t.Errorf("type esperado INSTALLMENT, got %s", analysis.Type)
	}
	if analysis.Installments == nil {
		t.Fatal("installments não deveria ser nil")
	}
	if analysis.Installments.Total != 12 {
		t.Errorf("total esperado 12, got %d", analysis.Installments.Total)
	}
	if analysis.Installments.AmountPerInstallment != 100.0 {
		t.Errorf("amount_per_installment esperado 100.0, got %v", analysis.Installments.AmountPerInstallment)
	}
	if analysis.PaymentMethod == nil || *analysis.PaymentMethod != "CREDIT_CARD" {
		t.Errorf("payment_method esperado CREDIT_CARD, got %v", analysis.PaymentMethod)
	}
}

func TestParseResponse_Recurring(t *testing.T) {
	raw := `{
		"type": "RECURRING",
		"amount": 55.0,
		"description": "Netflix",
		"category": "ENTERTAINMENT",
		"payment_method": "CREDIT_CARD",
		"confidence": 0.99,
		"recurring": {"day_of_month": 15}
	}`
	analysis, err := parseResponse(makeResponse(raw))
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if analysis.Type != "RECURRING" {
		t.Errorf("type esperado RECURRING, got %s", analysis.Type)
	}
	if analysis.RecurringInfo == nil {
		t.Fatal("recurring_info não deveria ser nil")
	}
	if analysis.RecurringInfo.DayOfMonth != 15 {
		t.Errorf("day_of_month esperado 15, got %d", analysis.RecurringInfo.DayOfMonth)
	}
}

func TestParseResponse_CancelRecurring(t *testing.T) {
	raw := `{
		"type": "CANCEL_RECURRING",
		"confidence": 0.95,
		"cancel_recurring": {"description": "Netflix"}
	}`
	analysis, err := parseResponse(makeResponse(raw))
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if analysis.Type != "CANCEL_RECURRING" {
		t.Errorf("type esperado CANCEL_RECURRING, got %s", analysis.Type)
	}
	if analysis.CancelInfo == nil {
		t.Fatal("cancel_info não deveria ser nil")
	}
	if analysis.CancelInfo.Description != "Netflix" {
		t.Errorf("description esperada Netflix, got %s", analysis.CancelInfo.Description)
	}
}

func TestToExpenseType(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"SINGLE", "SINGLE"},
		{"INSTALLMENT", "INSTALLMENT"},
		{"RECURRING", "RECURRING"},
		{"CANCEL_RECURRING", "CANCEL_RECURRING"},
		{"", "SINGLE"},
		{"UNKNOWN", "SINGLE"},
	}
	for _, c := range cases {
		got := toExpenseType(c.input)
		if string(got) != c.expected {
			t.Errorf("toExpenseType(%q) = %q, esperado %q", c.input, got, c.expected)
		}
	}
}

func TestToAnalysis_QueryCategory(t *testing.T) {
	cases := []struct {
		name string
		raw  *string
		want string
	}{
		{"categoria válida", strp("FOOD"), "FOOD"},
		{"minúscula e espaço", strp(" market "), "MARKET"},
		{"desconhecida é ignorada", strp("COMIDA"), ""},
		{"null", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &geminiResponse{Type: "QUERY", Query: &geminiQuery{Category: tc.raw}}
			a := g.toAnalysis("{}")
			if a.QueryInfo == nil || a.QueryInfo.Category != tc.want {
				t.Errorf("categoria esperada %q, got %+v", tc.want, a.QueryInfo)
			}
		})
	}
}

func TestToAnalysis_ExportIgnoresCategory(t *testing.T) {
	g := &geminiResponse{Type: "EXPORT_CSV", Export: &geminiQuery{Category: strp("FOOD")}}
	if a := g.toAnalysis("{}"); a.ExportInfo == nil || a.ExportInfo.Category != "" {
		t.Errorf("export não deveria levar categoria, got %+v", a.ExportInfo)
	}
}
func strp(s string) *string { return &s }
