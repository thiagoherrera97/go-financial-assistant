package planning

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// SheetItem é uma linha da tela do mês; Lines são os lançamentos que ela soma (para abrir e ver as notas).
type SheetItem struct {
	Label   string
	Amount  float64
	Count   int
	Pending bool // tem lançamento pendente (ou é previsão)
	Lines   []SheetLine
}

// SheetLine é um lançamento dentro de uma linha da tela do mês.
type SheetLine struct {
	Label  string
	Date   time.Time
	Amount float64
	Note   string
}

// SheetCard é a fatura de um cartão no mês, por categoria.
type SheetCard struct {
	Name       string
	Total      float64
	Categories []SheetItem // compras reais menos estornos, por categoria
	Forecast   float64     // parcelas previstas (ainda não entregues pelo banco)
	Adjustment float64     // ajuste pelo total da fatura do banco (positivo ou negativo)
	Refunds    float64     // estornos já abatidos nas categorias (valor negativo)
}

// SheetPending é uma parte do salário que ainda não caiu.
type SheetPending struct {
	Label  string
	Date   time.Time
	Amount float64
}

// MonthSheet é o mês no formato da planilha: entradas, despesas fixas e outras fora do cartão, a fatura de cada cartão
// por categoria e a sobra. As regras estão em docs/regras-de-calculo.md (Tela do mês).
type MonthSheet struct {
	Month               time.Time
	Income              []SheetItem
	IncomeTotal         float64
	Pending             []SheetPending // salário a receber no mês
	PendingTotal        float64
	Fixed               []SheetItem // fora do cartão: contas fixas e parcelas
	FixedTotal          float64
	Other               []SheetItem // fora do cartão: o resto, por categoria (o que está em Outros, pelo nome)
	OtherTotal          float64
	Cards               []SheetCard
	CardsTotal          float64
	ExpensesTotal       float64
	Leftover            float64 // entradas − despesas, com o que já caiu
	LeftoverWithPending float64 // contando o salário a receber
	Invested            float64 // aplicado − resgatado no mês (fora da sobra)
}

const salaryLabel = "Salário"

// BuildMonthSheet monta a tela do mês. classes é a classe (fixa, parcelada, variável) de cada conta pela chave, a mesma
// do orçamento; salary são as partes do salário do mês conferidas com o banco (vazio sem salário configurado).
func BuildMonthSheet(month time.Time, lines []domain.MonthLine, classes map[string]domain.ExpenseClass, salary []domain.SalaryStatus) MonthSheet {
	s := MonthSheet{Month: month}
	income := newGroup()
	fixed := newGroup()
	other := newGroup()
	type card struct {
		sheet SheetCard
		cats  *group
	}
	cards := map[string]*card{}
	var cardOrder []string

	for _, l := range lines {
		switch l.Kind {
		case string(domain.KindIncome):
			label := l.Label
			if l.Category == string(domain.CategorySalary) {
				label = salaryLabel
			}
			income.add(label, l)
		case string(domain.KindTransfer):
			if l.Direction == string(domain.TransferDirectionIn) {
				s.Invested -= l.Amount
			} else {
				s.Invested += l.Amount
			}
		case string(domain.KindExpense):
			if l.Card {
				id := l.AccountID
				c := cards[id]
				if c == nil {
					name := l.AccountName
					if name == "" {
						name = "Cartão (lançado à mão)"
					}
					c = &card{sheet: SheetCard{Name: name}, cats: newGroup()}
					cards[id] = c
					cardOrder = append(cardOrder, id)
				}
				c.sheet.Total += l.Amount
				switch l.Synthetic {
				case "forecast":
					c.sheet.Forecast += l.Amount
				case "bill":
					c.sheet.Adjustment += l.Amount
				default:
					if l.Amount < 0 {
						c.sheet.Refunds += l.Amount
					}
					c.cats.add(domain.Category(l.Category).Label(), l)
				}
				continue
			}
			if class := classes[l.Key]; class == domain.ClassFixed || class == domain.ClassInstallment {
				fixed.add(l.Label, l)
			} else if l.Category == string(domain.CategoryOther) {
				other.add(l.Label, l) // sem categoria, o nome diz mais (Pix para uma pessoa, um boleto)
			} else {
				other.add(domain.Category(l.Category).Label(), l)
			}
		}
	}

	s.Income, s.IncomeTotal = income.items()
	s.Fixed, s.FixedTotal = fixed.items()
	s.Other, s.OtherTotal = other.items()
	for _, id := range cardOrder {
		c := cards[id]
		c.sheet.Categories, _ = c.cats.items()
		c.sheet.Total, c.sheet.Forecast, c.sheet.Adjustment, c.sheet.Refunds =
			round2(c.sheet.Total), round2(c.sheet.Forecast), round2(c.sheet.Adjustment), round2(c.sheet.Refunds)
		s.Cards = append(s.Cards, c.sheet)
		s.CardsTotal += c.sheet.Total
	}
	sort.SliceStable(s.Cards, func(i, j int) bool { return s.Cards[i].Total > s.Cards[j].Total })

	for _, p := range salary {
		if p.Received == nil {
			s.Pending = append(s.Pending, SheetPending{Label: "Salário do dia " + strconv.Itoa(p.Day), Date: p.Expected, Amount: p.Amount})
			s.PendingTotal += p.Amount
		}
	}

	s.CardsTotal, s.PendingTotal, s.Invested = round2(s.CardsTotal), round2(s.PendingTotal), round2(s.Invested)
	s.ExpensesTotal = round2(s.FixedTotal + s.OtherTotal + s.CardsTotal)
	s.Leftover = round2(s.IncomeTotal - s.ExpensesTotal)
	s.LeftoverWithPending = round2(s.Leftover + s.PendingTotal)
	return s
}

// group soma linhas pelo rótulo, mantendo a contagem e se há pendência.
type group struct {
	byLabel map[string]*SheetItem
	order   []string
}

func newGroup() *group { return &group{byLabel: map[string]*SheetItem{}} }

func (g *group) add(label string, l domain.MonthLine) {
	it := g.byLabel[label]
	if it == nil {
		it = &SheetItem{Label: label}
		g.byLabel[label] = it
		g.order = append(g.order, label)
	}
	it.Amount += l.Amount
	it.Count++
	it.Pending = it.Pending || l.Pending
	it.Lines = append(it.Lines, SheetLine{Label: l.Label, Date: l.Date, Amount: l.Amount, Note: l.Note})
}

// items devolve as linhas da maior para a menor e o total (em centavos).
func (g *group) items() ([]SheetItem, float64) {
	out := make([]SheetItem, 0, len(g.order))
	var total float64
	for _, label := range g.order {
		it := *g.byLabel[label]
		it.Amount = round2(it.Amount)
		sort.SliceStable(it.Lines, func(i, j int) bool { return it.Lines[i].Date.Before(it.Lines[j].Date) })
		if it.Amount == 0 {
			continue
		}
		out = append(out, it)
		total += it.Amount
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	return out, round2(total)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
