package domain

import "time"

// UncategorizedGroup soma as despesas em "Outros" de uma mesma conta (descrição normalizada em Key).
type UncategorizedGroup struct {
	Key   string
	Label string // descrição de exemplo, como veio do banco
	Count int
	Total float64
	Last  time.Time
}

// CategoryRule é a categoria escolhida para uma conta; OTHER significa "manter em Outros".
type CategoryRule struct {
	Key      string
	Category string
}

// Classification é a correção de uma despesa feita por você: categoria, subcategoria ("" = o app escolhe) e essencial
// (nil = vale o da subcategoria). ApplyToAll grava a regra do estabelecimento (mesma descrição normalizada) e corrige as
// outras despesas dele, inclusive as que vierem.
type Classification struct {
	Category    Category
	Subcategory Subcategory
	Essential   *bool
	ApplyToAll  bool
}

// SubcategorySpend soma as despesas de um mês por subcategoria e por essencial/supérfluo (o mesmo estabelecimento pode ter
// gastos marcados à mão de um jeito e de outro). PrevAverage é a média por mês dos três meses anteriores, no mesmo grupo.
type SubcategorySpend struct {
	Category    Category
	Subcategory Subcategory // "" em categorias sem subcategoria (Pessoas, Outros)
	Essential   *bool       // nil = a revisar
	Count       int
	Total       float64
	PrevAverage float64
	// Installments é a parte de Total que é parcela de compra já feita: cortar o gasto não a tira dos próximos meses.
	Installments float64
}
