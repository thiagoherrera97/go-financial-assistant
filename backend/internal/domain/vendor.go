package domain

import (
	"time"

	"github.com/google/uuid"
)

// VendorPlan é o plano de um evento com fornecedores (ex.: casamento): o dia do aporte mensal e onde fica o
// dinheiro guardado para ele (uma caixinha da conta corrente ou um valor informado à mão).
type VendorPlan struct {
	Name            string
	ContributionDay int    // dia do mês em que se guarda dinheiro (1 a 28)
	SavingsBox      string // nome da caixinha; vazio = usa SavedManual
	SavedManual     float64
}

// Vendor é um fornecedor contratado.
type Vendor struct {
	ID         uuid.UUID
	Name       string
	Contracted float64
	DueDate    *time.Time // último dia para quitar; nil = sem prazo
	CreatedAt  time.Time
	Payments   []VendorPayment // do mais antigo para o mais novo
}

// VendorPayment é um pagamento a um fornecedor, à mão ou ligado a uma transação do extrato (TransactionID).
type VendorPayment struct {
	ID            uuid.UUID
	VendorID      uuid.UUID
	Amount        float64
	PaidOn        time.Time
	TransactionID *uuid.UUID
	Description   string // descrição da transação ligada; vazia no lançamento à mão
}

// Paid soma os pagamentos do fornecedor.
func (v Vendor) Paid() float64 {
	var t float64
	for _, p := range v.Payments {
		t += p.Amount
	}
	return t
}
