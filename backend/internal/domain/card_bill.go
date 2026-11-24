package domain

import "time"

// DefaultCardCloseDaysBeforeDue é quantos dias antes do vencimento a fatura do cartão fecha, quando o banco não informa o
// fechamento (o Open Finance raramente informa).
const DefaultCardCloseDaysBeforeDue = 6

// cardDueIn é o vencimento da fatura no mês de month: o dia dueDay, ou o último dia nos meses mais curtos.
func cardDueIn(month time.Time, dueDay int) time.Time {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return time.Date(first.Year(), first.Month(), min(max(dueDay, 1), last), 0, 0, 0, 0, time.UTC)
}

// CardBillDue devolve o vencimento da fatura em que cai uma compra à vista no cartão feita em purchase. A fatura de cada
// mês fecha closeDaysBefore dias antes do vencimento; compras no dia do fechamento ou depois vão para a fatura seguinte
// (o "melhor dia de compra" é o próprio dia do fechamento).
func CardBillDue(purchase time.Time, dueDay, closeDaysBefore int) time.Time {
	day := time.Date(purchase.Year(), purchase.Month(), purchase.Day(), 0, 0, 0, 0, time.UTC)
	for i := 0; ; i++ {
		due := cardDueIn(day.AddDate(0, i, 1-day.Day()), dueDay)
		if day.Before(due.AddDate(0, 0, -closeDaysBefore)) {
			return due
		}
	}
}

// CardInstallmentBillDue devolve o vencimento da fatura de uma parcela: a sincronização já data cada parcela no mês da
// fatura informado pelo banco (billForecastDate), então é o vencimento nesse mesmo mês.
func CardInstallmentBillDue(installmentDate time.Time, dueDay int) time.Time {
	return cardDueIn(installmentDate, dueDay)
}

// CardCloseEstimate é o fechamento estimado da fatura que vence em due.
func CardCloseEstimate(due time.Time, closeDaysBefore int) time.Time {
	return due.AddDate(0, 0, -closeDaysBefore)
}

// ReplacementDates devolve os dias em que entra o lançamento que substitui uma série de parcelas no cartão: um por mês,
// do mês de fromBill até o da última parcela (firstBill + installments-1 meses), no dia day (o último dia nos meses mais
// curtos). Só os que já chegaram (até today): os meses seguintes não aparecem antes da hora.
func ReplacementDates(firstBill, fromBill, today time.Time, installments, day int) []time.Time {
	month := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC) }
	last := month(firstBill).AddDate(0, installments-1, 0)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	var out []time.Time
	for m := month(fromBill); !m.After(last); m = m.AddDate(0, 1, 0) {
		d := cardDueIn(m, day)
		if d.After(today) {
			break
		}
		out = append(out, d)
	}
	return out
}

// CardInstallmentNotBeforeFirst corrige a fatura de uma parcela n (n > 1) que cairia antes da parcela 1 da mesma série:
// impossível, e sinal de data errada vinda do banco (ex.: parcela futura com a data da compra e sem previsão de fatura).
// Quando nenhuma parcela 1 da série (firstBills, faturas em ordem crescente) vence no mês de want ou antes, a parcela vai
// para n-1 meses depois da fatura da parcela 1 mais antiga. Com uma parcela 1 até want (série anterior, ou a própria
// série em ordem), want fica como está.
func CardInstallmentNotBeforeFirst(want time.Time, firstBills []time.Time, n, dueDay int) time.Time {
	if n <= 1 || len(firstBills) == 0 {
		return want
	}
	month := func(t time.Time) time.Time {
		t = t.UTC()
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	for _, f := range firstBills {
		if !month(f).After(month(want)) {
			return want
		}
	}
	return cardDueIn(month(firstBills[0]).AddDate(0, n-1, 0), dueDay)
}
