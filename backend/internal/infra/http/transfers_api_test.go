package httpserver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func TestAPI_TransfersReviewList(t *testing.T) {
	id, id2 := uuid.New(), uuid.New()
	decidedAt := time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)
	r := &fakeReader{
		pendingTr: []domain.TransferReviewItem{
			{PaymentID: id, Date: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), Amount: 250.5, Kind: "EXPENSE", Account: "Itaú 1234", Description: "Pix enviado Fulano", Category: "OTHER"},
			{PaymentID: id2, Date: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), Amount: 90, Kind: "INCOME", Account: "Itaú 1234", Description: "Pix recebido Beltrano", Category: "OTHER"},
		},
		decidedTr: []domain.TransferReviewItem{
			{PaymentID: uuid.New(), Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Amount: 10, Kind: "EXPENSE", Description: "TED", Category: "PEOPLE", Decision: domain.TransferPerson, DecidedAt: &decidedAt},
		},
	}
	s := newTestAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/transfers/review", "", nil, c)
	var b struct {
		Pending []struct {
			ID, Date, Direction, Account, Description string
			Amount                                    float64
		} `json:"pending"`
		PendingTotal int                                              `json:"pendingTotal"`
		Decided      []struct{ Decision, DecidedAt, Category string } `json:"decided"`
		Categories   []struct{ Value, Label string }                  `json:"categories"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &b) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if len(b.Pending) != 2 || b.PendingTotal != 2 || b.Pending[0].Direction != "out" || b.Pending[1].Direction != "in" || b.Pending[0].Date != "2026-11-03" || b.Pending[0].Amount != 250.5 || b.Pending[0].Account != "Itaú 1234" {
		t.Errorf("pendentes: %+v", b.Pending)
	}
	if len(b.Decided) != 1 || b.Decided[0].Decision != "PERSON" || b.Decided[0].DecidedAt != "2026-11-10T18:00:00Z" {
		t.Errorf("decididas: %+v", b.Decided)
	}
	for _, o := range b.Categories {
		if o.Value == "OTHER" || o.Value == "PEOPLE" {
			t.Errorf("Pagamento não oferece %s", o.Value)
		}
	}
	if len(b.Categories) == 0 {
		t.Error("deveria oferecer categorias de pagamento")
	}
	if rec := do(s, "GET", "/api/transfers/review", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_DecideAndUndoTransfer(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)
	id := uuid.New().String()
	put := func(path, body string) int {
		return do(s, "PUT", "/api/transfers/review/"+path, body, jsonHeader, c).Code
	}

	for body, want := range map[string]string{
		`{"decision":"OWN"}`:                          id + "|OWN|",
		`{"decision":"PERSON"}`:                       id + "|PERSON|",
		`{"decision":"PAYMENT","category":"HOUSING"}`: id + "|PAYMENT|HOUSING",
	} {
		r.decideCalls = nil
		if code := put(id, body); code != 200 || len(r.decideCalls) != 1 || r.decideCalls[0] != want {
			t.Errorf("%s: %d %v", body, code, r.decideCalls)
		}
	}

	r.decideCalls = nil
	for name, body := range map[string]string{
		"decisão inventada":       `{"decision":"DELETE"}`,
		"sem decisão":             `{}`,
		"pagamento sem categoria": `{"decision":"PAYMENT"}`,
		"pagamento em Outros":     `{"decision":"PAYMENT","category":"OTHER"}`,
		"pagamento em Pessoas":    `{"decision":"PAYMENT","category":"PEOPLE"}`,
		"categoria de renda":      `{"decision":"PAYMENT","category":"SALARY"}`,
		"categoria inventada":     `{"decision":"PAYMENT","category":"BANANA"}`,
		"categoria onde não vale": `{"decision":"OWN","category":"FOOD"}`,
		"campo desconhecido":      `{"decision":"OWN","kind":"TRANSFER"}`,
		"corpo inválido":          `nao-json`,
	} {
		if code := put(id, body); code != 400 {
			t.Errorf("%s = %d, esperado 400", name, code)
		}
	}
	if code := put("nao-e-uuid", `{"decision":"OWN"}`); code != 400 {
		t.Errorf("id inválido = %d", code)
	}
	if len(r.decideCalls) != 0 {
		t.Errorf("nada inválido pode chegar ao repositório: %v", r.decideCalls)
	}

	r.decideErr = domain.ErrTransferNotPending
	if code := put(id, `{"decision":"OWN"}`); code != 404 {
		t.Errorf("já decidida/inexistente = %d", code)
	}
	r.decideErr = domain.ErrTransferPaymentOnIncome
	if rec := do(s, "PUT", "/api/transfers/review/"+id, `{"decision":"PAYMENT","category":"FOOD"}`, jsonHeader, c); rec.Code != 400 {
		t.Errorf("pagamento em entrada = %d", rec.Code)
	}
	r.decideErr = nil

	// Desfazer.
	r.decideCalls = nil
	if rec := do(s, "DELETE", "/api/transfers/review/"+id, "", nil, c); rec.Code != 200 || len(r.decideCalls) != 1 || r.decideCalls[0] != "undo|"+id {
		t.Errorf("desfazer: %d %v", rec.Code, r.decideCalls)
	}
	r.decideErr = domain.ErrTransferNoDecision
	if rec := do(s, "DELETE", "/api/transfers/review/"+id, "", nil, c); rec.Code != 404 {
		t.Errorf("sem decisão = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/transfers/review/x", "", nil, c); rec.Code != 400 {
		t.Errorf("id inválido = %d", rec.Code)
	}
}

func TestAPI_TransfersGuards(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	c := login(t, s)
	id := uuid.New().String()
	if code := do(s, "PUT", "/api/transfers/review/"+id, `decision=OWN`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, c).Code; code != 403 {
		t.Errorf("form (CSRF) = %d", code)
	}
	if code := do(s, "PUT", "/api/transfers/review/"+id, `{"decision":"OWN"}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, c).Code; code != 403 {
		t.Errorf("origem diferente = %d", code)
	}
	if code := do(s, "DELETE", "/api/transfers/review/"+id, "", map[string]string{"Origin": "https://evil.example"}, c).Code; code != 403 {
		t.Errorf("DELETE de outra origem = %d", code)
	}
	if code := do(s, "PUT", "/api/transfers/review/"+id, `{"decision":"OWN"}`, jsonHeader).Code; code != 401 {
		t.Errorf("PUT sem sessão = %d", code)
	}
	if code := do(s, "DELETE", "/api/transfers/review/"+id, "", nil).Code; code != 401 {
		t.Errorf("DELETE sem sessão = %d", code)
	}
}
