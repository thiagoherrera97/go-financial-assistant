package httpserver

import (
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// filterParams lê os filtros comuns da lista e dos grupos de transações, validando cada um.
func (a *api) filterParams(w http.ResponseWriter, r *http.Request) (domain.TransactionFilter, bool) {
	q := r.URL.Query()
	f := domain.TransactionFilter{Limit: defaultPageSize, Search: q.Get("q")}

	month, ok := a.monthParam(w, r, "month", false)
	if !ok {
		return f, false
	}
	if !month.IsZero() {
		f.Month = &month
	}
	if raw := q.Get("day"); raw != "" {
		day, err := time.Parse(time.DateOnly, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "day deve estar no formato AAAA-MM-DD")
			return f, false
		}
		f.Day = &day
	}
	if f.Kind, ok = enumParam(w, q.Get("kind"), "kind", kinds); !ok {
		return f, false
	}
	if f.ExcludeKind, ok = enumParam(w, q.Get("exclude_kind"), "exclude_kind", kinds); !ok {
		return f, false
	}
	if f.Category, ok = enumParam(w, q.Get("category"), "category", categories); !ok {
		return f, false
	}
	if raw := q.Get("subcategory"); raw != "" && raw != "none" {
		if _, known := domain.Subcategory(raw).Info(); !known {
			writeError(w, http.StatusBadRequest, "subcategory inválido")
			return f, false
		}
	}
	f.Subcategory = q.Get("subcategory")
	if f.Essential, ok = enumParam(w, q.Get("essential"), "essential", essentialFilters); !ok {
		return f, false
	}
	if f.PaymentMethod, ok = enumParam(w, q.Get("payment_method"), "payment_method", paymentMethods); !ok {
		return f, false
	}
	if raw := q.Get("account"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "account inválido")
			return f, false
		}
		f.AccountID = &id
	}
	if utf8.RuneCountInString(f.Search) > maxSearchLen {
		writeError(w, http.StatusBadRequest, "q muito longo")
		return f, false
	}
	return f, true
}

// transactionGroups: totais da lista de transações por categoria ou por dia, sob os mesmos filtros.
func (a *api) transactionGroups(w http.ResponseWriter, r *http.Request) {
	f, ok := a.filterParams(w, r)
	if !ok {
		return
	}
	by := domain.GroupBy(r.URL.Query().Get("by"))
	if by != domain.GroupByCategory && by != domain.GroupByDay {
		writeError(w, http.StatusBadRequest, "by deve ser category ou day")
		return
	}
	groups, err := a.reader.TransactionGroups(r.Context(), f, by)
	if err != nil {
		a.fail(w, "grupos de transações", err)
		return
	}
	type item struct {
		Key      string  `json:"key"`
		Label    string  `json:"label"`
		Count    int     `json:"count"`
		Expense  float64 `json:"expense"`
		Income   float64 `json:"income"`
		Transfer float64 `json:"transfer"`
		Applied  float64 `json:"applied"`
		Redeemed float64 `json:"redeemed"`
	}
	out := make([]item, len(groups))
	for i, g := range groups {
		label := g.Key
		switch {
		case by == domain.GroupByCategory && g.Key == domain.IncomeGroupKey:
			label = "Entradas"
		case by == domain.GroupByCategory:
			label = domain.Category(g.Key).Label()
		}
		out[i] = item{g.Key, label, g.Count, g.Expense, g.Income, g.Transfer, g.Applied, g.Redeemed}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) transactions(w http.ResponseWriter, r *http.Request) {
	f, ok := a.filterParams(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page := 1
	if raw := q.Get("page"); raw != "" {
		var err error
		if page, err = strconv.Atoi(raw); err != nil || page < 1 || page > 10000 {
			writeError(w, http.StatusBadRequest, "page inválido")
			return
		}
	}
	if raw := q.Get("limit"); raw != "" {
		var err error
		if f.Limit, err = strconv.Atoi(raw); err != nil || f.Limit < 1 || f.Limit > maxPageSize {
			writeError(w, http.StatusBadRequest, "limit deve estar entre 1 e 100")
			return
		}
	}
	f.Offset = (page - 1) * f.Limit
	switch q.Get("sort") {
	case "", "date":
	case "amount":
		f.SortByAmount = true
	default:
		writeError(w, http.StatusBadRequest, "sort deve ser date ou amount")
		return
	}

	txs, total, err := a.reader.Transactions(r.Context(), f)
	if err != nil {
		a.fail(w, "transações", err)
		return
	}
	type item struct {
		ID                uuid.UUID  `json:"id"`
		Date              string     `json:"date"`
		Description       string     `json:"description"`
		Category          string     `json:"category"`
		CategoryLabel     string     `json:"categoryLabel"`
		Subcategory       string     `json:"subcategory,omitempty"`
		SubcategoryLabel  string     `json:"subcategoryLabel,omitempty"`
		Essential         *bool      `json:"essential"`
		PaymentMethod     string     `json:"paymentMethod"`
		PaymentLabel      string     `json:"paymentMethodLabel"`
		Kind              string     `json:"kind"`
		TransferDirection string     `json:"transferDirection,omitempty"`
		Type              string     `json:"type"`
		Status            string     `json:"status"`
		Amount            float64    `json:"amount"`
		InstallmentNumber *int       `json:"installmentNumber,omitempty"`
		AccountID         *uuid.UUID `json:"accountId,omitempty"`
		AccountName       string     `json:"accountName,omitempty"`
		Source            string     `json:"source"`
		Note              string     `json:"note"`
		BankTime          *time.Time `json:"bankTime,omitempty"`
	}
	items := make([]item, len(txs))
	for i, t := range txs {
		source := "MANUAL"
		if t.FromOpenFinance {
			source = "OPEN_FINANCE"
		}
		items[i] = item{t.ID, t.Date.Format(time.DateOnly), t.Description, t.Category, domain.Category(t.Category).Label(),
			t.Subcategory, domain.Subcategory(t.Subcategory).Label(), t.Essential,
			t.PaymentMethod, domain.PaymentMethod(t.PaymentMethod).Label(), t.Kind, t.TransferDirection, t.Type, t.Status,
			t.Amount, t.InstallmentNumber, t.AccountID, t.AccountName, source, t.Note, t.BankTime}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "limit": f.Limit})
}

const maxNoteBody = 2 << 10

// setTransactionNote grava a nota de uma transação ("o que foi"); nota vazia apaga.
func (a *api) setTransactionNote(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "transação não encontrada")
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	if !decodeJSON(w, r, maxNoteBody, &body) {
		return
	}
	note, err := domain.NormalizeNote(body.Note)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	found, err := a.reader.SetTransactionNote(r.Context(), id, note)
	if err != nil {
		a.fail(w, "nota da transação", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "transação não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"note": note})
}

// deleteTransaction apaga um lançamento manual avulso. Lançamento do banco não pode ser apagado (voltaria na
// sincronização); parcelado e recorrente se cancelam pelo chat.
func (a *api) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "lançamento não encontrado")
		return
	}
	deleted, err := a.reader.DeleteManualTransaction(r.Context(), id)
	if err != nil {
		a.fail(w, "apagar lançamento", err)
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "só dá para apagar lançamento manual avulso")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) registerTransactions(rt routes) {
	rt.mux.Handle("DELETE /api/transactions/{id}", rt.protected(sameOriginOnly(a.deleteTransaction)))
	rt.mux.Handle("PUT /api/transactions/{id}/note", rt.protected(jsonOnly(a.setTransactionNote)))
	rt.mux.Handle("GET /api/transactions", rt.protected(a.transactions))
	rt.mux.Handle("GET /api/transactions/groups", rt.protected(a.transactionGroups))
	rt.mux.Handle("GET /api/transactions/expected-income", rt.protected(a.expectedIncome))
	rt.mux.Handle("GET /api/transactions/expected-expenses", rt.protected(a.expectedExpenses))
}

type expectedIncomeJSON struct {
	Source string  `json:"source"`
	Label  string  `json:"label"`
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Late   bool    `json:"late"`
}

// expectedIncome: as receitas do ciclo (month, padrão o atual) que já se sabe que vão cair e ainda não chegaram do
// banco: partes do salário e entradas recorrentes.
func (a *api) expectedIncome(w http.ResponseWriter, r *http.Request) {
	month, ok := a.monthParam(w, r, "month", false)
	if !ok {
		return
	}
	if month.IsZero() {
		month = a.monthStart()
	}
	list, err := a.insights.ExpectedIncomes(r.Context(), month, a.localNow())
	if err != nil {
		a.fail(w, "receitas previstas", err)
		return
	}
	items := make([]expectedIncomeJSON, len(list))
	var total float64
	for n, e := range list {
		items[n] = expectedIncomeJSON{Source: e.Source, Label: e.Label, Date: e.Date.Format(time.DateOnly), Amount: e.Amount, Late: e.Late}
		total += e.Amount
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": cents(total)})
}

// expectedExpenses: as saídas recorrentes do ciclo (month, padrão o atual) que já se sabe que vão sair e ainda não
// saíram do banco: aluguel, seguro, financiamento...
func (a *api) expectedExpenses(w http.ResponseWriter, r *http.Request) {
	month, ok := a.monthParam(w, r, "month", false)
	if !ok {
		return
	}
	if month.IsZero() {
		month = a.monthStart()
	}
	list, err := a.insights.ExpectedExpenses(r.Context(), month, a.localNow())
	if err != nil {
		a.fail(w, "despesas previstas", err)
		return
	}
	items := make([]expectedIncomeJSON, len(list))
	var total float64
	for n, e := range list {
		items[n] = expectedIncomeJSON{Source: e.Source, Label: e.Label, Date: e.Date.Format(time.DateOnly), Amount: e.Amount, Late: e.Late}
		total += e.Amount
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": cents(total)})
}
