package domain

import (
	"time"

	"github.com/google/uuid"
)

// DebtKind diferencia as dívidas pessoais.
type DebtKind string

const (
	// DebtOverdraft é um buraco coberto pelo cheque especial de uma conta até o salário cair. O saldo negativo dessa conta
	// aparece como a dívida, e não como "conta no negativo".
	DebtOverdraft DebtKind = "OVERDRAFT"
	// DebtInstallments é uma dívida paga em parcelas fixas todo mês.
	DebtInstallments DebtKind = "INSTALLMENTS"
)

// Debt é uma dívida pessoal (ex.: com a família). Cada tipo usa só os seus campos.
type Debt struct {
	ID             uuid.UUID
	Kind           DebtKind
	Name           string
	Creditor       string // com quem (ex.: Mãe)
	Note           string // para que foi (ex.: compra do carro)
	OriginalAmount float64
	// DueDay: em INSTALLMENTS, o dia da parcela; em OVERDRAFT, o dia em que a conta costuma entrar no limite.
	DueDay    int
	AccountID *uuid.UUID // OVERDRAFT: a conta cujo limite cobre a dívida
	FreeDays  int        // OVERDRAFT: dias no limite sem juros que o banco dá

	InstallmentAmount float64   // INSTALLMENTS
	Installments      int       // INSTALLMENTS
	FirstDue          time.Time // INSTALLMENTS: vencimento da primeira parcela
	PayeeMatch        string    // INSTALLMENTS: trecho do nome no Pix que paga a parcela (ex.: nome da mãe)
	PaidManual        int       // INSTALLMENTS: parcelas marcadas como pagas à mão
	CreatedAt         time.Time
}

// BalanceDay é o menor e o último saldo de uma conta corrente num dia.
type BalanceDay struct {
	Day  time.Time
	Min  float64
	Last float64
}

// MoneyMovement é uma entrada ou saída de uma conta: data, valor (positivo) e descrição.
type MoneyMovement struct {
	Date   time.Time
	Amount float64
	Label  string
}

// DebtDueIn é o vencimento da dívida no mês de month: o dia DueDay, ou o último dia nos meses mais curtos.
func DebtDueIn(month time.Time, dueDay int) time.Time { return cardDueIn(month, dueDay) }
