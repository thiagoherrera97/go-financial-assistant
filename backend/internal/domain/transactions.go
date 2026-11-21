package domain

import (
	"time"

	"github.com/google/uuid"
)

type TransactionFilter struct {
	Month         *time.Time // primeiro dia do mês; nil = todos os meses
	Kind          string
	ExcludeKind   string // tira um tipo (ex.: INCOME, para os grupos de categoria só com gastos)
	Category      string
	Subcategory   string // código da subcategoria; "none" = sem subcategoria
	Essential     string // "true", "false" ou "unknown" (a revisar); vazio = todos
	PaymentMethod string
	AccountID     *uuid.UUID
	Day           *time.Time // um dia específico (sobrepõe Month)
	Search        string     // trecho da descrição ou da nota
	Limit         int
	Offset        int
}

type GroupBy string

const (
	GroupByCategory GroupBy = "category"
	GroupByDay      GroupBy = "day"
)

// IncomeGroupKey é a chave do grupo que junta todas as receitas no agrupamento por categoria: receita não se mistura
// com os gastos de uma categoria (ex.: Outros fica só com gastos sem categoria).
const IncomeGroupKey = "INCOME"

// TransactionGroup soma as transações de um grupo (categoria ou dia) sob os mesmos filtros da lista.
// Key é o valor do enum (categoria) ou a data AAAA-MM-DD.
type TransactionGroup struct {
	Key      string
	Count    int
	Expense  float64
	Income   float64
	Transfer float64 // aplicações + resgates (volume movimentado)
	// Aplicado (saiu para o investimento ou a caixinha) e resgatado (voltou). O que ficou guardado é Applied − Redeemed.
	Applied, Redeemed float64
}

type Transaction struct {
	ID                uuid.UUID
	Date              time.Time
	Description       string
	Category          string
	Subcategory       string // "" quando não tem (ou ainda não foi classificada)
	Essential         *bool  // nil = a revisar
	PaymentMethod     string
	Kind              string
	TransferDirection string
	Type              string
	Status            string
	Amount            float64
	InstallmentNumber *int
	AccountID         *uuid.UUID
	AccountName       string
	FromOpenFinance   bool
	Note              string     // escrita pela pessoa; vazia sem nota
	BankTime          *time.Time // instante (com hora) informado pelo banco; nil se não houver
}

// TransferCandidate é um lançamento do banco que parece Pix, TED ou DOC.
type TransferCandidate struct {
	PaymentID   uuid.UUID
	Description string
	Kind        string // EXPENSE ou INCOME
	Amount      float64
}
