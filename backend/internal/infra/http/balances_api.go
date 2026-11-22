package httpserver

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/balances"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/openfinance"

	"github.com/google/uuid"
)

// syncTimeout dá folga para vários itens do Pluggy; a sincronização não é cortada se a aba for fechada no meio.
const syncTimeout = 5 * time.Minute

// oldestSourceUpdate é a atualização mais antiga, entre os bancos, que o Pluggy informou (nil se nenhuma). É a idade real dos
// dados: sincronizar copia o que o Pluggy já tem, e o Meu Pluggy só busca no banco sozinho (cerca de 1x/dia) ou a pedido.
func (a *api) oldestSourceUpdate(ctx context.Context) *time.Time {
	insts, err := a.reader.Institutions(ctx)
	if err != nil {
		return nil
	}
	var oldest *time.Time
	for _, in := range insts {
		if in.SourceUpdatedAt != nil && (oldest == nil || in.SourceUpdatedAt.Before(*oldest)) {
			oldest = in.SourceUpdatedAt
		}
	}
	return oldest
}

type balanceAccountJSON struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	Last4         string                   `json:"last4"`
	Balance       float64                  `json:"balance"`
	AutoInvested  *float64                 `json:"autoInvested"`
	Reserved      []domain.ReservedBalance `json:"reserved"`
	ReservedTotal float64                  `json:"reservedTotal"`
	// Overdraft: quanto do limite a conta usa para cobrir a dívida OverdraftDebt (o saldo aparece zerado).
	Overdraft     float64 `json:"overdraft"`
	OverdraftDebt string  `json:"overdraftDebt"`
}

type balanceCardJSON struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Brand          string         `json:"brand"`
	Last4          string         `json:"last4"`
	Invoice        float64        `json:"invoice"`
	Limit          *float64       `json:"limit"`
	Available      *float64       `json:"available"`
	Used           *float64       `json:"used"`
	UsedRatio      *float64       `json:"usedRatio"`
	UsageLevel     balances.Level `json:"usageLevel"`
	CloseDate      *string        `json:"closeDate"`
	DueDate        *string        `json:"dueDate"`
	DaysToDue      *int           `json:"daysToDue"`
	DueLevel       balances.Level `json:"dueLevel"`
	MinimumPayment *float64       `json:"minimumPayment"`
	NextBillDue    *string        `json:"nextBillDue"`
	NextClose      *string        `json:"nextClose"`
	CloseEstimated bool           `json:"closeEstimated"`
}

type institutionJSON struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Color     *string              `json:"color"`
	Logo      *string              `json:"logo"`
	UpdatedAt time.Time            `json:"updatedAt"`
	Stale     bool                 `json:"stale"`
	Total     *float64             `json:"total"`
	Share     float64              `json:"shareOfTotal"`
	Accounts  []balanceAccountJSON `json:"accounts"`
	Cards     []balanceCardJSON    `json:"cards"`
}

func dayString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02")
	return &s
}

// balances devolve os saldos por banco. O item_id do Pluggy nunca sai: o id da instituição é opaco.
func (a *api) balances(w http.ResponseWriter, r *http.Request) {
	view, err := a.insights.Balances(r.Context(), a.now())
	if err != nil {
		a.fail(w, "saldos", err)
		return
	}
	institutions := make([]institutionJSON, len(view.Institutions))
	for i, in := range view.Institutions {
		j := institutionJSON{
			ID: in.ID, Name: in.Name, UpdatedAt: in.UpdatedAt, Stale: in.Stale, Share: in.Share,
			Accounts: make([]balanceAccountJSON, len(in.Accounts)), Cards: make([]balanceCardJSON, len(in.Cards)),
		}
		if in.Color != "" {
			c := "#" + in.Color
			j.Color = &c
		}
		if in.HasLogo {
			l := "/api/institutions/" + in.ID + "/logo"
			j.Logo = &l
		}
		if in.HasBank {
			t := in.Total
			j.Total = &t
		}
		for n, acc := range in.Accounts {
			reserved := acc.Reserved
			if reserved == nil {
				reserved = []domain.ReservedBalance{}
			}
			j.Accounts[n] = balanceAccountJSON{acc.ID, acc.Name, acc.Last4, acc.Balance, acc.AutoInvested, reserved, cents(acc.ReservedTotal), cents(acc.Overdraft), acc.OverdraftDebt}
		}
		for n, c := range in.Cards {
			j.Cards[n] = balanceCardJSON{
				ID: c.ID, Name: c.Name, Brand: c.Brand, Last4: c.Last4, Invoice: c.Invoice, Limit: c.Limit, Available: c.Available,
				Used: c.Used, UsedRatio: c.UsedRatio, UsageLevel: c.UsageLevel, CloseDate: dayString(c.CloseDate),
				DueDate: dayString(c.DueDate), DaysToDue: c.DaysToDue, DueLevel: c.DueLevel, MinimumPayment: c.MinimumPayment,
				NextBillDue: dayString(c.NextBillDue), NextClose: dayString(c.NextClose), CloseEstimated: c.CloseEstimated,
			}
		}
		institutions[i] = j
	}
	var asOf *time.Time
	if !view.AsOf.IsZero() {
		asOf = &view.AsOf
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"asOf": asOf, "totalInAccount": view.TotalInAccount, "openInvoices": view.OpenInvoices, "institutions": institutions,
	})
}

// institutionLogo serve o logo em cache. O tipo vem de uma lista fechada (checada no banco e na busca); mesmo assim o
// SVG vai com CSP restritivo e nosniff, para que não execute nada nem se for aberto direto.
func (a *api) institutionLogo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "logo não encontrado")
		return
	}
	data, mimeType, found, err := a.reader.InstitutionLogo(r.Context(), id)
	if err != nil {
		a.fail(w, "logo da instituição", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "logo não encontrado")
		return
	}
	h := w.Header()
	h.Set("Content-Type", mimeType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Cache-Control", "private, max-age=86400")
	w.Write(data) //nolint:errcheck
}

// syncNow dispara a mesma sincronização do /sync do chat. É a única escrita sem corpo da API: exige JSON e mesma origem.
func (a *api) syncNow(w http.ResponseWriter, r *http.Request) {
	if a.syncer == nil || a.settings != nil && !a.settings.Service.Sync().Configured() {
		writeError(w, http.StatusServiceUnavailable, "o Open Finance não está configurado")
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), syncTimeout)
	defer cancel()

	result, err := a.syncer.Sync(ctx)
	if errors.Is(err, openfinance.ErrNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "o Open Finance não está configurado")
		return
	}
	if errors.Is(err, openfinance.ErrSyncInProgress) {
		writeError(w, http.StatusConflict, "já existe uma sincronização em andamento")
		return
	}
	if err != nil {
		// Falhas parciais (um item) já foram registradas; as contas que deram certo foram atualizadas.
		a.logger.Error("erro na sincronização pelo dashboard", "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"inserted": result.Inserted, "reconciled": result.Reconciled, "existing": result.Existing, "positions": result.Positions,
		"partial": err != nil, "dataAsOf": a.oldestSourceUpdate(r.Context()),
	})
}

type salaryPartJSON struct {
	Day            int      `json:"day"`
	ExpectedDate   string   `json:"expectedDate"`
	Amount         float64  `json:"amount"`
	Received       bool     `json:"received"`
	ReceivedDate   *string  `json:"receivedDate"`
	ReceivedAmount *float64 `json:"receivedAmount"`
}

// salary devolve as partes do salário do mês atual e quais já caíram. configured=false quando não há salário configurado.
func (a *api) salary(w http.ResponseWriter, r *http.Request) {
	now := a.now()
	if a.settings != nil {
		now = now.In(a.settings.Service.Location())
	}
	view, err := a.insights.Salary(r.Context(), now)
	if err != nil {
		a.fail(w, "salário", err)
		return
	}
	status := view.Parts
	parts := make([]salaryPartJSON, len(status))
	var expected, received, pending float64
	for i, s := range status {
		p := salaryPartJSON{Day: s.Day, ExpectedDate: s.Expected.Format("2006-01-02"), Amount: s.Amount}
		expected += s.Amount
		if s.Received != nil {
			p.Received = true
			p.ReceivedDate = dayString(&s.Received.Date)
			amount := s.Received.Amount
			p.ReceivedAmount = &amount
			received += amount
		} else {
			pending += s.Amount
		}
		parts[i] = p
	}
	type extraJSON struct {
		Date   string  `json:"date"`
		Amount float64 `json:"amount"`
	}
	extras := make([]extraJSON, len(view.Extras))
	var extrasTotal float64
	for i, e := range view.Extras {
		extras[i] = extraJSON{e.Date.Format("2006-01-02"), e.Amount}
		extrasTotal += e.Amount
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"extras": extras, "extrasTotal": cents(extrasTotal),
		"configured": view.Configured, "month": formatMonth(a.monthStart()), "parts": parts,
		"expected": cents(expected), "received": cents(received), "pending": cents(pending),
	})
}

// localNow é agora no fuso do dono (configuração); sem configuração, o relógio como veio.
func (a *api) localNow() time.Time {
	now := a.now()
	if a.settings != nil {
		now = now.In(a.settings.Service.Location())
	}
	return now
}

// cents arredonda somas de valores em reais para centavos (evita 9566.619999 na resposta).
func cents(v float64) float64 { return math.Round(v*100) / 100 }

func (a *api) registerBalances(rt routes) {
	rt.mux.Handle("GET /api/balances", rt.protected(a.balances))
	rt.mux.Handle("GET /api/salary", rt.protected(a.salary))
	rt.mux.Handle("GET /api/institutions/{id}/logo", rt.protected(a.institutionLogo))
	// Cada sincronização consulta o Pluggy: limite apertado por IP, além de uma por vez (409).
	rt.mux.Handle("POST /api/sync", newIPRateLimiter(3, time.Minute).middleware(rt.protected(jsonOnly(a.syncNow))))
}
