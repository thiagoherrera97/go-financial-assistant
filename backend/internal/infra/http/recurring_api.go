package httpserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/insights"
	"github.com/google/uuid"
)

const maxFlows = 40

type flowJSON struct {
	ID            string              `json:"id"`
	In            bool                `json:"in"`
	Name          string              `json:"name"`
	Party         string              `json:"party"`
	Amount        float64             `json:"amount"`
	Step          float64             `json:"step"`
	MaxAmount     float64             `json:"maxAmount"`
	Day           int                 `json:"day"`
	StartMonth    string              `json:"startMonth"`
	Installments  int                 `json:"installments"`
	PayeeMatch    string              `json:"payeeMatch"`
	Done          int                 `json:"done"`
	ThisMonth     float64             `json:"thisMonth"`
	Active        bool                `json:"active"`
	ThisMonthDone bool                `json:"thisMonthDone"`
	NextDate      *string             `json:"nextDate"`
	NextAmount    float64             `json:"nextAmount"`
	Remaining     float64             `json:"remaining"`
	Ended         bool                `json:"ended"`
	Matches       []moneyMovementJSON `json:"matches"`
}

func flowsJSON(list []insights.FlowItem) []flowJSON {
	out := make([]flowJSON, len(list))
	for n, it := range list {
		f, v := it.Flow, it.View
		out[n] = flowJSON{
			ID: f.ID.String(), In: f.In, Name: f.Name, Party: f.Party, Amount: f.Amount, Step: f.Step, MaxAmount: f.MaxAmount, Day: f.Day,
			StartMonth: formatMonth(f.StartMonth), Installments: f.Installments, PayeeMatch: f.PayeeMatch,
			Done: v.Done, ThisMonth: v.ThisMonth, Active: v.Active, ThisMonthDone: v.ThisMonthDone, NextDate: dayString(v.NextDate),
			NextAmount: v.NextAmount, Remaining: v.Remaining, Ended: v.Ended, Matches: movementsJSON(v.Matches),
		}
	}
	return out
}

// flowInput é o corpo de POST e PUT /api/recurring.
type flowInput struct {
	In           bool    `json:"in"`
	Name         string  `json:"name"`
	Party        string  `json:"party"`
	Amount       float64 `json:"amount"`
	Step         float64 `json:"step"`
	MaxAmount    float64 `json:"maxAmount"`
	Day          int     `json:"day"`
	StartMonth   string  `json:"startMonth"`
	Installments int     `json:"installments"`
	PayeeMatch   string  `json:"payeeMatch"`
}

func flowFromInput(in flowInput, id uuid.UUID) (domain.RecurringFlow, error) {
	f := domain.RecurringFlow{ID: id, In: in.In}
	var err error
	if f.Name, err = cleanText("name", in.Name, 1, 60); err != nil {
		return f, err
	}
	if f.Party, err = cleanText("party", in.Party, 0, 60); err != nil {
		return f, err
	}
	if f.PayeeMatch, err = cleanText("payeeMatch", strings.ReplaceAll(in.PayeeMatch, "\n", ";"), 0, 200); err != nil {
		return f, err
	}
	if !(in.Amount > 0 && in.Amount <= maxDebtAmount) || !(in.Step >= 0 && in.Step <= maxDebtAmount) || !(in.MaxAmount >= 0 && in.MaxAmount <= maxDebtAmount) {
		return f, badRequest("amount deve ser maior que zero; step e maxAmount, zero ou mais")
	}
	if in.MaxAmount > 0 && in.MaxAmount < in.Amount {
		return f, badRequest("maxAmount não pode ser menor que amount")
	}
	if in.Day < 1 || in.Day > 31 {
		return f, badRequest("day deve ser de 1 a 31")
	}
	if in.Installments < 0 || in.Installments > 120 {
		return f, badRequest("installments deve ser de 0 (sem fim) a 120")
	}
	start, err := time.Parse("2006-01", in.StartMonth)
	if err != nil || start.Year() < 2000 || start.Year() > 2100 {
		return f, badRequest("startMonth deve ser um mês (AAAA-MM)")
	}
	f.Amount, f.Step, f.MaxAmount, f.Day, f.StartMonth, f.Installments = money(in.Amount), money(in.Step), money(in.MaxAmount), in.Day, start, in.Installments
	return f, nil
}

func (a *api) createFlow(w http.ResponseWriter, r *http.Request) {
	var in flowInput
	if !decodeJSONStrict(w, r, maxDebtBody, &in) {
		return
	}
	f, err := flowFromInput(in, uuid.New())
	if err != nil {
		a.respondErr(w, "recorrência", err)
		return
	}
	existing, err := a.reader.RecurringFlows(r.Context())
	if err != nil {
		a.fail(w, "recorrência", err)
		return
	}
	if len(existing) >= maxFlows {
		writeError(w, http.StatusConflict, "limite de 40 recorrências atingido")
		return
	}
	if err := a.reader.CreateRecurringFlow(r.Context(), f); err != nil {
		a.fail(w, "recorrência", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": f.ID.String()})
}

func (a *api) updateFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	var in flowInput
	if !decodeJSONStrict(w, r, maxDebtBody, &in) {
		return
	}
	f, err := flowFromInput(in, id)
	if err != nil {
		a.respondErr(w, "recorrência", err)
		return
	}
	found, err := a.reader.UpdateRecurringFlow(r.Context(), f)
	if err != nil {
		a.fail(w, "recorrência", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "recorrência não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) deleteFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	found, err := a.reader.DeleteRecurringFlow(r.Context(), id)
	if err != nil {
		a.fail(w, "recorrência", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "recorrência não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
