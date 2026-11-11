package httpserver

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

const (
	maxTransfersPending = 300
	maxTransfersDecided = 100
)

// transferCategories são as categorias que "Pagamento" aceita: as das contas, sem "Outros" (não é uma decisão) e sem "Pessoas"
// (para isso existe "Para uma pessoa").
var transferCategories = func() []domain.Category {
	var out []domain.Category
	for _, c := range ruleCategories {
		if c != domain.CategoryOther && c != domain.CategoryPeople {
			out = append(out, c)
		}
	}
	return out
}()

var transferDecisions = []domain.TransferDecision{domain.TransferOwn, domain.TransferPerson, domain.TransferPayment}

type transferJSON struct {
	ID          uuid.UUID `json:"id"`
	Date        string    `json:"date"`
	Amount      float64   `json:"amount"`
	Direction   string    `json:"direction"` // "out" (saída) ou "in" (entrada)
	Account     string    `json:"account"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Decision    string    `json:"decision,omitempty"`
	DecidedAt   string    `json:"decidedAt,omitempty"`
}

func toTransferJSON(it domain.TransferReviewItem) transferJSON {
	j := transferJSON{
		ID: it.PaymentID, Date: it.Date.Format("2006-01-02"), Amount: it.Amount, Direction: "out",
		Account: it.Account, Description: it.Description, Category: it.Category, Decision: string(it.Decision),
	}
	if it.Kind == "INCOME" {
		j.Direction = "in"
	}
	if it.DecidedAt != nil {
		j.DecidedAt = it.DecidedAt.UTC().Format(time.RFC3339)
	}
	return j
}

// transfersReview: as transferências do banco que ainda esperam a sua decisão e as já decididas. Nada daqui vai à IA nem ao log.
func (a *api) transfersReview(w http.ResponseWriter, r *http.Request) {
	pending, total, err := a.reader.PendingTransfers(r.Context(), maxTransfersPending)
	if err != nil {
		a.fail(w, "transferências", err)
		return
	}
	decided, err := a.reader.DecidedTransfers(r.Context(), maxTransfersDecided)
	if err != nil {
		a.fail(w, "transferências", err)
		return
	}
	type option struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}
	options := make([]option, len(transferCategories))
	for i, c := range transferCategories {
		options[i] = option{string(c), c.Label()}
	}
	out := struct {
		Pending      []transferJSON `json:"pending"`
		PendingTotal int            `json:"pendingTotal"`
		Decided      []transferJSON `json:"decided"`
		Categories   []option       `json:"categories"`
	}{make([]transferJSON, len(pending)), total, make([]transferJSON, len(decided)), options}
	for i, it := range pending {
		out.Pending[i] = toTransferJSON(it)
	}
	for i, it := range decided {
		out.Decided[i] = toTransferJSON(it)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) decideTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("paymentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	var body struct {
		Decision string `json:"decision"`
		Category string `json:"category"`
	}
	if !decodeJSONStrict(w, r, maxRuleBody, &body) {
		return
	}
	decision := domain.TransferDecision(body.Decision)
	if !slices.Contains(transferDecisions, decision) {
		writeError(w, http.StatusBadRequest, "decision inválida")
		return
	}
	switch {
	case decision == domain.TransferPayment && !slices.Contains(transferCategories, domain.Category(body.Category)):
		writeError(w, http.StatusBadRequest, "category inválida")
		return
	case decision != domain.TransferPayment && body.Category != "":
		writeError(w, http.StatusBadRequest, "category só vale em PAYMENT")
		return
	}
	switch err := a.reader.DecideTransfer(r.Context(), id, decision, body.Category); {
	case errors.Is(err, domain.ErrTransferNotPending):
		writeError(w, http.StatusNotFound, "transferência não encontrada ou já decidida")
	case errors.Is(err, domain.ErrTransferPaymentOnIncome):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		a.fail(w, "transferências", err)
	default:
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func (a *api) undoTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("paymentId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	switch err := a.reader.UndoTransferDecision(r.Context(), id); {
	case errors.Is(err, domain.ErrTransferNoDecision):
		writeError(w, http.StatusNotFound, "decisão não encontrada")
	case err != nil:
		a.fail(w, "transferências", err)
	default:
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func (a *api) registerTransfers(rt routes) {
	rt.mux.Handle("GET /api/transfers/review", rt.protected(a.transfersReview))
	// Uma decisão por clique: 60 por minuto cobre revisar uma lista longa e barra um laço descontrolado.
	writeLimiter := newIPRateLimiter(60, time.Minute)
	rt.mux.Handle("PUT /api/transfers/review/{paymentId}", writeLimiter.middleware(rt.protected(jsonOnly(a.decideTransfer))))
	rt.mux.Handle("DELETE /api/transfers/review/{paymentId}", writeLimiter.middleware(rt.protected(sameOriginOnly(a.undoTransfer))))
}
