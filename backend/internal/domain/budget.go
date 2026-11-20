package domain

import "time"

// ExpenseKeyMonth soma, em um mês, as despesas de uma mesma conta (descrição normalizada em Key).
type ExpenseKeyMonth struct {
	Key         string
	Label       string // descrição de exemplo, como veio do banco
	Category    string
	Month       time.Time
	Total       float64
	Count       int
	Day         int  // dia do mês do último lançamento
	AllPaid     bool // nenhum lançamento pendente
	Installment bool // parcelada (tipo INSTALLMENT ou "n/m" na descrição)
	Recurring   bool // cadastrada como recorrente
}

// IncomePayment é um recebimento (uma entrada de renda), com a descrição normalizada em Key.
type IncomePayment struct {
	Key    string
	Label  string // descrição de exemplo, como veio do banco
	Month  time.Time
	Amount float64
}

// ExpenseClass é a classificação de uma despesa para o orçamento.
type ExpenseClass string

const (
	ClassFixed       ExpenseClass = "FIXED"
	ClassInstallment ExpenseClass = "INSTALLMENT"
	ClassVariable    ExpenseClass = "VARIABLE"
)

// CategoryMonth soma as despesas de uma categoria em um mês.
type CategoryMonth struct {
	Category string
	Month    time.Time
	Total    float64
}

// ExpensePayment é uma despesa individual, com a descrição normalizada em Key.
type ExpensePayment struct {
	Key           string
	Label         string // descrição de exemplo, como veio do banco
	Category      string
	PaymentMethod string
	Date          time.Time
	Amount        float64
}

// MonthLine é um lançamento do mês para a tela do mês (estilo planilha), já com a chave da conta e de onde veio.
type MonthLine struct {
	Key         string // descrição normalizada (a mesma chave do orçamento)
	Label       string
	Category    string
	Kind        string // EXPENSE, INCOME ou TRANSFER
	Direction   string // IN ou OUT, só em transferências
	Amount      float64
	Pending     bool
	AccountID   string // vazio em lançamento manual sem conta
	AccountName string
	Card        bool   // no cartão de crédito
	Synthetic   string // "" (real), "forecast" (parcela prevista) ou "bill" (ajuste pelo total da fatura)
	Date        time.Time
	Note        string
}
