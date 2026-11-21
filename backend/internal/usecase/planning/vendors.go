package planning

import (
	"slices"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// VendorProgress é a situação de um fornecedor hoje.
type VendorProgress struct {
	Vendor    domain.Vendor
	Paid      float64
	Remaining float64 // contratado − pago, nunca negativo
	Done      bool
	// Contributions é quantos aportes (no dia do aporte) cabem de NextContribution até o vencimento; 0 sem prazo
	// ou quando o vencimento vem antes do próximo aporte.
	Contributions int
	Late          bool // venceu (ou vence antes do próximo aporte) e ainda falta pagar
}

// VendorSummary é a conta do plano inteiro, como na planilha: contratado, pago, guardado, falta pagar e quanto
// guardar por mês até o último vencimento. Nada disso é gravado.
type VendorSummary struct {
	Plan       domain.VendorPlan
	Contracted float64
	Paid       float64
	Saved      float64 // saldo da caixinha ligada, ou o valor informado à mão
	// SavedFromBox diz se Saved veio da caixinha; false quando ela não foi encontrada nas contas (ou não há caixinha).
	SavedFromBox bool
	Remaining    float64 // contratado − pago − guardado, nunca negativo

	NextContribution time.Time  // próximo dia de aporte (hoje conta, se for o dia)
	LastDue          *time.Time // vencimento mais distante entre os que ainda faltam pagar; nil sem prazo
	MonthsLeft       int        // aportes de NextContribution até LastDue (a fórmula da planilha: DATEDIF + 1)

	// PerMonth é quanto guardar por aporte para pagar cada fornecedor até o seu vencimento: o maior entre os
	// prazos (um fornecedor que vence cedo pode exigir mais que a média). FlatPerMonth é a conta da planilha:
	// o que falta dividido pelos aportes até o último vencimento.
	PerMonth     float64
	FlatPerMonth float64
	// Tight é o fornecedor cujo prazo define PerMonth quando ele é maior que FlatPerMonth; nil quando a média basta.
	Tight *domain.Vendor

	Surplus *float64 // sobra mensal média projetada até LastDue; nil sem histórico
	Fits    *bool    // PerMonth cabe na sobra; nil quando Surplus é desconhecida ou não há o que guardar

	Vendors []VendorProgress // pela ordem do vencimento; sem prazo e quitados por último
}

// NextContributionDate é o próximo dia de aporte a partir de today (o próprio dia conta).
func NextContributionDate(today time.Time, day int) time.Time {
	d := time.Date(today.Year(), today.Month(), day, 0, 0, 0, 0, time.UTC)
	if today.Day() > day {
		d = d.AddDate(0, 1, 0)
	}
	return d
}

// contributionsUntil conta os aportes de next (inclusive) até due, um por mês no dia de next; 0 se due vem antes.
func contributionsUntil(next, due time.Time) int {
	if due.Before(next) {
		return 0
	}
	m := (due.Year()-next.Year())*12 + int(due.Month()-next.Month())
	if due.Day() < next.Day() {
		m--
	}
	return m + 1
}

// BuildVendorSummary calcula o plano. boxes são as caixinhas das contas correntes (nome → saldo somado) e p é a
// projeção que diz a sobra mensal. today é o dia de hoje.
func BuildVendorSummary(plan domain.VendorPlan, vendors []domain.Vendor, boxes map[string]float64, p Projection, today time.Time) VendorSummary {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	s := VendorSummary{Plan: plan, NextContribution: NextContributionDate(today, plan.ContributionDay)}
	if amount, ok := boxes[plan.SavingsBox]; ok && plan.SavingsBox != "" {
		s.Saved, s.SavedFromBox = amount, true
	} else if plan.SavingsBox == "" {
		s.Saved = plan.SavedManual
	}

	for _, v := range vendors {
		vp := VendorProgress{Vendor: v, Paid: round2(v.Paid())}
		vp.Remaining = max(0, round2(v.Contracted-vp.Paid))
		vp.Done = vp.Remaining == 0
		if v.DueDate != nil && !vp.Done {
			vp.Contributions = contributionsUntil(s.NextContribution, *v.DueDate)
			vp.Late = vp.Contributions == 0
			if s.LastDue == nil || v.DueDate.After(*s.LastDue) {
				d := *v.DueDate
				s.LastDue = &d
			}
		}
		s.Contracted += v.Contracted
		s.Paid += vp.Paid
		s.Vendors = append(s.Vendors, vp)
	}
	s.Contracted, s.Paid = round2(s.Contracted), round2(s.Paid)
	s.Remaining = max(0, round2(s.Contracted-s.Paid-s.Saved))

	slices.SortStableFunc(s.Vendors, func(a, b VendorProgress) int {
		ka, kb := vendorOrder(a), vendorOrder(b)
		if ka != kb {
			return ka - kb
		}
		if a.Vendor.DueDate != nil && b.Vendor.DueDate != nil {
			return a.Vendor.DueDate.Compare(*b.Vendor.DueDate)
		}
		return 0
	})

	if s.LastDue == nil || s.Remaining == 0 {
		return s
	}
	s.MonthsLeft = max(1, contributionsUntil(s.NextContribution, *s.LastDue))
	s.FlatPerMonth = round2(s.Remaining / float64(s.MonthsLeft))
	s.PerMonth = s.FlatPerMonth

	// Prazo a prazo: o que vence até cada data (menos o guardado, que paga primeiro o que vence antes) dividido
	// pelos aportes até ela. Fornecedores sem prazo só entram no total, que já está no último prazo.
	var due float64
	for i, vp := range s.Vendors {
		if vp.Done || vp.Vendor.DueDate == nil {
			continue
		}
		due += vp.Remaining
		need := round2(max(0, due-s.Saved) / float64(max(1, vp.Contributions)))
		if need > s.PerMonth {
			s.PerMonth = need
			v := s.Vendors[i].Vendor
			s.Tight = &v
		}
	}

	now := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	s.Surplus = projectedSurplus(p, now, s.MonthsLeft)
	if s.Surplus != nil {
		fits := s.PerMonth <= *s.Surplus
		s.Fits = &fits
	}
	return s
}

// vendorOrder: com prazo e em aberto primeiro, depois sem prazo, depois quitados.
func vendorOrder(v VendorProgress) int {
	switch {
	case v.Done:
		return 2
	case v.Vendor.DueDate == nil:
		return 1
	}
	return 0
}
