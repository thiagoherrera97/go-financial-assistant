package domain

import (
	"strings"
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
	PayeeMatch        string    // INSTALLMENTS: nomes de quem recebe no Pix/TED, separados por ";" (ex.: os nomes da mãe)
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
	In     bool // entrada (recebido); falso = saída (enviado)
}

// PayeeNames separa os nomes de PayeeMatch (";" ou quebra de linha), já normalizados com Fold; ignora os vazios.
func PayeeNames(match string) []string {
	var out []string
	for _, n := range strings.FieldsFunc(match, func(r rune) bool { return r == ';' || r == '\n' }) {
		if f := Fold(n); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// MatchesPayee diz se a descrição de uma transação traz algum dos nomes de PayeeMatch, sem diferenciar maiúsculas e
// acentos (o banco escreve o mesmo nome de jeitos diferentes: "MONICA NUNES", "Monica Nunes").
func MatchesPayee(label, match string) bool {
	l := Fold(label)
	for _, n := range PayeeNames(match) {
		if strings.Contains(l, n) {
			return true
		}
	}
	return false
}

// DebtDueIn é o vencimento da dívida no mês de month: o dia DueDay, ou o último dia nos meses mais curtos.
func DebtDueIn(month time.Time, dueDay int) time.Time { return cardDueIn(month, dueDay) }

// RecurringFlow é uma entrada ou saída que se repete todo mês fora o salário: aluguel, seguro, financiamento, a mãe
// pagando uma dívida antiga, um valor em 10x, uma ajuda que cresce R$ 100 por mês. O valor do mês é Amount + Step ×
// (meses desde StartMonth), limitado a MaxAmount (0 = sem limite); Installments (0 = sem fim) é quantos meses dura.
type RecurringFlow struct {
	ID           uuid.UUID
	In           bool // entrada; falso = saída
	Name         string
	Party        string // com quem (ex.: Mãe, irmão)
	Amount       float64
	Step         float64
	MaxAmount    float64
	Day          int
	StartMonth   time.Time // primeiro dia do mês da primeira ocorrência
	Installments int
	PayeeMatch   string // nomes no extrato (Pix, boleto) separados por ";": o que casa e tem o valor do mês conta como feito
	CreatedAt    time.Time
}
