// Package debts calcula a situação das dívidas pessoais: as parcelas pagas e as que faltam, e, na dívida coberta pelo
// cheque especial, quanto do limite está em uso, há quantos dias e a previsão do próximo mergulho.
package debts

import (
	"math"
	"sort"
	"time"

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
	Label  string
	Amount float64
	Date   *time.Time
}

// ForecastInput é o que a previsão do próximo mergulho no limite usa.
type ForecastInput struct {
	Today    time.Time
	Dip      time.Time // dia em que a conta costuma entrar no limite
	Balance  float64   // saldo da conta agora
	FreeDays int
	Salaries []Line // partes do salário a cair, em ordem de data (podem passar de Dip)
	Bills    []Line // faturas e parcelas que saem da conta até Dip (valores negativos)
	Usual    *Line  // o que saiu da conta no mesmo período do mês passado (negativo), se houver
}

// Forecast é a previsão do saldo no dia do mergulho e de quando o salário tira a conta do limite.
type Forecast struct {
	Date       time.Time
	Lines      []Line  // como se chega ao saldo previsto, a partir do saldo de hoje
	Balance    float64 // saldo previsto no dia (negativo = no limite)
	InLimit    float64 // quanto do limite vai usar (0 se não entra)
	ExitDate   *time.Time
	ExitLabel  string
	Days       int // dias no limite até o salário
	WithinFree bool
}

// BuildForecast soma ao saldo de hoje o salário que cai até o mergulho e tira as faturas, parcelas e o gasto de costume.
// Depois do mergulho, a primeira parte do salário que deixa a conta positiva é a saída do limite.
func BuildForecast(in ForecastInput) Forecast {
	f := Forecast{Date: in.Dip, Balance: in.Balance}
	add := func(l Line) {
		f.Lines = append(f.Lines, l)
		f.Balance += l.Amount
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
	f.Balance = round(f.Balance)
	f.InLimit = round(max(0, -f.Balance))
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
