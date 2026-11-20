package insights

import (
	"context"
	"slices"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/balances"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/review"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/planning"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// goalProjectionMonths é até onde vai a projeção usada nas metas.
const goalProjectionMonths = 24

// Insights reúne as leituras que alimentam a revisão, as metas, a economia realizada, o Coach e o resumo
// semanal: busca os dados no banco e roda os cálculos. Tanto a API quanto o agendador de resumos usam.
type Insights struct {
	reader    ports.InsightsReader
	closeDays func() int // dias entre o fechamento e o vencimento da fatura (configuração); nil = padrão
	salary    func() []domain.SalaryPart
}

// SetSalarySchedule liga a configuração das partes do salário (lida a cada consulta); nil ou vazio = sem salário configurado.
func (i *Insights) SetSalarySchedule(f func() []domain.SalaryPart) { i.salary = f }

func (i *Insights) salaryParts() []domain.SalaryPart {
	if i.salary == nil {
		return nil
	}
	return i.salary()
}

// SalaryView é o salário do mês: as partes conferidas com o banco e os créditos extras (13º, PLR, férias).
type SalaryView struct {
	Configured bool
	Parts      []domain.SalaryStatus
	Extras     []domain.SalaryCredit
}

// Salary confere as partes do salário do mês de today contra os créditos que chegaram do banco. Configured=false quando
// o salário não está configurado.
func (i *Insights) Salary(ctx context.Context, today time.Time) (SalaryView, error) {
	parts := i.salaryParts()
	if len(parts) == 0 {
		return SalaryView{}, nil
	}
	month := MonthStart(today)
	// Folga nas pontas: a parte de um dia 1 pode cair no fim do mês anterior.
	credits, err := i.reader.SalaryCredits(ctx, month.AddDate(0, 0, -domain.SalaryMatchDays), month.AddDate(0, 1, domain.SalaryMatchDays))
	if err != nil {
		return SalaryView{}, err
	}
	status, extras := domain.SalaryMonth(parts, month, credits)
	return SalaryView{Configured: true, Parts: status, Extras: extras}, nil
}

// SetCardCloseDays liga a configuração de quantos dias antes do vencimento a fatura fecha (lida a cada consulta).
func (i *Insights) SetCardCloseDays(f func() int) { i.closeDays = f }

func NewInsights(reader ports.InsightsReader) *Insights { return &Insights{reader: reader} }

// MonthStart é o primeiro dia do mês de t, no fuso de t (quem chama passa o instante já no fuso do dono), como data em UTC.
func MonthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// BankBalance soma o saldo das contas correntes; nil quando não há conta sincronizada ("em conta" é desconhecido, não zero).
func BankBalance(accounts []domain.Account) *float64 {
	var bank *float64
	for _, acc := range accounts {
		if acc.Type == domain.AccountBank {
			sum := acc.Balance
			if bank != nil {
				sum += *bank
			}
			bank = &sum
		}
	}
	return bank
}

// Projection projeta `months` meses a partir de start (primeiro dia do mês atual).
func (i *Insights) Projection(ctx context.Context, start time.Time, months int) (planning.Projection, error) {
	from, to := start.AddDate(0, -review.BudgetMonths, 0), start.AddDate(0, -1, 0)
	rows, err := i.reader.ExpenseKeyMonths(ctx, from, to)
	if err != nil {
		return planning.Projection{}, err
	}
	rules, err := i.reader.ExpenseRules(ctx)
	if err != nil {
		return planning.Projection{}, err
	}
	incomes, err := i.reader.IncomePayments(ctx, from, to)
	if err != nil {
		return planning.Projection{}, err
	}
	known, err := i.reader.KnownInstallments(ctx, start, start.AddDate(0, months-1, 0))
	if err != nil {
		return planning.Projection{}, err
	}
	return planning.BuildProjection(rows, rules, incomes, i.salaryParts(), known, start, months), nil
}

// MonthSheet monta a tela do mês (primeiro dia do mês): os lançamentos, a classe de cada conta como no orçamento (12
// meses até o mês) e o salário do mês conferido com o banco.
func (i *Insights) MonthSheet(ctx context.Context, month time.Time) (planning.MonthSheet, error) {
	lines, err := i.reader.MonthLines(ctx, month)
	if err != nil {
		return planning.MonthSheet{}, err
	}
	from := month.AddDate(0, -(review.BudgetMonths - 1), 0)
	rows, err := i.reader.ExpenseKeyMonths(ctx, from, month)
	if err != nil {
		return planning.MonthSheet{}, err
	}
	rules, err := i.reader.ExpenseRules(ctx)
	if err != nil {
		return planning.MonthSheet{}, err
	}
	salary, err := i.Salary(ctx, month)
	if err != nil {
		return planning.MonthSheet{}, err
	}
	classes := planning.BuildBudget(rows, rules, from, month).Classes
	return planning.BuildMonthSheet(month, lines, classes, salary.Parts), nil
}

// Review roda os detectores da revisão sobre o mês to (primeiro dia).
func (i *Insights) Review(ctx context.Context, to time.Time) (review.Review, error) {
	cats, err := i.reader.CategoryMonths(ctx, to.AddDate(0, -(review.ReviewMatrixMonths-1), 0), to)
	if err != nil {
		return review.Review{}, err
	}
	keyRows, err := i.reader.ExpenseKeyMonths(ctx, to.AddDate(0, -(review.BudgetMonths-1), 0), to)
	if err != nil {
		return review.Review{}, err
	}
	rules, err := i.reader.ExpenseRules(ctx)
	if err != nil {
		return review.Review{}, err
	}
	payments, err := i.reader.ExpensePayments(ctx, to, to)
	if err != nil {
		return review.Review{}, err
	}
	dismissed, err := i.reader.Dismissals(ctx)
	if err != nil {
		return review.Review{}, err
	}
	return review.BuildReview(cats, keyRows, rules, payments, dismissed, to), nil
}

// Goals calcula o andamento de todas as metas hoje e o patrimônio (contas correntes + investimentos).
func (i *Insights) Goals(ctx context.Context, today time.Time) ([]planning.GoalProgress, float64, error) {
	now := MonthStart(today)
	goals, err := i.reader.Goals(ctx)
	if err != nil {
		return nil, 0, err
	}
	accounts, err := i.reader.Accounts(ctx)
	if err != nil {
		return nil, 0, err
	}
	positions, err := i.reader.Positions(ctx)
	if err != nil {
		return nil, 0, err
	}
	proj, err := i.Projection(ctx, now, goalProjectionMonths)
	if err != nil {
		return nil, 0, err
	}
	var cats []domain.CategoryMonth
	if slices.ContainsFunc(goals, func(g domain.Goal) bool { return g.Kind == domain.GoalCut }) {
		if cats, err = i.reader.CategoryMonths(ctx, now.AddDate(0, -11, 0), now); err != nil {
			return nil, 0, err
		}
	}

	var wealth float64
	if bank := BankBalance(accounts); bank != nil {
		wealth = *bank
	}
	for _, p := range positions {
		wealth += p.Balance
	}
	out := make([]planning.GoalProgress, len(goals))
	for n, g := range goals {
		out[n] = planning.BuildGoalProgress(g, wealth, proj, cats, today)
	}
	return out, wealth, nil
}

// Savings confere as decisões "cancelei" contra o que foi cobrado desde então (now = primeiro dia do mês atual).
func (i *Insights) Savings(ctx context.Context, now time.Time) ([]review.DecisionResult, error) {
	decisions, err := i.reader.Decisions(ctx)
	if err != nil || len(decisions) == 0 {
		return nil, err
	}
	from := slices.MinFunc(decisions, func(x, y domain.Decision) int { return x.Month.Compare(y.Month) }).Month
	rows, err := i.reader.ExpenseKeyMonths(ctx, from, now)
	if err != nil {
		return nil, err
	}
	return review.BuildSavings(decisions, rows, now), nil
}

// Balances monta os saldos por banco no instante now.
func (i *Insights) Balances(ctx context.Context, now time.Time) (balances.BalancesView, error) {
	accounts, err := i.reader.Accounts(ctx)
	if err != nil {
		return balances.BalancesView{}, err
	}
	institutions, err := i.reader.Institutions(ctx)
	if err != nil {
		return balances.BalancesView{}, err
	}
	days := domain.DefaultCardCloseDaysBeforeDue
	if i.closeDays != nil {
		days = i.closeDays()
	}
	return balances.BuildBalances(accounts, institutions, now, days), nil
}

// BalancesText é o comando /saldos: o mesmo cálculo do painel, em texto.
func (i *Insights) BalancesText(ctx context.Context, now time.Time) (string, error) {
	view, err := i.Balances(ctx, now)
	if err != nil {
		return "", err
	}
	return balances.FormatBalances(view, now), nil
}
