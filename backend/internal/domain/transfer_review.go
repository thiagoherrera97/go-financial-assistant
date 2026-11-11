package domain

import (
	"time"

	"github.com/google/uuid"
)

// TransferDecision é o que a pessoa decidiu sobre uma transferência vinda do banco.
type TransferDecision string

const (
	TransferOwn     TransferDecision = "OWN"     // entre contas suas: deixa de contar (CANCELLED)
	TransferPerson  TransferDecision = "PERSON"  // para uma pessoa: categoria PEOPLE
	TransferPayment TransferDecision = "PAYMENT" // pagamento: a categoria escolhida
)

// AppliedCategory é a categoria que a decisão grava no lançamento (chosen é a escolhida em PAYMENT). OWN não muda a
// categoria e devolve "". Desfazer só restaura a categoria anterior se a atual ainda é esta.
func (d TransferDecision) AppliedCategory(chosen string) string {
	switch d {
	case TransferPerson:
		return string(CategoryPeople)
	case TransferPayment:
		return chosen
	default:
		return ""
	}
}

// TransferReviewItem é uma transferência para revisar (sem decisão) ou já decidida.
type TransferReviewItem struct {
	PaymentID   uuid.UUID
	Date        time.Time
	Amount      float64
	Kind        string // EXPENSE (saída) ou INCOME (entrada)
	Account     string
	Description string
	Category    string
	Decision    TransferDecision // só nas decididas
	DecidedAt   *time.Time       // só nas decididas
}
