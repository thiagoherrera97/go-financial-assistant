package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CardInstallment é uma parcela de cartão já conhecida (vinda do banco): a descrição sem o "(n/m)", a parcela, o total
// de parcelas, o valor e o vencimento da fatura em que caiu.
type CardInstallment struct {
	AccountID uuid.UUID
	Base      string
	Number    int
	Total     int
	Amount    float64
	Category  Category
	BillDue   time.Time
}

// ForecastInstallment é uma parcela prevista: ainda não chegou do banco, mas a parcela anterior da mesma compra diz que
// ela cai na fatura BillDue com o mesmo valor. Key é estável (mesma compra, mesma parcela, mesma cópia) para a parcela
// prevista ser reconhecida entre sincronizações.
type ForecastInstallment struct {
	Key       string
	AccountID uuid.UUID
	Base      string
	Number    int
	Total     int
	Amount    float64
	Category  Category
	BillDue   time.Time
}

// Description é como a parcela prevista aparece: igual à do banco, "BASE (n/m)".
func (f ForecastInstallment) Description() string {
	return fmt.Sprintf("%s (%d/%d)", f.Base, f.Number, f.Total)
}

func monthIndex(t time.Time) int { return t.Year()*12 + int(t.Month()) - 1 }

// MonthsBetween é quantos meses de calendário vão de a até b (negativo se b vem antes).
func MonthsBetween(a, b time.Time) int { return monthIndex(b) - monthIndex(a) }

// CardForecastMonthsAhead é até quantos meses à frente de hoje as parcelas de cartão são previstas.
const CardForecastMonthsAhead = 3

// CardForecastHorizon é a última fatura com parcelas previstas: a que vence CardForecastMonthsAhead meses depois do mês
// de today.
func CardForecastHorizon(today time.Time, dueDay int) time.Time {
	return cardDueIn(time.Date(today.Year(), today.Month()+CardForecastMonthsAhead, 1, 0, 0, 0, 0, time.UTC), dueDay)
}

// ForecastCardInstallments prevê as parcelas que faltam chegar do banco. Cada compra parcelada é reconhecida pela conta,
// descrição, número de parcelas, valor e mês em que começou (fatura menos a parcela, que é constante na mesma compra);
// compras iguais no mesmo mês contam como cópias. O banco entrega as parcelas com atraso, então só se prevê depois da
// última fatura em que chegou alguma parcela daquele cartão, e só para as compras que tinham parcela nela (as que pararam
// antes terminaram ou mudaram de nome no banco). A partir da maior parcela conhecida, prevê as seguintes, uma por fatura,
// até a fatura horizon[conta] (inclusive). dueDay é o dia do vencimento de cada cartão.
func ForecastCardInstallments(known []CardInstallment, dueDay map[uuid.UUID]int, horizon map[uuid.UUID]time.Time) []ForecastInstallment {
	type chainKey struct {
		account uuid.UUID
		base    string
		total   int
		cents   int64
		start   int
	}
	type chain struct {
		sample CardInstallment
		maxK   int
		copies map[int]int // parcela -> quantas compras iguais
		bill   time.Time   // fatura da maior parcela
	}
	chains := map[chainKey]*chain{}
	var order []chainKey
	for _, k := range known {
		if k.Total < 2 || k.Number < 1 || k.Number > k.Total || k.Amount <= 0 {
			continue
		}
		key := chainKey{k.AccountID, strings.ToLower(strings.TrimSpace(k.Base)), k.Total, int64(math.Round(k.Amount * 100)), monthIndex(k.BillDue) - k.Number}
		c := chains[key]
		if c == nil {
			c = &chain{sample: k, copies: map[int]int{}}
			chains[key] = c
			order = append(order, key)
		}
		c.copies[k.Number]++
		if k.Number > c.maxK {
			c.maxK, c.bill = k.Number, k.BillDue
		}
	}

	lastKnown := map[uuid.UUID]time.Time{}
	for _, k := range known {
		if k.BillDue.After(lastKnown[k.AccountID]) {
			lastKnown[k.AccountID] = k.BillDue
		}
	}

	var out []ForecastInstallment
	for _, key := range order {
		c := chains[key]
		if monthIndex(c.bill) != monthIndex(lastKnown[key.account]) {
			continue
		}
		day, ok := dueDay[key.account]
		limit, hasLimit := horizon[key.account]
		if !ok || !hasLimit {
			continue
		}
		copies := c.copies[c.maxK]
		for j := c.maxK + 1; j <= key.total; j++ {
			bill := cardDueIn(c.bill.AddDate(0, j-c.maxK, 1-c.bill.Day()), day)
			if bill.After(limit) {
				break
			}
			for n := 1; n <= copies; n++ {
				out = append(out, ForecastInstallment{
					Key:       fmt.Sprintf("%s:%s:%d:%d:%d:%d:%d", key.account, key.base, key.total, key.cents, key.start, j, n),
					AccountID: key.account, Base: strings.TrimSpace(c.sample.Base), Number: j, Total: key.total,
					Amount: c.sample.Amount, Category: c.sample.Category, BillDue: bill,
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].BillDue.Before(out[j].BillDue) })
	return out
}

// CardBillGapTolerance é a diferença mínima entre o total da fatura do banco e o que o app conhece para lançar o ajuste.
const CardBillGapTolerance = 0.5

// CardBillGap é a diferença entre o total da fatura do banco e o que o app conhece: positiva quando falta detalhe (compra ou
// parcela que o Open Finance não mandou, saldo que veio da fatura anterior), negativa quando o app passa do banco (crédito
// que não veio). Zero abaixo da tolerância.
func CardBillGap(billTotal, known float64) float64 {
	gap := math.Round((billTotal-known)*100) / 100
	if math.Abs(gap) < CardBillGapTolerance {
		return 0
	}
	return gap
}
