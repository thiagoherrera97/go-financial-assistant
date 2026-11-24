package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

type cardBillJSON struct {
	Due   string               `json:"due"`
	State domain.CardBillState `json:"state"`
	// BankTotal é o total da fatura fechada informado pelo banco; null enquanto ela não fechou.
	BankTotal *float64 `json:"bankTotal"`
	Detailed  float64  `json:"detailed"`
	Forecast  float64  `json:"forecast"`
	Gap       float64  `json:"gap"`
	// Known = Detailed + Forecast (o que o app conhece); Difference = BankTotal − Known (0 dentro da tolerância).
	Known      float64 `json:"known"`
	Difference float64 `json:"difference"`
	Total      float64 `json:"total"`
	Count      int     `json:"count"`
}

type cardWithBillsJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Brand string `json:"brand"`
	Last4 string `json:"last4"`
	// BankBalance é o saldo devedor que o banco informa agora para o cartão.
	BankBalance float64        `json:"bankBalance"`
	Bills       []cardBillJSON `json:"bills"`
}

// creditCards são as contas de cartão de crédito.
func (a *api) creditCards(ctx context.Context) ([]domain.Account, error) {
	accounts, err := a.reader.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	var cards []domain.Account
	for _, acc := range accounts {
		if acc.Type == domain.AccountCredit {
			cards = append(cards, acc)
		}
	}
	return cards, nil
}

// cardBills devolve cada cartão com as suas faturas: o que o app soma ao lado do total que o banco informou.
func (a *api) cardBills(w http.ResponseWriter, r *http.Request) {
	cards, err := a.creditCards(r.Context())
	if err != nil {
		a.fail(w, "faturas", err)
		return
	}
	today := a.localNow()
	out := make([]cardWithBillsJSON, 0, len(cards))
	for _, c := range cards {
		bills, err := a.reader.CardBills(r.Context(), c.ID)
		if err != nil {
			a.fail(w, "faturas", err)
			return
		}
		states := domain.CardBillStates(bills, today)
		js := make([]cardBillJSON, len(bills))
		for i, b := range bills {
			js[i] = cardBillJSON{
				Due: b.Due.Format(time.DateOnly), State: states[i], BankTotal: b.BankTotal,
				Detailed: cents(b.Detailed), Forecast: cents(b.Forecast), Gap: cents(b.Gap),
				Known: b.Known(), Difference: b.Difference(), Total: b.Total(), Count: b.Count,
			}
		}
		out = append(out, cardWithBillsJSON{ID: c.ID.String(), Name: c.Name, Brand: c.Brand, Last4: c.Last4,
			BankBalance: cents(c.Balance), Bills: js})
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": out})
}

type cardBillLineJSON struct {
	ID            uuid.UUID             `json:"id"`
	Date          string                `json:"date"`
	PurchaseDate  *string               `json:"purchaseDate,omitempty"`
	Description   string                `json:"description"`
	Category      domain.Category       `json:"category"`
	CategoryLabel string                `json:"categoryLabel"`
	Amount        float64               `json:"amount"`
	Installment   *int                  `json:"installmentNumber,omitempty"`
	Origin        domain.CardBillOrigin `json:"origin"`
	Pending       bool                  `json:"pending"`
	Note          string                `json:"note"`
}

// cardBillLines devolve as linhas da fatura do cartão {card} que vence no mês de {due} (AAAA-MM-DD).
func (a *api) cardBillLines(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("card"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cartão inválido")
		return
	}
	due, err := time.Parse(time.DateOnly, r.PathValue("due"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "vencimento inválido (use AAAA-MM-DD)")
		return
	}
	cards, err := a.creditCards(r.Context())
	if err != nil {
		a.fail(w, "fatura", err)
		return
	}
	found := false
	for _, c := range cards {
		found = found || c.ID == id
	}
	if !found {
		writeError(w, http.StatusNotFound, "cartão não encontrado")
		return
	}
	lines, err := a.reader.CardBillLines(r.Context(), id, due)
	if err != nil {
		a.fail(w, "fatura", err)
		return
	}
	out := make([]cardBillLineJSON, len(lines))
	for i, l := range lines {
		out[i] = cardBillLineJSON{
			ID: l.ID, Date: l.Date.UTC().Format(time.DateOnly), PurchaseDate: dayString(l.PurchaseDate),
			Description: l.Description, Category: l.Category, CategoryLabel: l.Category.Label(), Amount: l.Amount,
			Installment: l.Installment, Origin: l.Origin, Pending: l.Pending, Note: l.Note,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": out})
}

func (a *api) registerCardBills(rt routes) {
	rt.mux.Handle("GET /api/card-bills", rt.protected(a.cardBills))
	rt.mux.Handle("GET /api/card-bills/{card}/{due}", rt.protected(a.cardBillLines))
}
