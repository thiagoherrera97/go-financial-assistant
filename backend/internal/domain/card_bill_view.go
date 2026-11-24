package domain

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// CardBillOrigin diz de onde vem uma linha da fatura do cartão.
type CardBillOrigin string

const (
	CardBillOriginBank     CardBillOrigin = "BANK"     // veio do banco pelo Open Finance
	CardBillOriginManual   CardBillOrigin = "MANUAL"   // lançada à mão no cartão
	CardBillOriginForecast CardBillOrigin = "FORECAST" // parcela prevista pelo app (o banco ainda não mandou)
	CardBillOriginGap      CardBillOrigin = "GAP"      // ajuste: total do banco menos o que o app conhece
	CardBillOriginReplaced CardBillOrigin = "REPLACED" // parcela trocada por lançamento manual; o banco cobrou
)

// CardBillSummary resume uma fatura do cartão (um mês de vencimento): o total que o banco informou, quando ela já fechou,
// e o que o app soma, separado pela origem.
type CardBillSummary struct {
	Due       time.Time
	BankTotal *float64 // nil enquanto o banco não informou a fatura fechada
	Detailed  float64  // lançamentos do banco, manuais e parcelas trocadas
	Forecast  float64  // parcelas previstas pelo app
	Gap       float64  // ajuste "não detalhado no Open Finance"
	Count     int      // linhas fora o ajuste
}

// Known é o que o app conhece da fatura, sem o ajuste: o que é comparado ao total do banco.
func (b CardBillSummary) Known() float64 { return math.Round((b.Detailed+b.Forecast)*100) / 100 }

// Total é a soma de todas as linhas, com o ajuste.
func (b CardBillSummary) Total() float64 { return math.Round((b.Detailed+b.Forecast+b.Gap)*100) / 100 }

// Difference é quanto o total do banco passa do que o app conhece (negativo: o app conhece mais); 0 dentro da
// tolerância ou sem total do banco.
func (b CardBillSummary) Difference() float64 {
	if b.BankTotal == nil {
		return 0
	}
	return CardBillGap(*b.BankTotal, b.Known())
}

// CardBillState é a situação da fatura em relação a hoje.
type CardBillState string

const (
	CardBillClosed CardBillState = "CLOSED" // o banco já informou o total
	CardBillOpen   CardBillState = "OPEN"   // a próxima a vencer, ainda sem total do banco
	CardBillFuture CardBillState = "FUTURE" // depois da aberta: só parcelas
)

// CardBillStates dá a situação de cada fatura (em ordem de vencimento crescente): fechada quando o banco informou o total;
// entre as outras, a primeira que vence hoje ou depois é a aberta e as seguintes, futuras. Fatura sem total do banco que
// já venceu fica como aberta (o banco ainda não mandou).
func CardBillStates(bills []CardBillSummary, today time.Time) []CardBillState {
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	out := make([]CardBillState, len(bills))
	openSeen := false
	for i, b := range bills {
		switch {
		case b.BankTotal != nil:
			out[i] = CardBillClosed
		case !openSeen && !b.Due.Before(day):
			out[i], openSeen = CardBillOpen, true
		case openSeen:
			out[i] = CardBillFuture
		default:
			out[i] = CardBillOpen
		}
	}
	return out
}

// CardBillLine é uma linha da fatura do cartão.
type CardBillLine struct {
	ID           uuid.UUID
	Date         time.Time  // dia da transação (na parcela 2+, o dia em que a parcela foi datada)
	PurchaseDate *time.Time // dia da compra numa parcela 2+, quando se acha a parcela 1
	Description  string
	Category     Category
	Amount       float64
	Installment  *int
	Origin       CardBillOrigin
	Pending      bool
	Note         string
}
