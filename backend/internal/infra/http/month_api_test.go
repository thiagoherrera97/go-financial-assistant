package httpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
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

func TestAPI_SetTransactionNote(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)
	id := uuid.New()
	rec := do(s, "PUT", "/api/transactions/"+id.String()+"/note", `{"note":"  insulfilm,\n mãe ajudando "}`, jsonHdr, c)
	if rec.Code != 200 || r.notes[id] != "insulfilm, mãe ajudando" {
		t.Fatalf("%d %s %q", rec.Code, rec.Body.String(), r.notes[id])
	}
	long := strings.Repeat("a", 301)
	if rec := do(s, "PUT", "/api/transactions/"+id.String()+"/note", `{"note":"`+long+`"}`, jsonHdr, c); rec.Code != 400 {
		t.Errorf("nota longa: %d", rec.Code)
	}
	if rec := do(s, "PUT", "/api/transactions/"+uuid.Nil.String()+"/note", `{"note":"x"}`, jsonHdr, c); rec.Code != 404 {
		t.Errorf("inexistente: %d", rec.Code)
	}
	if rec := do(s, "PUT", "/api/transactions/"+id.String()+"/note", `{"note":"x"}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, c); rec.Code != 403 {
		t.Errorf("outra origem: %d", rec.Code)
	}
}

func TestAPI_DeleteManualTransaction(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)
	id := uuid.New()
	if rec := do(s, "DELETE", "/api/transactions/"+id.String(), "", nil, c); rec.Code != 204 || len(r.deletedTx) != 1 || r.deletedTx[0] != id {
		t.Fatalf("%d %v", rec.Code, r.deletedTx)
	}
	if rec := do(s, "DELETE", "/api/transactions/"+uuid.Nil.String(), "", nil, c); rec.Code != 404 {
		t.Errorf("não apagável: %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/transactions/"+id.String(), "", map[string]string{"Origin": "https://evil.example"}, c); rec.Code != 403 {
		t.Errorf("outra origem: %d", rec.Code)
	}
}
