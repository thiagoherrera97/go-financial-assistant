package planning

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// averageMonths é quantos meses (com dados) entram nas médias de fixas e variáveis e na detecção de fontes recorrentes.
const averageMonths = 3

// incomeMedianMonths é quantos meses (com dados) entram na mediana da renda total: mais meses que as médias
// de despesa, porque a renda costuma ser irregular e a mediana precisa de amostra para ignorar o mês atípico.
const incomeMedianMonths = 6

// otherIncomeLabel nomeia a parte da renda que não vem de uma fonte recorrente identificável.
const otherIncomeLabel = "Outras entradas (mediana mensal dos últimos meses)"

// Assumptions são as premissas da projeção: o que se espera de um mês comum daqui para a frente.
type Assumptions struct {
	Income        float64
	IncomeSources []IncomeSource // de onde vem a renda estimada
	Fixed         float64
	Variable      float64
	BasedOn       int // quantos meses com dados sustentam as médias (0 = sem histórico)
}

// IncomeSource é uma fonte de renda recorrente e quanto ela rende em um mês comum.
type IncomeSource struct {
	Label   string
	Monthly float64
}

type ProjectedMonth struct {
	Month       time.Time
	Fixed       float64
	Installment float64 // parcelas já conhecidas
	Variable    float64
}

type Projection struct {
	Assumptions Assumptions
	Months      []ProjectedMonth
}

var installmentRE = regexp.MustCompile(`(?i)(?:\(\s*(\d+)\s*/\s*(\d+)\s*\)|parc[a-z.]*\s*(\d+)\s*/\s*(\d+))`)

// StripInstallment tira do nome a marcação de parcela ("(2/12)", "Parc 011/012").
func StripInstallment(label string) string {
	return strings.TrimSpace(installmentRE.ReplaceAllString(label, ""))
}

// parseInstallment lê "n/m" de "(2/12)" ou "Parc 011/012": a parcela atual e o total.
func parseInstallment(label string) (current, total int, ok bool) {
	m := installmentRE.FindStringSubmatch(label)
	if m == nil {
		return 0, 0, false
	}
	a, b := m[1], m[2]
	if a == "" {
		a, b = m[3], m[4]
	}
	current, _ = strconv.Atoi(a)
	total, _ = strconv.Atoi(b)
	return current, total, total > 1 && current >= 1 && current <= total
}

// BuildProjection projeta `months` meses a partir de start (primeiro dia do mês, em geral o mês atual): médias de fixas e
// variáveis, mediana da renda e parcelas já conhecidas. As regras estão em docs/regras-de-calculo.md (Projeção).
// rows são as despesas por conta dos 12 meses completos antes de start; incomes, as entradas de renda no mesmo período.
// salary, quando configurado, são as partes do salário: entram pelo valor combinado no lugar da estimativa do salário.
func BuildProjection(rows []domain.ExpenseKeyMonth, rules map[string]domain.ExpenseClass, incomes []domain.IncomePayment,
	salary []domain.SalaryPart, knownInstallments map[time.Time]float64, start time.Time, months int) Projection {

	b := BuildBudget(rows, rules, start.AddDate(0, -12, 0), start.AddDate(0, -1, 0))

	// Médias dos últimos meses que tiveram alguma despesa.
	var a Assumptions
	var used, window []time.Time // used: base das médias de despesa; window: base da mediana da renda
	for i := len(b.Series) - 1; i >= 0 && len(window) < incomeMedianMonths; i-- {
		m := b.Series[i]
		if m.Fixed+m.Installment+m.Variable <= 0 {
			continue
		}
		window = append(window, m.Month)
		if a.BasedOn < averageMonths {
			a.BasedOn++
			a.Fixed += m.Fixed
			a.Variable += m.Variable
			used = append(used, m.Month)
		}
	}
	if a.BasedOn > 0 {
		n := float64(a.BasedOn)
		a.Fixed = a.Fixed / n
		a.Variable = typicalVariable(rows, b.Classes, used)
		a.IncomeSources, a.Income = estimateIncomeTotal(incomes, used, window)
	}
	if len(salary) > 0 {
		a.IncomeSources, a.Income = withSalary(salary, incomes, used, window)
	}

	// Parcelas que faltam: a última aparição de cada conta parcelada diz em que parcela está.
	remaining := map[time.Time]float64{}
	latest := map[string]domain.ExpenseKeyMonth{}
	for _, r := range rows {
		if b.Classes[r.Key] == domain.ClassInstallment && !r.Month.Before(latest[r.Key].Month) {
			latest[r.Key] = r
		}
	}
	for _, r := range latest {
		cur, total, ok := parseInstallment(r.Label)
		if !ok {
			continue
		}
		for k := 1; k <= total-cur; k++ {
			remaining[r.Month.AddDate(0, k, 0)] += r.Total
		}
	}

	p := Projection{Assumptions: a}
	for i := 0; i < months; i++ {
		m := start.AddDate(0, i, 0)
		p.Months = append(p.Months, ProjectedMonth{Month: m, Fixed: a.Fixed, Variable: a.Variable, Installment: remaining[m] + knownInstallments[m]})
	}
	sort.Slice(p.Months, func(i, j int) bool { return p.Months[i].Month.Before(p.Months[j].Month) })
	return p
}

// typicalVariable é o gasto variável de um mês comum: a soma, por categoria, da mediana do que ela gastou em cada mês de
// used (mês sem gasto conta como zero). Um gasto pontual enorme (a entrada de um carro, um Pix para a família) pesa
// num mês só e fica de fora, enquanto a média o espalharia por todos os meses projetados.
func typicalVariable(rows []domain.ExpenseKeyMonth, classes map[string]domain.ExpenseClass, used []time.Time) float64 {
	inUsed := map[time.Time]bool{}
	for _, m := range used {
		inUsed[m] = true
	}
	byCategory := map[string]map[time.Time]float64{}
	for _, r := range rows {
		if classes[r.Key] != domain.ClassVariable || !inUsed[r.Month] {
			continue
		}
		if byCategory[r.Category] == nil {
			byCategory[r.Category] = map[time.Time]float64{}
		}
		byCategory[r.Category][r.Month] += r.Total
	}
	var total float64
	for _, perMonth := range byCategory {
		values := make([]float64, len(used))
		for i, m := range used {
			values[i] = perMonth[m]
		}
		total += median(values)
	}
	return total
}

// estimateIncomeTotal combina as duas leituras da renda: a mediana da renda total dos meses de window e as
// fontes recorrentes dos meses de used. Vale a maior (a mediana já inclui as fontes; se um mês fraco a puxou
// para baixo, a soma das fontes recorrentes é o piso). O que as fontes não explicam aparece como "outras entradas".
func estimateIncomeTotal(payments []domain.IncomePayment, used, window []time.Time) ([]IncomeSource, float64) {
	sources := EstimateIncome(payments, used)
	var recurring float64
	for _, s := range sources {
		recurring += s.Monthly
	}
	typical := MedianMonthlyIncome(payments, window)
	if typical <= recurring {
		return sources, recurring
	}
	return append(sources, IncomeSource{Label: otherIncomeLabel, Monthly: typical - recurring}), typical
}

// salaryKey é a chave que a consulta de renda dá a toda entrada na categoria Salário.
const salaryKey = "salario"

// withSalary troca a estimativa do salário pelas partes configuradas. O resto da renda segue a mesma leitura de
// estimateIncomeTotal, só que sem os créditos de salário (que já estão nas partes). Sem histórico, vale só o salário.
func withSalary(parts []domain.SalaryPart, incomes []domain.IncomePayment, used, window []time.Time) ([]IncomeSource, float64) {
	sources := make([]IncomeSource, 0, len(parts))
	for _, p := range parts {
		sources = append(sources, IncomeSource{Label: "Salário do dia " + strconv.Itoa(p.Day), Monthly: p.Amount})
	}
	total := domain.SalaryTotal(parts)
	if len(used) == 0 {
		return sources, total
	}
	var others []domain.IncomePayment
	for _, p := range incomes {
		if p.Key != salaryKey {
			others = append(others, p)
		}
	}
	rest, restTotal := estimateIncomeTotal(others, used, window)
	return append(sources, rest...), total + restTotal
}

// MedianMonthlyIncome é a mediana da renda total mensal nos meses dados; mês sem entrada conta como zero.
func MedianMonthlyIncome(payments []domain.IncomePayment, months []time.Time) float64 {
	if len(months) == 0 {
		return 0
	}
	total := map[time.Time]float64{}
	for _, p := range payments {
		total[p.Month] += p.Amount
	}
	values := make([]float64, len(months))
	for i, m := range months {
		values[i] = total[m]
	}
	return median(values)
}

// EstimateIncome estima a renda de um mês comum olhando cada fonte (mesma descrição) nos meses dados.
// Para cada fonte recorrente, vale a mediana dos pagamentos vezes quantas vezes ela costuma cair por mês,
// então um pagamento fora do padrão (adiantamento de férias, 13º) não pesa. Fonte que aparece em menos
// meses do que o necessário (3, ou todos quando há menos de 3) é eventual e fica de fora.
func EstimateIncome(payments []domain.IncomePayment, months []time.Time) []IncomeSource {
	inWindow := map[time.Time]bool{}
	for _, m := range months {
		inWindow[m] = true
	}
	type source struct {
		label    string
		amounts  []float64
		perMonth map[time.Time]int
	}
	sources := map[string]*source{}
	for _, p := range payments {
		if !inWindow[p.Month] {
			continue
		}
		s := sources[p.Key]
		if s == nil {
			s = &source{label: p.Label, perMonth: map[time.Time]int{}}
			sources[p.Key] = s
		}
		s.amounts = append(s.amounts, p.Amount)
		s.perMonth[p.Month]++
	}

	needed := max(1, min(maxFixedMonthsNeeded, len(months)))
	var result []IncomeSource
	for _, s := range sources {
		if len(s.perMonth) < needed {
			continue
		}
		counts := make([]float64, 0, len(s.perMonth))
		for _, c := range s.perMonth {
			counts = append(counts, float64(c))
		}
		sort.Float64s(counts)
		perMonth := counts[(len(counts)-1)/2] // mediana inferior: na dúvida, o menos otimista
		result = append(result, IncomeSource{Label: s.label, Monthly: median(s.amounts) * perMonth})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Monthly > result[j].Monthly })
	return result
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
