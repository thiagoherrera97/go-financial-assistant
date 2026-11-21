package httpserver

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/debts"
	"github.com/google/uuid"
)

const (
	maxDebtBody   = 2 << 10
	maxDebts      = 20
	maxDebtAmount = 1e8
)

type moneyMovementJSON struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Label  string  `json:"label"`
}

type debtLineJSON struct {
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
	Date   *string `json:"date"`
}

type debtJSON struct {
	ID                string  `json:"id"`
	Kind              string  `json:"kind"`
	Name              string  `json:"name"`
	Creditor          string  `json:"creditor"`
	Note              string  `json:"note"`
	OriginalAmount    float64 `json:"originalAmount"`
	DueDay            int     `json:"dueDay"`
	AccountID         *string `json:"accountId"`
	AccountName       string  `json:"accountName"`
	FreeDays          int     `json:"freeDays"`
	InstallmentAmount float64 `json:"installmentAmount"`
	Installments      int     `json:"installments"`
	FirstDue          *string `json:"firstDue"`
	PayeeMatch        string  `json:"payeeMatch"`
	PaidManual        int     `json:"paidManual"`
	Progress          *struct {
		Paid       int                 `json:"paid"`
		Total      int                 `json:"total"`
		PaidAmount float64             `json:"paidAmount"`
		Remaining  float64             `json:"remaining"`
		NextDue    *string             `json:"nextDue"`
		Overdue    bool                `json:"overdue"`
		Done       bool                `json:"done"`
		Payments   []moneyMovementJSON `json:"payments"`
	} `json:"progress"`
	Limit *struct {
		InLimit  float64 `json:"inLimit"`
		Since    *string `json:"since"`
		DaysUsed int     `json:"daysUsed"`
		Months   []struct {
			Month string  `json:"month"`
			Days  int     `json:"days"`
			Worst float64 `json:"worst"`
		} `json:"months"`
	} `json:"limit"`
	Forecast *struct {
		Date       string         `json:"date"`
		Lines      []debtLineJSON `json:"lines"`
		Balance    float64        `json:"balance"`
		InLimit    float64        `json:"inLimit"`
		ExitDate   *string        `json:"exitDate"`
		ExitLabel  string         `json:"exitLabel"`
		Days       int            `json:"days"`
		WithinFree bool           `json:"withinFree"`
	} `json:"forecast"`
}

func (a *api) debts(w http.ResponseWriter, r *http.Request) {
	now := a.now()
	if a.settings != nil {
		now = now.In(a.settings.Service.Location())
	}
	views, err := a.insights.Debts(r.Context(), now)
	if err != nil {
		a.fail(w, "dívidas", err)
		return
	}
	out := make([]debtJSON, len(views))
	for n, v := range views {
		out[n] = debtToJSON(v.Debt)
		out[n].AccountName = v.AccountName
		if p := v.Installments; p != nil {
			j := &out[n]
			j.Progress = &struct {
				Paid       int                 `json:"paid"`
				Total      int                 `json:"total"`
				PaidAmount float64             `json:"paidAmount"`
				Remaining  float64             `json:"remaining"`
				NextDue    *string             `json:"nextDue"`
				Overdue    bool                `json:"overdue"`
				Done       bool                `json:"done"`
				Payments   []moneyMovementJSON `json:"payments"`
			}{p.Paid, p.Total, p.PaidAmount, p.Remaining, dayString(p.NextDue), p.Overdue, p.Done, movementsJSON(p.Payments)}
		}
		if l := v.Limit; l != nil {
			lj := &struct {
				InLimit  float64 `json:"inLimit"`
				Since    *string `json:"since"`
				DaysUsed int     `json:"daysUsed"`
				Months   []struct {
					Month string  `json:"month"`
					Days  int     `json:"days"`
					Worst float64 `json:"worst"`
				} `json:"months"`
			}{InLimit: l.InLimit, Since: dayString(l.Since), DaysUsed: l.DaysUsed}
			lj.Months = make([]struct {
				Month string  `json:"month"`
				Days  int     `json:"days"`
				Worst float64 `json:"worst"`
			}, len(l.Months))
			for k, m := range l.Months {
				lj.Months[k].Month, lj.Months[k].Days, lj.Months[k].Worst = formatMonth(m.Month), m.Days, m.Worst
			}
			out[n].Limit = lj
		}
		if f := v.Forecast; f != nil {
			out[n].Forecast = &struct {
				Date       string         `json:"date"`
				Lines      []debtLineJSON `json:"lines"`
				Balance    float64        `json:"balance"`
				InLimit    float64        `json:"inLimit"`
				ExitDate   *string        `json:"exitDate"`
				ExitLabel  string         `json:"exitLabel"`
				Days       int            `json:"days"`
				WithinFree bool           `json:"withinFree"`
			}{f.Date.Format("2006-01-02"), linesJSON(f.Lines), cents(f.Balance), cents(f.InLimit), dayString(f.ExitDate), f.ExitLabel, f.Days, f.WithinFree}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"debts": out})
}

func debtToJSON(d domain.Debt) debtJSON {
	j := debtJSON{
		ID: d.ID.String(), Kind: string(d.Kind), Name: d.Name, Creditor: d.Creditor, Note: d.Note, OriginalAmount: d.OriginalAmount,
		DueDay: d.DueDay, FreeDays: d.FreeDays, InstallmentAmount: d.InstallmentAmount, Installments: d.Installments,
		PayeeMatch: d.PayeeMatch, PaidManual: d.PaidManual,
	}
	if d.AccountID != nil {
		s := d.AccountID.String()
		j.AccountID = &s
	}
	if !d.FirstDue.IsZero() {
		j.FirstDue = dayString(&d.FirstDue)
	}
	return j
}

func movementsJSON(list []domain.MoneyMovement) []moneyMovementJSON {
	out := make([]moneyMovementJSON, len(list))
	for n, m := range list {
		out[n] = moneyMovementJSON{m.Date.Format("2006-01-02"), cents(m.Amount), m.Label}
	}
	return out
}

func linesJSON(list []debts.Line) []debtLineJSON {
	out := make([]debtLineJSON, len(list))
	for n, l := range list {
		out[n] = debtLineJSON{l.Label, cents(l.Amount), dayString(l.Date)}
	}
	return out
}

// debtInput é o corpo de POST e PUT /api/debts; cada tipo usa só os seus campos.
type debtInput struct {
	Kind              string  `json:"kind"`
	Name              string  `json:"name"`
	Creditor          string  `json:"creditor"`
	Note              string  `json:"note"`
	OriginalAmount    float64 `json:"originalAmount"`
	DueDay            int     `json:"dueDay"`
	AccountID         string  `json:"accountId"`
	FreeDays          int     `json:"freeDays"`
	InstallmentAmount float64 `json:"installmentAmount"`
	Installments      int     `json:"installments"`
	FirstDue          string  `json:"firstDue"`
	PayeeMatch        string  `json:"payeeMatch"`
	PaidManual        int     `json:"paidManual"`
}

func cleanText(field, s string, minLen, maxLen int) (string, error) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < minLen || n > maxLen || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return "", badRequest(field + " inválido (até " + strconv.Itoa(maxLen) + " caracteres, sem caracteres de controle)")
	}
	return s, nil
}

func money(v float64) float64 { return math.Round(v*100) / 100 }

// debtFromInput valida o corpo e monta a dívida. Erros de validação voltam como apiError (400).
func (a *api) debtFromInput(r *http.Request, in debtInput, id uuid.UUID) (domain.Debt, error) {
	d := domain.Debt{ID: id, Kind: domain.DebtKind(in.Kind)}
	var err error
	if d.Name, err = cleanText("name", in.Name, 1, 60); err != nil {
		return d, err
	}
	if d.Creditor, err = cleanText("creditor", in.Creditor, 0, 60); err != nil {
		return d, err
	}
	if d.Note, err = cleanText("note", in.Note, 0, 120); err != nil {
		return d, err
	}
	if in.DueDay < 1 || in.DueDay > 31 {
		return d, badRequest("dueDay deve ser de 1 a 31")
	}
	d.DueDay = in.DueDay
	switch d.Kind {
	case domain.DebtOverdraft:
		if !(in.OriginalAmount > 0 && in.OriginalAmount <= maxDebtAmount) {
			return d, badRequest("originalAmount deve ser maior que zero")
		}
		if in.FreeDays < 0 || in.FreeDays > 60 {
			return d, badRequest("freeDays deve ser de 0 a 60")
		}
		acc, err := uuid.Parse(in.AccountID)
		if err != nil {
			return d, badRequest("accountId: escolha a conta cujo limite cobre a dívida")
		}
		accounts, err := a.reader.Accounts(r.Context())
		if err != nil {
			return d, err
		}
		found := false
		for _, x := range accounts {
			found = found || x.ID == acc && x.Type == domain.AccountBank
		}
		if !found {
			return d, badRequest("accountId: conta corrente não encontrada")
		}
		d.OriginalAmount, d.FreeDays, d.AccountID = money(in.OriginalAmount), in.FreeDays, &acc
	case domain.DebtInstallments:
		if !(in.InstallmentAmount > 0 && in.InstallmentAmount <= maxDebtAmount) {
			return d, badRequest("installmentAmount deve ser maior que zero")
		}
		if in.Installments < 1 || in.Installments > 120 {
			return d, badRequest("installments deve ser de 1 a 120")
		}
		first, err := time.Parse("2006-01-02", in.FirstDue)
		if err != nil || first.Year() < 2000 || first.Year() > 2100 {
			return d, badRequest("firstDue deve ser uma data (AAAA-MM-DD)")
		}
		if in.PaidManual < 0 || in.PaidManual > in.Installments {
			return d, badRequest("paidManual deve ser de 0 ao número de parcelas")
		}
		if d.PayeeMatch, err = cleanText("payeeMatch", in.PayeeMatch, 0, 60); err != nil {
			return d, err
		}
		d.InstallmentAmount, d.Installments, d.FirstDue, d.PaidManual = money(in.InstallmentAmount), in.Installments, first, in.PaidManual
		d.OriginalAmount = money(d.InstallmentAmount * float64(d.Installments))
		if in.OriginalAmount > 0 && in.OriginalAmount <= maxDebtAmount {
			d.OriginalAmount = money(in.OriginalAmount)
		}
	default:
		return d, badRequest("kind deve ser OVERDRAFT ou INSTALLMENTS")
	}
	return d, nil
}

func (a *api) createDebt(w http.ResponseWriter, r *http.Request) {
	var in debtInput
	if !decodeJSONStrict(w, r, maxDebtBody, &in) {
		return
	}
	d, err := a.debtFromInput(r, in, uuid.New())
	if err != nil {
		a.respondErr(w, "dívida", err)
		return
	}
	existing, err := a.reader.Debts(r.Context())
	if err != nil {
		a.fail(w, "dívida", err)
		return
	}
	if len(existing) >= maxDebts {
		writeError(w, http.StatusConflict, "limite de 20 dívidas atingido")
		return
	}
	if err := a.reader.CreateDebt(r.Context(), d); err != nil {
		a.fail(w, "dívida", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": d.ID.String()})
}

func (a *api) updateDebt(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	var in debtInput
	if !decodeJSONStrict(w, r, maxDebtBody, &in) {
		return
	}
	d, err := a.debtFromInput(r, in, id)
	if err != nil {
		a.respondErr(w, "dívida", err)
		return
	}
	found, err := a.reader.UpdateDebt(r.Context(), d)
	if err != nil {
		a.fail(w, "dívida", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "dívida não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) deleteDebt(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	found, err := a.reader.DeleteDebt(r.Context(), id)
	if err != nil {
		a.fail(w, "dívida", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "dívida não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) registerDebts(rt routes) {
	rt.mux.Handle("GET /api/debts", rt.protected(a.debts))
	rt.mux.Handle("POST /api/debts", rt.protected(jsonOnly(a.createDebt)))
	rt.mux.Handle("PUT /api/debts/{id}", rt.protected(jsonOnly(a.updateDebt)))
	rt.mux.Handle("DELETE /api/debts/{id}", rt.protected(sameOriginOnly(a.deleteDebt)))
}
