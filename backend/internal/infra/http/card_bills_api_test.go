package httpserver

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func TestAPI_CardBills(t *testing.T) {
	card, bank := uuid.New(), uuid.New()
	total := 1000.0
	inst := 3
	f := &fakeReader{accounts: []domain.Account{
		{ID: bank, Type: domain.AccountBank, Name: "Conta"},
		{ID: card, Type: domain.AccountCredit, Name: "Nubank", Last4: "1234", Balance: 1500.456},
	}}
	f.debt.cardBills = []domain.CardBillSummary{
		{Due: time.Date(2020, 1, 10, 0, 0, 0, 0, time.UTC), BankTotal: &total, Detailed: 900, Forecast: 50, Gap: 50, Count: 7},
	}
	f.debt.billLines = []domain.CardBillLine{{ID: uuid.New(), Date: time.Date(2019, 12, 20, 0, 0, 0, 0, time.UTC),
		Description: "Mercado (3/5)", Category: domain.CategoryFood, Amount: 30, Installment: &inst, Origin: domain.CardBillOriginBank}}
	s := newTestAPI(t, f)
	c := login(t, s)

	rec := do(s, "GET", "/api/card-bills", "", nil, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/card-bills = %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		Cards []cardWithBillsJSON `json:"cards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Cards) != 1 || got.Cards[0].ID != card.String() || got.Cards[0].BankBalance != 1500.46 {
		t.Fatalf("só o cartão deveria vir, com o saldo em centavos: %+v", got.Cards)
	}
	b := got.Cards[0].Bills[0]
	if b.State != domain.CardBillClosed || b.Known != 950 || b.Difference != 50 || b.Total != 1000 || b.Due != "2020-01-10" {
		t.Errorf("fatura: %+v", b)
	}

	rec = do(s, "GET", "/api/card-bills/"+card.String()+"/2020-01-10", "", nil, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("linhas = %d: %s", rec.Code, rec.Body)
	}
	var lines struct {
		Lines []cardBillLineJSON `json:"lines"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lines); err != nil {
		t.Fatal(err)
	}
	if len(lines.Lines) != 1 || lines.Lines[0].CategoryLabel == "" || lines.Lines[0].Origin != domain.CardBillOriginBank {
		t.Errorf("linhas: %+v", lines.Lines)
	}
	if f.debt.billAsked.Format(time.DateOnly) != "2020-01-10" {
		t.Errorf("vencimento pedido: %v", f.debt.billAsked)
	}

	for path, want := range map[string]int{
		"/api/card-bills/" + bank.String() + "/2020-01-10": http.StatusNotFound,
		"/api/card-bills/nao-uuid/2020-01-10":              http.StatusBadRequest,
		"/api/card-bills/" + card.String() + "/2020-1-10":  http.StatusBadRequest,
	} {
		if rec := do(s, "GET", path, "", nil, c); rec.Code != want {
			t.Errorf("GET %s = %d, esperava %d", path, rec.Code, want)
		}
	}
	if rec := do(s, "GET", "/api/card-bills", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}
