package pluggy

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// A categoria do Pluggy chega como texto em inglês (ex.: "Groceries", "Same person
// transfer - PIX"). Casamos por trechos, em ordem; a primeira regra que bater vence.
// Categorias novas ou não mapeadas caem em OTHER sem quebrar nada.
var (
	// Não são gasto nem renda: a fatura já é contada pelas compras do cartão, e
	// transferência entre contas do mesmo titular só move dinheiro de lugar.
	ignoredCategories = []string{"same person transfer"}

	// O Pluggy classifica como "credit card payment" todo boleto pago e até Pix QR Code de parcelamento, não só pagamento de
	// fatura (ex.: "Pagamento de boleto BANCO C6 S.A." era a parcela de um financiamento e sumia do app). Só é pagamento de
	// fatura, e portanto ignorado, o que na descrição se parece com um: fatura, cartão, bandeira, pagamento recebido.
	cardPaymentCategory = "credit card payment"
	cardPaymentKeywords = []string{"fatura", "cartao", "cartão", "pagamento recebido", "pagamento com saldo", " mc", "visa", "mastercard", "elo ", "amex", "hipercard"}

	// Aplicação (saída) e resgate (entrada) de investimentos viram TRANSFER.
	investmentCategories = []string{"investment", "fixed income", "variable income", "mutual fund"}

	// Aplicação e resgate automáticos do Itaú ("APLIC AUT MAIS") só varrem o saldo da conta
	// para um CDB e de volta: não são aportes seus e inflariam o fluxo de investimentos.
	// Os rendimentos pagos por essa aplicação são renda e continuam entrando.
	autoSweepDescriptions = []string{"aplic aut mais"}

	// Quando o Pluggy não classifica a despesa (OTHER), a descrição decide. Vale a primeira regra
	// que bater; nomes de pessoas e o que não for óbvio ficam em OTHER de propósito.
	descriptionRules = []struct {
		keywords []string
		category domain.Category
	}{
		{[]string{"ifood", "99 food", "rappi", "uber eats"}, domain.CategoryFood},
		{[]string{"supermercado", "atacadao", "assai"}, domain.CategoryMarket},
		{[]string{"auto posto", " posto ", "combustivel", "uber", "99 pop", "sem parar"}, domain.CategoryTransport},
		{[]string{"rd saude", "drogaria", "drogasil", "farmacia", "pague menos", "santa casa", "clinica", "clínica", "hospital"}, domain.CategoryHealth},
		{[]string{"kalunga", "magalupay", "magazine luiza", "mercadolivre", "mercado livre"}, domain.CategoryShopping},
		{[]string{"anthropic", "hostinger", "ionos", "spotify", "netflix", "apple.com"}, domain.CategoryEntertainment},
	}

	// Receitas cuja descrição indica salário (o Pluggy nem sempre classifica como "salary").
	salaryDescriptions = []string{"salário", "salario", "folha de pagamento"}

	// Categorias do Pluggy que casam pela categoria INTEIRA (minúscula, sem espaços nas pontas), e não por trecho:
	// "services" como trecho pegaria "Digital services" (Lazer) e qualquer outra "... services". Vale antes das
	// regras por trecho abaixo. É a única lista: a reclassificação do que já está em Outros usa o mesmo mapa.
	exactExpenseCategories = map[string]domain.Category{
		"housing":                  domain.CategoryHousing,
		"utilities":                domain.CategoryBills,
		"telecommunications":       domain.CategoryBills,
		"loans and financing":      domain.CategoryBills,
		"services":                 domain.CategoryBills,
		"bookstore":                domain.CategoryEducation,
		"gyms and fitness centers": domain.CategoryHealth,
	}

	expenseCategories = []struct {
		keywords []string
		category domain.Category
	}{
		{[]string{"groceries", "supermarket"}, domain.CategoryMarket},
		{[]string{"food", "restaurant", "eating", "delivery", "bakery"}, domain.CategoryFood},
		{[]string{"transport", "automotive", "fuel", "taxi", "ride", "parking", "vehicle"}, domain.CategoryTransport},
		{[]string{"health", "pharmacy", "medical", "dental"}, domain.CategoryHealth},
		{[]string{"leisure", "entertainment", "digital services", "travel", "streaming", "gaming"}, domain.CategoryEntertainment},
		{[]string{"shopping", "clothing", "electronics"}, domain.CategoryShopping},
	}
)

// CategoryFromPluggy devolve a categoria do app para a categoria de despesa do Pluggy (sem olhar a descrição).
// Categoria não mapeada devolve OTHER.
func CategoryFromPluggy(pluggyCategory string) domain.Category {
	category := strings.ToLower(strings.TrimSpace(pluggyCategory))
	if c, ok := exactExpenseCategories[category]; ok {
		return c
	}
	for _, rule := range expenseCategories {
		if containsAny(category, rule.keywords) {
			return rule.category
		}
	}
	return domain.CategoryOther
}

// ExactExpenseCategories devolve uma cópia do mapa por igualdade (chave em minúsculas), para reclassificar o que
// já foi gravado em Outros.
func ExactExpenseCategories() map[string]domain.Category {
	m := make(map[string]domain.Category, len(exactExpenseCategories))
	for k, v := range exactExpenseCategories {
		m[k] = v
	}
	return m
}

func containsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// toExternal converte a transação do Pluggy para o vocabulário do app. Devolve false
// quando ela não deve virar lançamento. O sentido (entrada/saída) vem do campo "type",
// não do sinal do valor, porque o sinal é invertido nos cartões de crédito.
func toExternal(accountType domain.AccountType, t transaction) (ports.ExternalTransaction, bool) {
	date, err := time.Parse(time.RFC3339, t.Date)
	amount := math.Abs(t.Amount)
	if err != nil || amount == 0 || t.ID == "" {
		return ports.ExternalTransaction{}, false
	}
	bankTime := bankMoment(date, accountType == domain.AccountCredit)
	date = bankDay(date, bankTime)

	category := strings.ToLower(t.Category)
	isCard := accountType == domain.AccountCredit
	inflow := t.Type == "CREDIT"

	// Em cartão, crédito é pagamento de fatura (ignorado: a fatura já conta pelas compras) ou estorno, que não é renda: entra
	// como despesa negativa e abate a fatura, como no extrato do banco. O Pluggy marca o pagamento com "credit card payment";
	// sem a categoria, a descrição de pagamento basta.
	refund := isCard && inflow
	if containsAny(category, ignoredCategories) || (refund && isCardBillPayment(category, t.Description+" "+t.DescriptionRaw)) ||
		isCardPayment(category, t.Description+" "+t.DescriptionRaw) {
		return ports.ExternalTransaction{}, false
	}
	if refund {
		inflow = false
	}

	description := t.Description
	if description == "" {
		description = t.DescriptionRaw
	}
	installment := false
	if m := t.CreditCardMetadata; m != nil && m.TotalInstallments > 1 {
		description = fmt.Sprintf("%s (%d/%d)", description, m.InstallmentNumber, m.TotalInstallments)
		installment = true
		bankTime = nil // a hora é da compra original, meses antes: ao lado do dia da parcela, só confundiria
		date = installmentDate(date, m.BillForecastDate)
	}

	ext := ports.ExternalTransaction{
		ID:            t.ID,
		Date:          date,
		BankTime:      bankTime,
		Description:   description,
		RawInput:      fmt.Sprintf("[open finance: %s | %s]", t.Category, t.DescriptionRaw),
		Amount:        amount,
		Category:      domain.CategoryOther,
		PaymentMethod: paymentMethod(isCard, t),
		Pending:       t.Status == "PENDING",
		Installment:   installment,
		Refund:        refund,
	}
	if t.CreditCardMetadata != nil {
		ext.BillID = t.CreditCardMetadata.BillID
	}

	switch {
	case containsAny(category, investmentCategories) && !refund:
		if containsAny(strings.ToLower(description), autoSweepDescriptions) {
			return ports.ExternalTransaction{}, false
		}
		ext.Kind, ext.Category = domain.KindTransfer, domain.CategoryInvestment
		ext.Direction = domain.TransferDirectionOut
		if inflow {
			ext.Direction = domain.TransferDirectionIn
		}
	case inflow:
		ext.Kind = domain.KindIncome
		if strings.Contains(category, "salary") || containsAny(strings.ToLower(description), salaryDescriptions) {
			ext.Category = domain.CategorySalary
		}
	default:
		ext.Kind = domain.KindExpense
		ext.Category = CategoryFromPluggy(t.Category)
		if ext.Category == domain.CategoryOther {
			ext.Category = categoryFromDescription(description + " " + t.DescriptionRaw)
		}
	}
	return ext, true
}

// bankMoment devolve o instante em que o banco registrou a transação, ou nil quando ele só mandou o dia (meia-noite exata
// em UTC ou em Brasília). Na conta o Pluggy manda o instante em UTC; no cartão, manda o horário de Brasília marcado como
// UTC (compra às 00:26 no app do Santander chega como "00:26:33Z"), então o relógio é lido como horário de São Paulo.
func bankMoment(t time.Time, card bool) *time.Time {
	u := t.UTC()
	if u.Minute() == 0 && u.Second() == 0 && u.Nanosecond() == 0 && (u.Hour() == 0 || u.Hour() == 3) {
		return nil
	}
	if card {
		u = time.Date(u.Year(), u.Month(), u.Day(), u.Hour(), u.Minute(), u.Second(), u.Nanosecond(), saoPaulo).UTC()
	}
	return &u
}

// saoPaulo é o fuso do dia e da hora das transações: o Open Finance é brasileiro.
var saoPaulo = mustLocation("America/Sao_Paulo")

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// bankDay é o dia civil da transação (meia-noite UTC, como o resto do app guarda datas). Com hora, é o dia em São Paulo:
// um Pix às 21h54 de 13/11 chega como 00:54 UTC de 14/11 e deve contar no dia 13. Sem hora (só o dia), vale a data enviada.
func bankDay(t time.Time, moment *time.Time) time.Time {
	if moment != nil {
		t = moment.In(saoPaulo)
	} else {
		t = t.UTC()
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// installmentDate leva a parcela para o mês da fatura em que ela cai (billForecastDate, "AAAA-MM"), mantendo o dia da
// compra. O Pluggy datava todas as parcelas no dia da compra: o gasto de 12 meses caía inteiro no mês da compra e os
// meses seguintes ficavam sem a parcela. Sem a previsão da fatura, a data da compra fica como está.
func installmentDate(purchase time.Time, forecast string) time.Time {
	m, err := time.Parse("2006-01", forecast)
	if err != nil {
		return purchase
	}
	last := time.Date(m.Year(), m.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(m.Year(), m.Month(), min(purchase.Day(), last), 0, 0, 0, 0, time.UTC)
}

// isCardBillPayment diz se um crédito no cartão é o pagamento da fatura (e não um estorno).
func isCardBillPayment(category, description string) bool {
	return strings.Contains(category, cardPaymentCategory) || strings.Contains(strings.ToLower(description), "pagamento")
}

// isCardPayment diz se a transação é pagamento de fatura de cartão: categoria "credit card payment" do Pluggy e a descrição
// com cara de fatura. Boleto para um banco (financiamento) e Pix QR Code com essa categoria são gasto de verdade.
func isCardPayment(category, description string) bool {
	if !strings.Contains(category, cardPaymentCategory) {
		return false
	}
	d := " " + strings.ToLower(description) + " "
	if strings.Contains(d, "pix") {
		return false
	}
	return containsAny(d, cardPaymentKeywords)
}

// categoryFromDescription aplica descriptionRules; sem correspondência devolve OTHER.
func categoryFromDescription(text string) domain.Category {
	text = " " + strings.ToLower(text) + " "
	for _, rule := range descriptionRules {
		if containsAny(text, rule.keywords) {
			return rule.category
		}
	}
	return domain.CategoryOther
}

func paymentMethod(isCard bool, t transaction) domain.PaymentMethod {
	if isCard {
		return domain.PaymentMethodCreditCard
	}
	if t.PaymentData != nil && strings.EqualFold(t.PaymentData.PaymentMethod, "PIX") {
		return domain.PaymentMethodPix
	}
	return domain.PaymentMethodOther
}
