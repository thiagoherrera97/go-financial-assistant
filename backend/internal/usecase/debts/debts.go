// Package debts calcula a situação das dívidas pessoais: as parcelas pagas e as que faltam, e, na dívida coberta pelo
// cheque especial, quanto do limite está em uso, há quantos dias e a previsão do próximo mergulho.
package debts

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// paymentTolerance: um Pix conta como parcela se o valor for uma parcela (ou algumas) com até 2% de diferença. Outros
// valores para a mesma pessoa (presente, divisão de conta) não abatem a dívida.
const paymentTolerance = 0.02

// InstallmentView é a situação de uma dívida parcelada.
type InstallmentView struct {
	Paid       int     // parcelas pagas (Pix reconhecidos + marcadas à mão), até Total
	Total      int     // número de parcelas
	PaidAmount float64 // valor já pago
	Remaining  float64 // quanto falta
	NextDue    *time.Time
	Overdue    bool // a próxima parcela já venceu e não foi paga
	Done       bool
	Payments   []domain.MoneyMovement // Pix reconhecidos, do mais recente ao mais antigo
}

// Installments soma os Pix enviados que têm valor de parcela (moves; os recebidos são ignorados) e as parcelas marcadas
// à mão. Um Pix de duas parcelas conta como duas.
func Installments(d domain.Debt, moves []domain.MoneyMovement, today time.Time) InstallmentView {
	v := InstallmentView{Total: d.Installments}
	paid := float64(d.PaidManual) * d.InstallmentAmount
	for _, h := range moves {
		if h.In || d.InstallmentAmount <= 0 || h.Date.Before(d.FirstDue.AddDate(0, -1, 0)) {
			continue
		}
		left := d.Installments - int(math.Floor(paid/d.InstallmentAmount+0.01))
		k := math.Round(h.Amount / d.InstallmentAmount)
		if k >= 1 && int(k) <= left && math.Abs(h.Amount-k*d.InstallmentAmount) <= k*d.InstallmentAmount*paymentTolerance {
			paid += h.Amount
			v.Payments = append(v.Payments, h)
		}
	}
	sort.Slice(v.Payments, func(i, j int) bool { return v.Payments[i].Date.After(v.Payments[j].Date) })
	v.PaidAmount = round(min(paid, d.OriginalAmount))
	v.Remaining = round(max(0, d.OriginalAmount-paid))
	if d.InstallmentAmount > 0 {
		v.Paid = min(d.Installments, int(math.Floor(paid/d.InstallmentAmount+0.01)))
	}
	v.Done = v.Remaining <= 0.005 || v.Paid >= v.Total
	if !v.Done {
		next := domain.DebtDueIn(d.FirstDue.AddDate(0, v.Paid, 1-d.FirstDue.Day()), d.DueDay)
		v.NextDue = &next
		v.Overdue = next.Before(civil(today))
	}
	return v
}

// MonthUse é quanto uma conta usou do limite num mês, segundo os saldos registrados.
type MonthUse struct {
	Month time.Time
	Days  int     // dias em que o saldo ficou negativo
	Worst float64 // o maior uso do limite (positivo)
}

// LimitUse é o uso do limite agora: quanto, desde quando e o histórico por mês.
type LimitUse struct {
	InLimit  float64    // quanto do limite está em uso agora (0 se a conta está positiva)
	Since    *time.Time // primeiro dia da sequência atual no limite; nil se não está no limite
	DaysUsed int        // dias seguidos no limite contando hoje
	Months   []MonthUse // do mais recente ao mais antigo; só meses com registro
	// AtDue é quanto a conta ficou negativa no último dia de vencimento (ex.: dia 10) até hoje: o tamanho atual da dívida.
	// 0 quando a conta fechou positiva nesse dia (dívida quitada). DueDate é esse dia; nil sem saldo registrado nele.
	AtDue   float64
	DueDate *time.Time
}

// DueBalance devolve o saldo da conta no último dia dueDay até hoje: o saldo atual se hoje é o dia, senão o saldo do fim
// do dia registrado (ou o último registro até 3 dias antes, se faltou o do dia). ok=false sem registro.
func DueBalance(balance float64, days []domain.BalanceDay, today time.Time, dueDay int) (due time.Time, last float64, ok bool) {
	today = civil(today)
	due = domain.DebtDueIn(today, dueDay)
	if due.After(today) {
		due = domain.DebtDueIn(today.AddDate(0, 0, 1-today.Day()).AddDate(0, -1, 0), dueDay)
	}
	if due.Equal(today) {
		return due, balance, true
	}
	var best time.Time
	for _, d := range days {
		day := civil(d.Day)
		if day.After(due) || day.Before(due.AddDate(0, 0, -3)) || day.Before(best) {
			continue
		}
		best, last, ok = day, d.Last, true
	}
	return due, last, ok
}

// Usage lê o uso do limite a partir do saldo atual e dos saldos registrados por dia.
func Usage(balance float64, days []domain.BalanceDay, today time.Time) LimitUse {
	u := LimitUse{InLimit: round(max(0, -balance))}
	today = civil(today)
	byMonth := map[time.Time]*MonthUse{}
	negative := map[time.Time]bool{}
	for _, d := range days {
		day := civil(d.Day)
		m := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
		mu := byMonth[m]
		if mu == nil {
			mu = &MonthUse{Month: m}
			byMonth[m] = mu
		}
		if d.Min < 0 {
			mu.Days++
			mu.Worst = round(max(mu.Worst, -d.Min))
			negative[day] = true
		}
	}
	if u.InLimit > 0 {
		negative[today] = true
		since := today
		for negative[since.AddDate(0, 0, -1)] {
			since = since.AddDate(0, 0, -1)
		}
		u.Since = &since
		u.DaysUsed = int(today.Sub(since).Hours()/24) + 1
	}
	for _, mu := range byMonth {
		u.Months = append(u.Months, *mu)
	}
	sort.Slice(u.Months, func(i, j int) bool { return u.Months[i].Month.After(u.Months[j].Month) })
	return u
}

// Line é uma linha da previsão: entrada (positiva) ou saída (negativa).
type Line struct {
	Label    string
	Amount   float64
	Date     *time.Time
	Estimate bool // gasto que ainda não aconteceu, estimado pelo mês passado (ex.: compras no cartão até o fechamento)
}

// ForecastInput é o que a previsão do próximo mergulho no limite usa.
type ForecastInput struct {
	Today    time.Time
	Dip      time.Time // dia em que a conta costuma entrar no limite
	Until    time.Time // véspera do primeiro salário depois de Dip: até onde as saídas contam (zero = Dip)
	Balance  float64   // saldo da conta agora
	FreeDays int
	Salaries []Line // partes do salário a cair, em ordem de data (podem passar de Dip)
	Bills    []Line // faturas e parcelas que saem da conta até Dip (valores negativos)
	Usual    *Line  // o que saiu da conta no mesmo período do mês passado (negativo), se houver
}

// Forecast é a previsão do saldo no dia do mergulho e de quando o salário tira a conta do limite.
type Forecast struct {
	Date    time.Time
	Until   time.Time // até quando as saídas foram contadas (véspera do salário)
	Lines   []Line    // como se chega ao saldo previsto, a partir do saldo de hoje
	Balance float64   // saldo previsto no dia (negativo = no limite)
	InLimit float64   // quanto do limite vai usar (0 se não entra)
	// Committed é o saldo previsto sem as estimativas (só o que já está lançado ou se repete todo mês): o piso se não
	// houver compras novas. CommittedInLimit é o uso do limite nesse caso.
	Committed        float64
	CommittedInLimit float64
	ExitDate         *time.Time
	ExitLabel        string
	Days             int // dias no limite até o salário
	WithinFree       bool
}

// BuildForecast soma ao saldo de hoje o salário que cai até o mergulho e tira as faturas, parcelas e o gasto de costume.
// Depois do mergulho, a primeira parte do salário que deixa a conta positiva é a saída do limite.
func BuildForecast(in ForecastInput) Forecast {
	f := Forecast{Date: in.Dip, Until: in.Until, Balance: in.Balance}
	if f.Until.Before(in.Dip) {
		f.Until = in.Dip
	}
	f.Committed = in.Balance
	add := func(l Line) {
		f.Lines = append(f.Lines, l)
		f.Balance += l.Amount
		if !l.Estimate {
			f.Committed += l.Amount
		}
	}
	var after []Line
	for _, s := range in.Salaries {
		if s.Date != nil && s.Date.After(in.Dip) {
			after = append(after, s)
		} else {
			add(s)
		}
	}
	for _, b := range in.Bills {
		add(b)
	}
	if in.Usual != nil && in.Usual.Amount != 0 {
		add(*in.Usual)
	}
	f.Balance, f.Committed = round(f.Balance), round(f.Committed)
	f.InLimit, f.CommittedInLimit = round(max(0, -f.Balance)), round(max(0, -f.Committed))
	if f.InLimit == 0 {
		f.WithinFree = true
		return f
	}
	running := f.Balance
	for _, s := range after {
		running += s.Amount
		if running >= 0 {
			f.ExitDate, f.ExitLabel = s.Date, s.Label
			f.Days = int(civil(*s.Date).Sub(civil(in.Dip)).Hours() / 24)
			f.WithinFree = f.Days <= in.FreeDays
			break
		}
	}
	return f
}

// NextDip é o próximo dia dueDay a partir de hoje (hoje conta).
func NextDip(today time.Time, dueDay int) time.Time {
	today = civil(today)
	d := domain.DebtDueIn(today, dueDay)
	if d.Before(today) {
		d = domain.DebtDueIn(today.AddDate(0, 1, 1-today.Day()), dueDay)
	}
	return d
}

func civil(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func round(v float64) float64 { return math.Round(v*100) / 100 }

// recurringSpread: uma conta que se repete pode variar de um mês para o outro (seguro, luz) até 35%.
const recurringSpread = 0.35

// Recurring são as movimentações do mês passado (last) que têm par no mês anterior (before): mesmo sentido, mesma
// descrição (números ignorados) e valor parecido. É o que se repete todo mês. Cada par vale uma vez, então Pix avulsos
// para a mesma pessoa (uma compra, um empréstimo) ficam de fora. Cada uma vira uma linha um mês à frente.
func Recurring(last, before []domain.MoneyMovement) []Line {
	key := func(m domain.MoneyMovement) string {
		dir := "out:"
		if m.In {
			dir = "in:"
		}
		return dir + strings.Join(strings.Fields(strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) || unicode.IsPunct(r) {
				return ' '
			}
			return r
		}, domain.Fold(m.Label))), " ")
	}
	used := make([]bool, len(before))
	var out []Line
	for _, m := range last {
		k := key(m)
		best := -1
		for n, b := range before {
			if used[n] || key(b) != k || math.Abs(b.Amount-m.Amount) > recurringSpread*max(b.Amount, m.Amount) {
				continue
			}
			if best < 0 || math.Abs(b.Amount-m.Amount) < math.Abs(before[best].Amount-m.Amount) {
				best = n
			}
		}
		if best < 0 {
			continue
		}
		used[best] = true
		amount := m.Amount
		if !m.In {
			amount = -amount
		}
		date := m.Date.AddDate(0, 1, 0)
		out = append(out, Line{Label: strings.Join(strings.Fields(m.Label), " "), Amount: round(amount), Date: &date})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.Before(*out[j].Date) })
	return out
}

// LastCycle é quanto a dívida coberta pelo limite custou no ciclo passado: o último salário caiu com a conta no buraco, e
// o que ele não deixou de sobra foi engolido. Amount = Salary − Leftover.
type LastCycle struct {
	SalaryDate time.Time
	Salary     float64
	Balance    float64 // saldo da conta agora
	Saved      float64 // guardado em outras contas e caixinhas
	Leftover   float64 // Balance + Saved
	Amount     float64
}

// BuildLastCycle usa o crédito de salário mais recente até hoje; nil sem salário recebido ou se o salário sobrou inteiro.
func BuildLastCycle(credits []domain.SalaryCredit, balance, saved float64, today time.Time) *LastCycle {
	var last *domain.SalaryCredit
	for n := range credits {
		c := credits[n]
		if c.Kind.IsExtra() {
			continue // hora extra, PLR, 13º: não é o salário que fecha o ciclo
		}
		if !civil(c.Date).After(civil(today)) && (last == nil || !c.Date.Before(last.Date)) {
			last = &c
		}
	}
	if last == nil {
		return nil
	}
	lc := &LastCycle{SalaryDate: civil(last.Date), Salary: round(last.Amount), Balance: round(balance), Saved: round(saved)}
	lc.Leftover = round(balance + saved)
	lc.Amount = round(lc.Salary - lc.Leftover)
	if lc.Amount <= 0 {
		return nil
	}
	return lc
}
