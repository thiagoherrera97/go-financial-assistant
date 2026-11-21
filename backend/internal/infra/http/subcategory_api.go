package httpserver

import (
	"net/http"
	"slices"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

const maxClassificationBody = 1 << 10

// essentialFilters são os valores aceitos no filtro de essencial/supérfluo das transações.
var essentialFilters = []string{"true", "false", "unknown"}

// subcategoryCatalog: as categorias que uma despesa pode ter e as subcategorias de cada uma, com o essencial/supérfluo
// padrão (null = depende do caso).
func (a *api) subcategoryCatalog(w http.ResponseWriter, _ *http.Request) {
	type sub struct {
		Code      string `json:"code"`
		Category  string `json:"category"`
		Label     string `json:"label"`
		Essential *bool  `json:"essential"`
	}
	type category struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}
	cats := make([]category, len(ruleCategories))
	for i, c := range ruleCategories {
		cats[i] = category{string(c), c.Label()}
	}
	all := domain.Subcategories()
	subs := make([]sub, len(all))
	for i, s := range all {
		subs[i] = sub{string(s.Code), string(s.Category), s.Label, s.Essential}
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats, "subcategories": subs})
}

// setClassification corrige a categoria, a subcategoria e o essencial/supérfluo de uma despesa; com applyToAll, vale para
// as outras do mesmo estabelecimento e para as próximas.
func (a *api) setClassification(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "transação não encontrada")
		return
	}
	var body struct {
		Category    string `json:"category"`
		Subcategory string `json:"subcategory"`
		Essential   *bool  `json:"essential"`
		ApplyToAll  bool   `json:"applyToAll"`
	}
	if !decodeJSON(w, r, maxClassificationBody, &body) {
		return
	}
	c := domain.Classification{Category: domain.Category(body.Category), Subcategory: domain.Subcategory(body.Subcategory),
		Essential: body.Essential, ApplyToAll: body.ApplyToAll}
	if !slices.Contains(ruleCategories, c.Category) {
		writeError(w, http.StatusBadRequest, "category inválida")
		return
	}
	if c.Subcategory != "" && !c.Subcategory.BelongsTo(c.Category) {
		writeError(w, http.StatusBadRequest, "subcategory não é dessa categoria")
		return
	}
	found, others, err := a.reader.SetClassification(r.Context(), id, c)
	if err != nil {
		a.fail(w, "classificação da transação", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "só dá para classificar despesas")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"others": others})
}

// subcategorySpend: os gastos do mês por subcategoria, com os totais de essencial, supérfluo e a revisar. O ranking de
// supérfluos ("onde cortar") sai daqui.
func (a *api) subcategorySpend(w http.ResponseWriter, r *http.Request) {
	month, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	items, err := a.reader.SpendBySubcategory(r.Context(), month)
	if err != nil {
		a.fail(w, "gastos por subcategoria", err)
		return
	}
	type item struct {
		Category         string  `json:"category"`
		CategoryLabel    string  `json:"categoryLabel"`
		Subcategory      string  `json:"subcategory"`
		SubcategoryLabel string  `json:"subcategoryLabel"`
		Essential        *bool   `json:"essential"`
		Count            int     `json:"count"`
		Total            float64 `json:"total"`
		PrevAverage      float64 `json:"prevAverage"`
	}
	var essential, superfluous, unknown float64
	out := make([]item, len(items))
	for i, s := range items {
		label := s.Subcategory.Label()
		if label == "" {
			label = s.Category.Label()
		}
		out[i] = item{string(s.Category), s.Category.Label(), string(s.Subcategory), label, s.Essential, s.Count, s.Total, s.PrevAverage}
		switch {
		case s.Essential == nil:
			unknown += s.Total
		case *s.Essential:
			essential += s.Total
		default:
			superfluous += s.Total
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"month": formatMonth(month), "essential": essential, "superfluous": superfluous, "unknown": unknown, "items": out,
	})
}

func (a *api) registerSubcategories(rt routes) {
	rt.mux.Handle("GET /api/subcategories", rt.protected(a.subcategoryCatalog))
	rt.mux.Handle("GET /api/spending/subcategories", rt.protected(a.subcategorySpend))
	rt.mux.Handle("PUT /api/transactions/{id}/classification", rt.protected(jsonOnly(a.setClassification)))
}
