package domain

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SalaryPart é uma parte do salário: o dia combinado do mês e o valor líquido esperado.
type SalaryPart struct {
	Day    int
	Amount float64
}

// SalaryMatchDays é a distância máxima, em dias, entre a data prevista e o crédito real para contar como a mesma parte.
const SalaryMatchDays = 5

// ParseSalarySchedule lê "15=5200; 30=4366,62": partes separadas por ponto e vírgula, cada uma "dia=valor". O valor aceita
// vírgula ou ponto como decimal. Texto vazio é "sem salário configurado". As partes voltam em ordem de dia.
func ParseSalarySchedule(raw string) ([]SalaryPart, error) {
	var parts []SalaryPart
	seen := map[int]bool{}
	for _, chunk := range strings.Split(raw, ";") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		dayText, amountText, ok := strings.Cut(chunk, "=")
		if !ok {
			return nil, fmt.Errorf("%q: use dia=valor, como 15=5200", chunk)
		}
		day, err := strconv.Atoi(strings.TrimSpace(dayText))
		if err != nil || day < 1 || day > 31 {
			return nil, fmt.Errorf("%q: o dia vai de 1 a 31", chunk)
		}
		if seen[day] {
			return nil, fmt.Errorf("dia %d repetido", day)
		}
		seen[day] = true
		amountText = strings.TrimSpace(amountText)
		if strings.Contains(amountText, ",") {
			amountText = strings.ReplaceAll(strings.ReplaceAll(amountText, ".", ""), ",", ".")
		}
		amount, err := strconv.ParseFloat(amountText, 64)
		if err != nil || amount <= 0 || math.IsInf(amount, 0) || amount > 10_000_000 {
			return nil, fmt.Errorf("%q: valor inválido", chunk)
		}
		parts = append(parts, SalaryPart{Day: day, Amount: math.Round(amount*100) / 100})
	}
	if len(parts) > 6 {
		return nil, errors.New("no máximo 6 partes")
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Day < parts[j].Day })
	return parts, nil
}

// SalaryPayDate é quando a parte cai no mês: o dia combinado (limitado ao último dia do mês, então 30 em fevereiro vira o
// dia 28 ou 29) ou, se não for dia útil, o dia útil anterior.
func SalaryPayDate(year int, month time.Month, day int) time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return PreviousBusinessDay(time.Date(year, month, min(day, last), 0, 0, 0, 0, time.UTC))
}

// SalaryKind é o que a pessoa disse que um crédito de salário é. Vazio: ainda não disse, e o app decide pelo valor e pela data.
type SalaryKind string

const (
	SalaryKindMonth      SalaryKind = "MONTH"      // o salário do mês inteiro: cobre todas as partes do mês Month
	SalaryKindPart       SalaryKind = "PART"       // uma parte: a do dia Day no mês Month
	SalaryKindOvertime   SalaryKind = "OVERTIME"   // hora extra paga à parte
	SalaryKindPLR        SalaryKind = "PLR"        // participação nos lucros
	SalaryKindThirteenth SalaryKind = "THIRTEENTH" // 13º ou adiantamento do 13º
	SalaryKindVacation   SalaryKind = "VACATION"   // férias
	SalaryKindOther      SalaryKind = "OTHER"      // outro extra
)

// SalaryKinds são as respostas possíveis, na ordem em que a tela mostra.
var SalaryKinds = []SalaryKind{SalaryKindMonth, SalaryKindPart, SalaryKindOvertime, SalaryKindPLR, SalaryKindThirteenth, SalaryKindVacation, SalaryKindOther}

// Valid diz se k é uma resposta conhecida (vazio não é).
func (k SalaryKind) Valid() bool { return slices.Contains(SalaryKinds, k) }

// IsExtra diz se k é um extra: nunca conta como parte do salário.
func (k SalaryKind) IsExtra() bool { return k.Valid() && k != SalaryKindMonth && k != SalaryKindPart }

// Label é o nome da resposta na tela.
func (k SalaryKind) Label() string {
	switch k {
	case SalaryKindMonth:
		return "Salário do mês inteiro"
	case SalaryKindPart:
		return "Parte do salário"
	case SalaryKindOvertime:
		return "Hora extra"
	case SalaryKindPLR:
		return "PLR"
	case SalaryKindThirteenth:
		return "13º"
	case SalaryKindVacation:
		return "Férias"
	case SalaryKindOther:
		return "Outro extra"
	}
	return "Extra"
}

// SalaryCredit é um crédito de salário que chegou do banco, com a decisão da pessoa sobre ele, se houver.
type SalaryCredit struct {
	ID     string // id do pagamento
	Date   time.Time
	Amount float64
	Kind   SalaryKind // vazio = sem decisão
	Month  time.Time  // MONTH e PART: o mês (primeiro dia) do ciclo a que o crédito pertence
	Day    int        // PART: o dia da parte
}

// SalaryAmountTolerance é a diferença de valor que ainda conta como "o valor da parte" (10%): o líquido muda um pouco de
// um mês para outro. Fora disso o app só adivinha e pede para a pessoa conferir.
const SalaryAmountTolerance = 0.10

// salaryFits diz se amount tem o valor esperado, dentro da tolerância.
func salaryFits(amount, expected float64) bool {
	return math.Abs(amount-expected) <= expected*SalaryAmountTolerance+0.005
}

// SalaryStatus é uma parte do salário em um mês: quando deve cair, quanto, e se já caiu.
type SalaryStatus struct {
	Day      int
	Expected time.Time
	Amount   float64
	Received *SalaryCredit // nil = ainda a receber
	Guessed  bool          // Received é um palpite (valor fora do combinado): espera a pessoa conferir
}

// SalaryMonth confere as partes do mês contra os créditos de salário que chegaram, nesta ordem:
//  1. o que a pessoa decidiu: "salário do mês inteiro" cobre todas as partes do mês escolhido; "parte" cobre a do dia
//     escolhido; um extra (hora extra, PLR, 13º...) nunca cobre parte nenhuma;
//  2. cada parte fica com o crédito sem decisão mais perto da data prevista, até SalaryMatchDays de distância, que tenha
//     o valor da parte (SalaryAmountTolerance);
//  3. palpites, marcados com Guessed: um crédito com a soma das partes que faltam cobre todas (o salário cheio que veio de
//     uma vez); senão, o crédito mais perto da data de uma parte, qualquer que seja o valor (o comportamento de antes).
//
// Um crédito serve a uma regra só. Os créditos do mês que não ficaram com nenhuma parte (13º, PLR, férias, hora extra paga
// à parte) voltam em extras, por data; um crédito decidido para outro mês não é extra deste.
func SalaryMonth(parts []SalaryPart, month time.Time, credits []SalaryCredit) (status []SalaryStatus, extras []SalaryCredit) {
	out := make([]SalaryStatus, len(parts))
	for i, p := range parts {
		out[i] = SalaryStatus{Day: p.Day, Expected: SalaryPayDate(month.Year(), month.Month(), p.Day), Amount: p.Amount}
	}
	key := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	used := make([]bool, len(credits))
	give := func(i, j int, guessed bool) {
		c := credits[j]
		out[i].Received, out[i].Guessed = &c, guessed
		used[j] = true
	}

	// 1. As decisões.
	for j, c := range credits {
		if c.Kind == "" {
			continue
		}
		used[j] = true
		if c.Kind.IsExtra() || !sameMonth(c.Month, key) {
			continue
		}
		for i := range out {
			if out[i].Received == nil && (c.Kind == SalaryKindMonth || out[i].Day == c.Day) {
				give(i, j, false)
			}
		}
	}

	// 2. Data e valor batem.
	type pair struct {
		part, credit int
		dist         float64
	}
	near := func(byValue bool) []pair {
		var pairs []pair
		for i, s := range out {
			if s.Received != nil {
				continue
			}
			for j, c := range credits {
				if used[j] || byValue && !salaryFits(c.Amount, s.Amount) {
					continue
				}
				if dist := math.Abs(dayOf(c.Date).Sub(s.Expected).Hours() / 24); dist <= SalaryMatchDays {
					pairs = append(pairs, pair{i, j, dist})
				}
			}
		}
		sort.SliceStable(pairs, func(a, b int) bool { return pairs[a].dist < pairs[b].dist })
		return pairs
	}
	for _, p := range near(true) {
		if out[p.part].Received == nil && !used[p.credit] {
			give(p.part, p.credit, false)
		}
	}

	// 3. Palpites. Primeiro o salário cheio: um crédito com a soma das partes que faltam, entre a primeira e a última delas.
	var pending []int
	sum := 0.0
	for i, s := range out {
		if s.Received == nil {
			pending = append(pending, i)
			sum += s.Amount
		}
	}
	if len(pending) >= 2 {
		from := out[pending[0]].Expected.AddDate(0, 0, -SalaryMatchDays)
		to := out[pending[len(pending)-1]].Expected.AddDate(0, 0, SalaryMatchDays)
		for j, c := range credits {
			d := dayOf(c.Date)
			if used[j] || d.Before(from) || d.After(to) || !salaryFits(c.Amount, sum) {
				continue
			}
			for _, i := range pending {
				give(i, j, true)
			}
			break
		}
	}
	for _, p := range near(false) {
		if out[p.part].Received == nil && !used[p.credit] {
			give(p.part, p.credit, true)
		}
	}

	for j, c := range credits {
		if sameMonth(c.Date, key) && (c.Kind.IsExtra() || c.Kind == "" && !used[j]) {
			extras = append(extras, c)
		}
	}
	sort.SliceStable(extras, func(a, b int) bool { return extras[a].Date.Before(extras[b].Date) })
	return out, extras
}

// SalaryReview é um crédito de salário que espera a pessoa dizer o que é. Suggest é o palpite do app (vazio para um extra:
// o app não sabe dizer se é hora extra, PLR ou 13º); Month e Day completam o palpite de mês inteiro e de parte.
type SalaryReview struct {
	Credit  SalaryCredit
	Suggest SalaryKind
	Month   time.Time
	Day     int
}

// SalaryReviews são os créditos do mês sem decisão que o app não reconheceu com certeza: os que viraram parte só por
// palpite (valor fora do combinado) e os extras. Um crédito aparece uma vez só, em ordem de data.
func SalaryReviews(parts []SalaryPart, month time.Time, credits []SalaryCredit) []SalaryReview {
	status, extras := SalaryMonth(parts, month, credits)
	key := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	var out []SalaryReview
	index := map[SalaryCredit]int{}
	for _, s := range status {
		if !s.Guessed || s.Received.Kind != "" {
			continue
		}
		if n, ok := index[*s.Received]; ok {
			out[n].Suggest, out[n].Day = SalaryKindMonth, 0 // o mesmo crédito cobriu mais de uma parte
			continue
		}
		index[*s.Received] = len(out)
		out = append(out, SalaryReview{Credit: *s.Received, Suggest: SalaryKindPart, Month: key, Day: s.Day})
	}
	for _, c := range extras {
		if c.Kind == "" {
			out = append(out, SalaryReview{Credit: c, Month: key})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Credit.Date.Before(out[b].Credit.Date) })
	return out
}

func sameMonth(a, b time.Time) bool { return a.Year() == b.Year() && a.Month() == b.Month() }

// SalaryTotal soma o valor esperado das partes.
func SalaryTotal(parts []SalaryPart) float64 {
	var total float64
	for _, p := range parts {
		total += p.Amount
	}
	return total
}
