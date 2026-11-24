package httpserver

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

type salaryReviewJSON struct {
	ID      string  `json:"id"`
	Date    string  `json:"date"`
	Amount  float64 `json:"amount"`
	Label   string  `json:"label"`
	Suggest string  `json:"suggest"`       // palpite: MONTH, PART ou vazio (extra: o app não sabe dizer qual)
	Month   string  `json:"month"`         // AAAA-MM: o ciclo do palpite
	Day     int     `json:"day,omitempty"` // PART: a parte do palpite
}

type salaryDecidedJSON struct {
	ID        string  `json:"id"`
	Date      string  `json:"date"`
	Amount    float64 `json:"amount"`
	Label     string  `json:"label"`
	Kind      string  `json:"kind"`
	KindLabel string  `json:"kindLabel"`
	Month     string  `json:"month,omitempty"`
	Day       int     `json:"day,omitempty"`
}

// salaryReview: os créditos de salário que o app não reconheceu com certeza (valor fora do combinado ou extra) e os já
// decididos, dos últimos ciclos, com as partes e as respostas possíveis.
func (a *api) salaryReview(w http.ResponseWriter, r *http.Request) {
	view, err := a.insights.SalaryReviews(r.Context(), a.localNow())
	if err != nil {
		a.fail(w, "conferência do salário", err)
		return
	}
	type partJSON struct {
		Day    int     `json:"day"`
		Amount float64 `json:"amount"`
	}
	type kindJSON struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}
	out := struct {
		Pending []salaryReviewJSON  `json:"pending"`
		Decided []salaryDecidedJSON `json:"decided"`
		Parts   []partJSON          `json:"parts"`
		Kinds   []kindJSON          `json:"kinds"`
	}{Pending: []salaryReviewJSON{}, Decided: []salaryDecidedJSON{}, Parts: []partJSON{}}
	for _, p := range view.Parts {
		out.Parts = append(out.Parts, partJSON{p.Day, p.Amount})
	}
	for _, k := range domain.SalaryKinds {
		out.Kinds = append(out.Kinds, kindJSON{string(k), k.Label()})
	}
	for _, it := range view.Pending {
		out.Pending = append(out.Pending, salaryReviewJSON{
			ID: it.Credit.ID, Date: it.Credit.Date.Format("2006-01-02"), Amount: it.Credit.Amount, Label: it.Credit.Label,
			Suggest: string(it.Suggest), Month: formatMonth(it.Month), Day: it.Day,
		})
	}
	for _, c := range view.Decided {
		d := salaryDecidedJSON{
			ID: c.ID, Date: c.Date.Format("2006-01-02"), Amount: c.Amount, Label: c.Label,
			Kind: string(c.Kind), KindLabel: c.Kind.Label(), Day: c.Day,
		}
		if !c.Month.IsZero() {
			d.Month = formatMonth(c.Month)
		}
		out.Decided = append(out.Decided, d)
	}
	writeJSON(w, http.StatusOK, out)
}

// decideSalary grava o que a pessoa disse que o crédito é e recalcula os ciclos (o mês pode começar em outro dia).
func (a *api) decideSalary(w http.ResponseWriter, r *http.Request) {
	store, ok := a.reader.(ports.SalaryDecisionStore)
	id, err := uuid.Parse(r.PathValue("paymentId"))
	if !ok || err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	var body struct {
		Kind  string `json:"kind"`
		Month string `json:"month"`
		Day   int    `json:"day"`
	}
	if !decodeJSONStrict(w, r, maxRuleBody, &body) {
		return
	}
	kind := domain.SalaryKind(body.Kind)
	if !kind.Valid() {
		writeError(w, http.StatusBadRequest, "kind inválido")
		return
	}
	if kind != domain.SalaryKindPart {
		body.Day = 0 // o dia só vale para uma parte
	}
	var month *time.Time
	if kind == domain.SalaryKindMonth || kind == domain.SalaryKindPart {
		m, err := time.Parse("2006-01", body.Month)
		if err != nil {
			writeError(w, http.StatusBadRequest, "month inválido: use AAAA-MM")
			return
		}
		month = &m
	}
	if kind == domain.SalaryKindPart && !slices.ContainsFunc(a.insightsSalaryParts(), func(p domain.SalaryPart) bool { return p.Day == body.Day }) {
		writeError(w, http.StatusBadRequest, "day não é o dia de uma parte do salário")
		return
	}
	switch err := store.DecideSalary(r.Context(), id, kind, month, body.Day); {
	case errors.Is(err, domain.ErrSalaryCreditNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		a.fail(w, "conferência do salário", err)
	default:
		a.afterSalaryDecision(r)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *api) undoSalary(w http.ResponseWriter, r *http.Request) {
	store, ok := a.reader.(ports.SalaryDecisionStore)
	id, err := uuid.Parse(r.PathValue("paymentId"))
	if !ok || err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if err := store.UndoSalaryDecision(r.Context(), id); err != nil {
		a.fail(w, "conferência do salário", err)
		return
	}
	a.afterSalaryDecision(r)
	w.WriteHeader(http.StatusNoContent)
}

// afterSalaryDecision recalcula os ciclos; uma falha aqui só atrasa o novo início do mês até a próxima sincronização.
func (a *api) afterSalaryDecision(r *http.Request) {
	if err := a.refreshCycles(r.Context()); err != nil {
		a.logger.Error("erro ao recalcular os ciclos depois da conferência do salário", "error", err)
	}
}

// insightsSalaryParts são as partes do salário configuradas (vazio sem configuração).
func (a *api) insightsSalaryParts() []domain.SalaryPart {
	if a.settings == nil {
		return nil
	}
	return a.settings.Service.SalaryParts()
}

func (a *api) registerSalaryReview(rt routes) {
	rt.mux.Handle("GET /api/salary/review", rt.protected(a.salaryReview))
	writeLimiter := newIPRateLimiter(60, time.Minute)
	rt.mux.Handle("PUT /api/salary/review/{paymentId}", writeLimiter.middleware(rt.protected(jsonOnly(a.decideSalary))))
	rt.mux.Handle("DELETE /api/salary/review/{paymentId}", writeLimiter.middleware(rt.protected(sameOriginOnly(a.undoSalary))))
}
