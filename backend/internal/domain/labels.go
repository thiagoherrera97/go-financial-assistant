package domain

// IsKnown diz se c é uma das categorias do app (as constantes Category*).
func (c Category) IsKnown() bool {
	switch c {
	case CategoryFood, CategoryTransport, CategoryHealth, CategoryEntertainment, CategoryShopping, CategoryMarket,
		CategoryInvestment, CategorySalary, CategoryHousing, CategoryBills, CategoryEducation, CategoryPeople, CategoryOther:
		return true
	}
	return false
}

func (c Category) Label() string {
	switch c {
	case CategoryFood:
		return "Alimentação"
	case CategoryTransport:
		return "Transporte"
	case CategoryHealth:
		return "Saúde"
	case CategoryEntertainment:
		return "Lazer"
	case CategoryShopping:
		return "Compras"
	case CategoryMarket:
		return "Mercado"
	case CategoryInvestment:
		return "Investimento"
	case CategorySalary:
		return "Salário/Renda"
	case CategoryHousing:
		return "Moradia"
	case CategoryBills:
		return "Contas"
	case CategoryEducation:
		return "Educação"
	case CategoryPeople:
		return "Pessoas"
	default:
		return "Outros"
	}
}

func (p PaymentMethod) Label() string {
	switch p {
	case PaymentMethodCash:
		return "Dinheiro"
	case PaymentMethodCreditCard:
		return "Cartão de Crédito"
	case PaymentMethodDebitCard:
		return "Cartão de Débito"
	case PaymentMethodPix:
		return "Pix"
	default:
		return "Outro"
	}
}
