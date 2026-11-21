package debts

import (
	"fmt"
	"math"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// flowTolerance: a transação conta para o mês se o valor bate com o do mês com até 2% (ou R$ 1) de diferença. Quando ela é
// a única com aquele nome no mês, vale até flowLooseTolerance (conta que varia, como seguro e luz).
const (
	flowTolerance      = 0.02
	flowLooseTolerance = 0.35
)

// FlowAmount é o valor da recorrência no mês de month; ok falso antes do início ou depois da última vez.
func FlowAmount(f domain.RecurringFlow, month time.Time) (amount float64, ok bool) {
	idx := domain.MonthsBetween(f.StartMonth, month)
	if idx < 0 || f.Installments > 0 && idx >= f.Installments {
		return 0, false
	}
	amount = f.Amount + f.Step*float64(idx)
	if f.MaxAmount > 0 {
		amount = min(amount, f.MaxAmount)
	}
	return round(amount), true
}

// FlowView é a situação de uma recorrência.
type FlowView struct {
	Done          int // meses, do início até este, em que a transação foi reconhecida
	Total         int // vezes no total (0 = sem fim)
	ThisMonth     float64
	Active        bool // vale neste mês
	ThisMonthDone bool
	NextDate      *time.Time
	NextAmount    float64
	Remaining     float64 // com número de vezes: soma do que falta; sem fim: 0
	Ended         bool
	Matches       []domain.MoneyMovement // transações reconhecidas, da mais recente à mais antiga
}

func monthOf(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC) }

// FlowStatus reconhece, em moves, as transações da recorrência: mesmo sentido, nome em PayeeMatch (já filtrado pelo
// banco) e valor do mês. Cada mês conta uma vez.
func FlowStatus(f domain.RecurringFlow, moves []domain.MoneyMovement, today time.Time) FlowView {
	v := FlowView{Total: f.Installments}
	current := monthOf(today)
	done := map[time.Time]bool{}
	var mine []domain.MoneyMovement
	perMonth := map[time.Time]int{}
	for _, m := range moves {
		if m.In == f.In && domain.MatchesPayee(m.Label, f.PayeeMatch) {
			mine = append(mine, m)
			perMonth[monthOf(m.Date)]++
		}
	}
	for _, m := range mine {
		month := monthOf(m.Date)
		want, ok := FlowAmount(f, month)
		tol := max(1, want*flowTolerance)
		if perMonth[month] == 1 {
			tol = max(tol, want*flowLooseTolerance)
		}
		if !ok || done[month] || math.Abs(m.Amount-want) > tol {
			continue
		}
		done[month] = true
		v.Matches = append([]domain.MoneyMovement{m}, v.Matches...)
	}
	for m := monthOf(f.StartMonth); !m.After(current); m = m.AddDate(0, 1, 0) {
		if _, ok := FlowAmount(f, m); ok && done[m] {
			v.Done++
		}
	}
	v.ThisMonth, v.Active = FlowAmount(f, current)
	v.ThisMonthDone = done[current]
	for m, n := current, 0; n < 240; m, n = m.AddDate(0, 1, 0), n+1 {
		amount, ok := FlowAmount(f, m)
		if !ok {
			if f.Installments > 0 && domain.MonthsBetween(f.StartMonth, m) >= f.Installments {
				break
			}
			continue
		}
		if m.Equal(current) && v.ThisMonthDone {
			continue
		}
		if v.NextDate == nil {
			d := domain.DebtDueIn(m, f.Day)
			v.NextDate, v.NextAmount = &d, amount
		}
		if f.Installments == 0 {
			break
		}
	}
	if f.Installments > 0 {
		for i := 0; i < f.Installments; i++ {
			m := monthOf(f.StartMonth).AddDate(0, i, 0)
			if amount, _ := FlowAmount(f, m); !done[m] {
				v.Remaining += amount
			}
		}
		v.Remaining = round(v.Remaining)
		v.Ended = v.NextDate == nil
	}
	return v
}

// FlowLines são as ocorrências da recorrência de hoje até until, para a previsão: entrada positiva, saída negativa. A do
// mês atual que já aconteceu fica de fora; a que já passou do dia e não apareceu entra hoje, como atrasada.
func FlowLines(f domain.RecurringFlow, v FlowView, today, until time.Time) []Line {
	var out []Line
	today = civil(today)
	for m := monthOf(today); !m.After(until); m = m.AddDate(0, 1, 0) {
		amount, ok := FlowAmount(f, m)
		if !ok || m.Equal(monthOf(today)) && v.ThisMonthDone {
			continue
		}
		date := domain.DebtDueIn(m, f.Day)
		label := f.Name
		if f.Party != "" {
			label += " (" + f.Party + ")"
		}
		if date.Before(today) {
			date, label = today, label+" · atrasada"
		}
		if date.After(until) {
			continue
		}
		if f.Installments > 0 {
			label += fmt.Sprintf(" · %d/%d", domain.MonthsBetween(f.StartMonth, m)+1, f.Installments)
		}
		if !f.In {
			amount = -amount
		}
		d := date
		out = append(out, Line{Label: label, Amount: amount, Date: &d})
	}
	return out
}
