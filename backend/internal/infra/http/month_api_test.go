package httpserver

import (
	"encoding/json"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func TestAPI_MonthSheet(t *testing.T) {
	r := &fakeReader{monthLines: []domain.MonthLine{
		{Key: "salario", Label: "LIQUIDO", Category: "SALARY", Kind: "INCOME", Amount: 5200},
		{Key: "ifood", Label: "IFOOD", Category: "FOOD", Kind: "EXPENSE", Amount: 80, AccountID: "c1", AccountName: "SANTANDER", Card: true},
	}}
	s := newTestAPI(t, r)
	c := login(t, s)
	rec := do(s, "GET", "/api/month?month=2026-11", "", nil, c)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Month       string  `json:"month"`
		IncomeTotal float64 `json:"incomeTotal"`
		Leftover    float64 `json:"leftover"`
		Cards       []struct {
			Name       string `json:"name"`
			Categories []struct {
				Label string `json:"label"`
			} `json:"categories"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Month != "2026-11" || got.IncomeTotal != 5200 || got.Leftover != 5120 || len(got.Cards) != 1 || got.Cards[0].Categories[0].Label != domain.CategoryFood.Label() {
		t.Errorf("%+v", got)
	}
	if rec := do(s, "GET", "/api/month", "", nil, c); rec.Code != 400 {
		t.Errorf("sem mês deveria ser 400, got %d", rec.Code)
	}
}
