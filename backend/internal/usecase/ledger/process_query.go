package ledger

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (uc *AnalyzeExpense) processQuery(ctx context.Context, analysis *ports.ExpenseAnalysis) (*ExpenseOutput, error) {
	targetMonth := resolveQueryMonth(analysis.QueryInfo, uc.now())

	if analysis.QueryInfo != nil && analysis.QueryInfo.Category != "" {
		return uc.processCategoryQuery(ctx, targetMonth, domain.Category(analysis.QueryInfo.Category))
	}

	summaries, err := uc.repo.FindPaymentsByMonth(ctx, targetMonth)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas: %w", err)
	}

	incomeTotal, err := uc.repo.FindIncomeTotalByMonth(ctx, targetMonth)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar entradas: %w", err)
	}

	applied, redeemed, err := uc.repo.FindTransferNetByMonth(ctx, targetMonth)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar transferências: %w", err)
	}

	if len(summaries) == 0 && incomeTotal == 0 && applied == 0 && redeemed == 0 {
		return &ExpenseOutput{
			Type:       "QUERY",
			QueryMonth: formatMonthPT(targetMonth),
			QueryEmpty: true,
		}, nil
	}

	var total float64
	categories := make([]CategorySummary, 0, len(summaries))
	for _, s := range summaries {
		total += s.Total
		categories = append(categories, CategorySummary{
			Category: domain.Category(s.Category).Label(),
			Total:    s.Total,
		})
	}

	netInvested := applied - redeemed
	return &ExpenseOutput{
		Type:             "QUERY",
		QueryMonth:       formatMonthPT(targetMonth),
		QueryTotal:       total,
		QueryCategories:  categories,
		QueryIncome:      incomeTotal,
		QueryBalance:     incomeTotal - total,
		QueryApplied:     applied,
		QueryRedeemed:    redeemed,
		QueryNetInvested: netInvested,
		QueryInAccount:   (incomeTotal - total) - netInvested,
	}, nil
}

// queryTopItems é quantos estabelecimentos a consulta por categoria mostra.
const queryTopItems = 5

// processCategoryQuery responde "quanto gastei com X": o total da categoria no mês (mesma regra do resumo:
// despesas não canceladas) e os estabelecimentos que mais pesaram nela.
func (uc *AnalyzeExpense) processCategoryQuery(ctx context.Context, month time.Time, category domain.Category) (*ExpenseOutput, error) {
	details, err := uc.repo.FindPaymentDetailsByMonth(ctx, month)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas: %w", err)
	}

	var total float64
	count := 0
	byDesc := map[string]*CategoryItem{}
	var order []string
	for _, d := range details {
		if d.PurchaseKind != "EXPENSE" || d.Category != string(category) || d.Status == "CANCELLED" {
			continue
		}
		total += d.Amount
		count++
		name := "Sem descrição"
		if d.Description != nil && strings.TrimSpace(*d.Description) != "" {
			name = strings.TrimSpace(*d.Description)
		}
		key := strings.ToLower(name)
		item, ok := byDesc[key]
		if !ok {
			item = &CategoryItem{Description: name}
			byDesc[key] = item
			order = append(order, key)
		}
		item.Total += d.Amount
		item.Count++
	}

	items := make([]CategoryItem, 0, len(order))
	for _, k := range order {
		items = append(items, *byDesc[k])
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Total > items[j].Total })
	if len(items) > queryTopItems {
		items = items[:queryTopItems]
	}

	return &ExpenseOutput{
		Type:          "QUERY",
		QueryMonth:    formatMonthPT(month),
		QueryCategory: category.Label(),
		QueryTotal:    total,
		QueryItems:    items,
		QueryCount:    count,
		QueryEmpty:    total == 0,
	}, nil
}

// resolveQueryMonth devolve o mês pedido; sem ele, o mês de now (que deve vir no fuso do dono).
func resolveQueryMonth(info *ports.QueryInfo, now time.Time) time.Time {
	month, year := int(now.Month()), now.Year()

	if info != nil {
		if info.Month >= 1 && info.Month <= 12 {
			month = info.Month
		}
		if info.Year >= 2000 {
			year = info.Year
		}
	}

	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}

var ptMonths = [...]string{
	"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
	"Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro",
}

func formatMonthPT(t time.Time) string {
	return fmt.Sprintf("%s %d", ptMonths[t.Month()-1], t.Year())
}
