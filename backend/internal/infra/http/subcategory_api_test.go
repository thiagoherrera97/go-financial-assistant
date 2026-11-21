package httpserver

import (
	"strings"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func TestAPI_SetClassification(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)
	url := "/api/transactions/6f1c2d3e-0000-4000-8000-000000000001/classification"
	put := func(body string) int { return do(s, "PUT", url, body, jsonHeader, c).Code }

	if rec := do(s, "PUT", url, `{"category":"TRANSPORT","subcategory":"TRANSPORT_WASH","essential":false,"applyToAll":true}`, jsonHeader, c); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"others":2`) {
		t.Fatalf("gravar: %d %s", rec.Code, rec.Body)
	}
	got := r.classified[0]
	if got.Category != domain.CategoryTransport || got.Subcategory != "TRANSPORT_WASH" || got.Essential == nil || *got.Essential || !got.ApplyToAll {
		t.Errorf("classificação = %+v", got)
	}
	if put(`{"category":"FOOD"}`) != 200 || r.classified[1].Essential != nil {
		t.Error("só a categoria, com essencial automático, é válido")
	}
	for name, body := range map[string]string{
		"subcategoria de outra categoria": `{"category":"FOOD","subcategory":"TRANSPORT_WASH"}`,
		"subcategoria inventada":          `{"category":"FOOD","subcategory":"BANANA"}`,
		"categoria de renda":              `{"category":"SALARY"}`,
		"essencial texto":                 `{"category":"FOOD","essential":"sim"}`,
	} {
		if code := put(body); code != 400 {
			t.Errorf("%s: %d, esperava 400", name, code)
		}
	}
	if code := do(s, "PUT", "/api/transactions/nao-uuid/classification", `{"category":"FOOD"}`, jsonHeader, c).Code; code != 404 {
		t.Errorf("id inválido = %d", code)
	}
	if code := do(s, "PUT", url, `{"category":"FOOD"}`, jsonHeader).Code; code != 401 {
		t.Errorf("sem sessão = %d", code)
	}
}

func TestAPI_SubcategorySpend(t *testing.T) {
	no, yes := false, true
	r := &fakeReader{subSpend: []domain.SubcategorySpend{
		{Category: domain.CategoryFood, Subcategory: "FOOD_DELIVERY", Essential: &no, Count: 3, Total: 120, PrevAverage: 80},
		{Category: domain.CategoryMarket, Subcategory: "MARKET_GROCERY", Essential: &yes, Count: 2, Total: 500},
		{Category: domain.CategoryOther, Count: 1, Total: 40},
	}}
	s := newTestAPI(t, r)
	c := login(t, s)
	rec := do(s, "GET", "/api/spending/subcategories?month=2026-11", "", nil, c)
	body := rec.Body.String()
	for _, want := range []string{`"superfluous":120`, `"essential":500`, `"unknown":40`, `"subcategoryLabel":"Delivery"`, `"subcategoryLabel":"Outros"`} {
		if !strings.Contains(body, want) {
			t.Errorf("faltou %s em %s", want, body)
		}
	}
	if code := do(s, "GET", "/api/spending/subcategories", "", nil, c).Code; code != 400 {
		t.Errorf("sem mês = %d", code)
	}
	if rec := do(s, "GET", "/api/subcategories", "", nil, c); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"code":"TRANSPORT_PARKING"`) {
		t.Errorf("catálogo: %d", rec.Code)
	}
	if code := do(s, "GET", "/api/transactions?essential=talvez", "", nil, c).Code; code != 400 {
		t.Errorf("filtro essencial inválido = %d", code)
	}
	if code := do(s, "GET", "/api/transactions?subcategory=BANANA", "", nil, c).Code; code != 400 {
		t.Errorf("filtro subcategoria inválido = %d", code)
	}
}
