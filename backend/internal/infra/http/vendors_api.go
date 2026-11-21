package httpserver

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

const (
	maxVendorBody     = 2 << 10
	maxVendors        = 50
	maxVendorPayments = 200
	maxVendorNameLen  = 60
	maxVendorAmount   = 1e9
	maxVendorYears    = 10
	maxBoxNameLen     = 120
)

// vendors: o plano de fornecedores (ex.: casamento) com os totais calculados agora, e as caixinhas que dá para
// ligar a ele.
func (a *api) vendors(w http.ResponseWriter, r *http.Request) {
	s, boxes, err := a.insights.Vendors(r.Context(), a.reader, a.now())
	if err != nil {
		a.fail(w, "fornecedores", err)
		return
	}

	type payment struct {
		ID            uuid.UUID  `json:"id"`
		Amount        float64    `json:"amount"`
		PaidOn        string     `json:"paidOn"`
		TransactionID *uuid.UUID `json:"transactionId"`
		Description   string     `json:"description"`
	}
	type vendor struct {
		ID            uuid.UUID `json:"id"`
		Name          string    `json:"name"`
		Contracted    float64   `json:"contracted"`
		DueDate       *string   `json:"dueDate"`
		Paid          float64   `json:"paid"`
		Remaining     float64   `json:"remaining"`
		Done          bool      `json:"done"`
		Contributions int       `json:"contributions"`
		Late          bool      `json:"late"`
		Payments      []payment `json:"payments"`
	}
	out := make([]vendor, len(s.Vendors))
	for i, vp := range s.Vendors {
		v := vp.Vendor
		o := vendor{ID: v.ID, Name: v.Name, Contracted: v.Contracted, DueDate: dateOrNil(v.DueDate), Paid: vp.Paid,
			Remaining: vp.Remaining, Done: vp.Done, Contributions: vp.Contributions, Late: vp.Late, Payments: []payment{}}
		for _, p := range v.Payments {
			o.Payments = append(o.Payments, payment{p.ID, p.Amount, p.PaidOn.Format(time.DateOnly), p.TransactionID, p.Description})
		}
		out[i] = o
	}
	var tight *string
	if s.Tight != nil {
		tight = &s.Tight.Name
	}
	if boxes == nil {
		boxes = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plan": map[string]any{
			"name": s.Plan.Name, "contributionDay": s.Plan.ContributionDay, "savingsBox": s.Plan.SavingsBox, "savedManual": s.Plan.SavedManual,
		},
		"boxes":            boxes,
		"contracted":       s.Contracted,
		"paid":             s.Paid,
		"saved":            s.Saved,
		"savedFromBox":     s.SavedFromBox,
		"remaining":        s.Remaining,
		"nextContribution": s.NextContribution.Format(time.DateOnly),
		"lastDue":          dateOrNil(s.LastDue),
		"monthsLeft":       s.MonthsLeft,
		"perMonth":         s.PerMonth,
		"flatPerMonth":     s.FlatPerMonth,
		"tight":            tight,
		"surplus":          s.Surplus,
		"fits":             s.Fits,
		"vendors":          out,
	})
}

func dateOrNil(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

func vendorName(raw, field string) (string, error) {
	name := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxVendorNameLen || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", badRequest(field + " deve ter de 1 a 60 caracteres, sem caracteres de controle")
	}
	return name, nil
}

func validAmount(v float64, allowZero bool) bool {
	return (v > 0 || (allowZero && v == 0)) && v <= maxVendorAmount
}

// vendorDate lê AAAA-MM-DD dentro de 10 anos para trás ou para a frente; vazio = nil quando optional.
func (a *api) vendorDate(raw, field string, optional bool) (*time.Time, error) {
	if raw == "" && optional {
		return nil, nil
	}
	d, err := time.Parse(time.DateOnly, raw)
	now := a.now()
	if err != nil || d.Before(now.AddDate(-maxVendorYears, 0, 0)) || d.After(now.AddDate(maxVendorYears, 0, 0)) {
		return nil, badRequest(field + " deve ser uma data (AAAA-MM-DD) em até 10 anos")
	}
	return &d, nil
}

type planInput struct {
	Name            string  `json:"name"`
	ContributionDay int     `json:"contributionDay"`
	SavingsBox      string  `json:"savingsBox"`
	SavedManual     float64 `json:"savedManual"`
}

func (a *api) saveVendorPlan(w http.ResponseWriter, r *http.Request) {
	var in planInput
	if !decodeJSONStrict(w, r, maxVendorBody, &in) {
		return
	}
	name, err := vendorName(in.Name, "name")
	if err != nil {
		a.respondErr(w, "plano", err)
		return
	}
	box := strings.TrimSpace(in.SavingsBox)
	switch {
	case in.ContributionDay < 1 || in.ContributionDay > 28:
		writeError(w, http.StatusBadRequest, "contributionDay deve ser de 1 a 28")
		return
	case utf8.RuneCountInString(box) > maxBoxNameLen:
		writeError(w, http.StatusBadRequest, "savingsBox muito longo")
		return
	case !validAmount(in.SavedManual, true):
		writeError(w, http.StatusBadRequest, "savedManual deve ser zero ou positivo")
		return
	}
	p := domain.VendorPlan{Name: name, ContributionDay: in.ContributionDay, SavingsBox: box, SavedManual: cents(in.SavedManual)}
	if err := a.reader.SaveVendorPlan(r.Context(), p); err != nil {
		a.fail(w, "plano", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type vendorInput struct {
	Name       string  `json:"name"`
	Contracted float64 `json:"contracted"`
	DueDate    string  `json:"dueDate"`
	// Paid é o que já foi pago antes de usar o app; vira um pagamento à mão com a data de hoje. Só na criação.
	Paid float64 `json:"paid"`
}

func (a *api) vendorFromInput(in vendorInput) (domain.Vendor, error) {
	name, err := vendorName(in.Name, "name")
	if err != nil {
		return domain.Vendor{}, err
	}
	if !validAmount(in.Contracted, false) {
		return domain.Vendor{}, badRequest("contracted deve ser maior que zero")
	}
	due, err := a.vendorDate(in.DueDate, "dueDate", true)
	if err != nil {
		return domain.Vendor{}, err
	}
	return domain.Vendor{Name: name, Contracted: cents(in.Contracted), DueDate: due}, nil
}

func (a *api) createVendor(w http.ResponseWriter, r *http.Request) {
	var in vendorInput
	if !decodeJSONStrict(w, r, maxVendorBody, &in) {
		return
	}
	v, err := a.vendorFromInput(in)
	if err != nil {
		a.respondErr(w, "fornecedor", err)
		return
	}
	if !validAmount(in.Paid, true) {
		writeError(w, http.StatusBadRequest, "paid deve ser zero ou positivo")
		return
	}
	existing, err := a.reader.Vendors(r.Context())
	if err != nil {
		a.fail(w, "fornecedor", err)
		return
	}
	if len(existing) >= maxVendors {
		writeError(w, http.StatusConflict, "limite de 50 fornecedores atingido")
		return
	}
	v.ID = uuid.New()
	var initial *domain.VendorPayment
	if in.Paid > 0 {
		now := a.now()
		initial = &domain.VendorPayment{ID: uuid.New(), VendorID: v.ID, Amount: cents(in.Paid),
			PaidOn: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)}
	}
	if err := a.reader.CreateVendor(r.Context(), v, initial); err != nil {
		a.fail(w, "fornecedor", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": v.ID.String()})
}

func (a *api) updateVendor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "fornecedor não encontrado")
	if !ok {
		return
	}
	var in vendorInput
	if !decodeJSONStrict(w, r, maxVendorBody, &in) {
		return
	}
	v, err := a.vendorFromInput(in)
	if err != nil {
		a.respondErr(w, "fornecedor", err)
		return
	}
	v.ID = id
	found, err := a.reader.UpdateVendor(r.Context(), v)
	if err != nil {
		a.fail(w, "fornecedor", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "fornecedor não encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) deleteVendor(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "fornecedor não encontrado")
	if !ok {
		return
	}
	found, err := a.reader.DeleteVendor(r.Context(), id)
	if err != nil {
		a.fail(w, "fornecedor", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "fornecedor não encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// addVendorPayment lança um pagamento: à mão (amount + paidOn) ou ligando uma transação do extrato (transactionId),
// de onde vêm valor e data.
func (a *api) addVendorPayment(w http.ResponseWriter, r *http.Request) {
	vendorID, ok := pathID(w, r, "id", "fornecedor não encontrado")
	if !ok {
		return
	}
	var in struct {
		Amount        float64 `json:"amount"`
		PaidOn        string  `json:"paidOn"`
		TransactionID string  `json:"transactionId"`
	}
	if !decodeJSONStrict(w, r, maxVendorBody, &in) {
		return
	}
	p := domain.VendorPayment{ID: uuid.New(), VendorID: vendorID}
	if in.TransactionID != "" {
		txID, err := uuid.Parse(in.TransactionID)
		if err != nil || in.Amount != 0 || in.PaidOn != "" {
			writeError(w, http.StatusBadRequest, "informe transactionId sozinho, ou amount e paidOn")
			return
		}
		p.TransactionID = &txID
	} else {
		if !validAmount(in.Amount, false) {
			writeError(w, http.StatusBadRequest, "amount deve ser maior que zero")
			return
		}
		paidOn, err := a.vendorDate(in.PaidOn, "paidOn", false)
		if err != nil {
			a.respondErr(w, "pagamento", err)
			return
		}
		p.Amount, p.PaidOn = cents(in.Amount), *paidOn
	}

	vendors, err := a.reader.Vendors(r.Context())
	if err != nil {
		a.fail(w, "pagamento", err)
		return
	}
	for _, v := range vendors {
		if v.ID == vendorID && len(v.Payments) >= maxVendorPayments {
			writeError(w, http.StatusConflict, "limite de 200 pagamentos por fornecedor atingido")
			return
		}
	}

	switch err := a.reader.AddVendorPayment(r.Context(), p); {
	case errors.Is(err, domain.ErrVendorNotFound), errors.Is(err, domain.ErrVendorTxNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrVendorTxLinked):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil:
		a.fail(w, "pagamento", err)
	default:
		writeJSON(w, http.StatusCreated, map[string]string{"id": p.ID.String()})
	}
}

func (a *api) deleteVendorPayment(w http.ResponseWriter, r *http.Request) {
	vendorID, ok := pathID(w, r, "id", "pagamento não encontrado")
	if !ok {
		return
	}
	paymentID, ok := pathID(w, r, "paymentId", "pagamento não encontrado")
	if !ok {
		return
	}
	found, err := a.reader.DeleteVendorPayment(r.Context(), vendorID, paymentID)
	if err != nil {
		a.fail(w, "pagamento", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "pagamento não encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func pathID(w http.ResponseWriter, r *http.Request, name, notFound string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		writeError(w, http.StatusNotFound, notFound)
		return uuid.UUID{}, false
	}
	return id, true
}

func (a *api) registerVendors(rt routes) {
	rt.mux.Handle("GET /api/vendors", rt.protected(a.vendors))
	rt.mux.Handle("PUT /api/vendors/plan", rt.protected(jsonOnly(a.saveVendorPlan)))
	rt.mux.Handle("POST /api/vendors", rt.protected(jsonOnly(a.createVendor)))
	rt.mux.Handle("PUT /api/vendors/{id}", rt.protected(jsonOnly(a.updateVendor)))
	rt.mux.Handle("DELETE /api/vendors/{id}", rt.protected(sameOriginOnly(a.deleteVendor)))
	rt.mux.Handle("POST /api/vendors/{id}/payments", rt.protected(jsonOnly(a.addVendorPayment)))
	rt.mux.Handle("DELETE /api/vendors/{id}/payments/{paymentId}", rt.protected(sameOriginOnly(a.deleteVendorPayment)))
}
