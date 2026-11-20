package httpserver

import (
	"net/http"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/planning"
)

type sheetLineJSON struct {
	Label  string  `json:"label"`
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
	Note   string  `json:"note"`
}

type sheetItemJSON struct {
	Label   string          `json:"label"`
	Amount  float64         `json:"amount"`
	Count   int             `json:"count"`
	Pending bool            `json:"pending"`
	Lines   []sheetLineJSON `json:"lines"`
}

func sheetItems(in []planning.SheetItem) []sheetItemJSON {
	out := make([]sheetItemJSON, len(in))
	for i, it := range in {
		lines := make([]sheetLineJSON, len(it.Lines))
		for n, l := range it.Lines {
			lines[n] = sheetLineJSON{l.Label, l.Date.Format("2006-01-02"), l.Amount, l.Note}
		}
		out[i] = sheetItemJSON{it.Label, it.Amount, it.Count, it.Pending, lines}
	}
	return out
}

// monthSheet: o mês no formato da planilha (entradas, fixas, outras, fatura de cada cartão por categoria e sobra).
func (a *api) monthSheet(w http.ResponseWriter, r *http.Request) {
	month, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	s, err := a.insights.MonthSheet(r.Context(), month)
	if err != nil {
		a.fail(w, "mês", err)
		return
	}
	type pendingJSON struct {
		Label  string  `json:"label"`
		Date   string  `json:"date"`
		Amount float64 `json:"amount"`
	}
	type cardJSON struct {
		Name       string          `json:"name"`
		Total      float64         `json:"total"`
		Categories []sheetItemJSON `json:"categories"`
		Forecast   float64         `json:"forecast"`
		Adjustment float64         `json:"adjustment"`
		Refunds    float64         `json:"refunds"`
	}
	pending := make([]pendingJSON, len(s.Pending))
	for i, p := range s.Pending {
		pending[i] = pendingJSON{p.Label, p.Date.Format("2006-01-02"), p.Amount}
	}
	cards := make([]cardJSON, len(s.Cards))
	for i, c := range s.Cards {
		cards[i] = cardJSON{c.Name, c.Total, sheetItems(c.Categories), c.Forecast, c.Adjustment, c.Refunds}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"month":  formatMonth(s.Month),
		"income": sheetItems(s.Income), "incomeTotal": s.IncomeTotal,
		"pending": pending, "pendingTotal": s.PendingTotal,
		"fixed": sheetItems(s.Fixed), "fixedTotal": s.FixedTotal,
		"other": sheetItems(s.Other), "otherTotal": s.OtherTotal,
		"cards": cards, "cardsTotal": s.CardsTotal,
		"expensesTotal": s.ExpensesTotal, "leftover": s.Leftover, "leftoverWithPending": s.LeftoverWithPending,
		"invested": s.Invested,
	})
}
