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
	debts     ports.DebtStore // nil sem dívidas (leitores de teste)
	cycles    func() domain.Cycles
}

// SetCycles liga os ciclos do salário (lidos a cada consulta); nil = mês do calendário.
func (i *Insights) SetCycles(f func() domain.Cycles) { i.cycles = f }

// Cycles são os ciclos em uso (vazio = mês do calendário).
func (i *Insights) Cycles() domain.Cycles {
	if i.cycles == nil {
		return domain.Cycles{}
	}
	return i.cycles()
}

// CycleKey é a chave do ciclo em que today cai.
func (i *Insights) CycleKey(today time.Time) time.Time { return i.Cycles().KeyOf(today) }

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

// Salary confere as partes do salário do ciclo de today contra os créditos que chegaram do banco. Configured=false quando
// o salário não está configurado.
func (i *Insights) Salary(ctx context.Context, today time.Time) (SalaryView, error) {
	return i.salaryFor(ctx, i.CycleKey(today))
}

// salaryFor confere as partes do salário do ciclo month (a chave) contra os créditos do banco.
func (i *Insights) salaryFor(ctx context.Context, month time.Time) (SalaryView, error) {
	parts := i.salaryParts()
	if len(parts) == 0 {
		return SalaryView{}, nil
	}
	// Folga nas pontas: uma parte pode cair alguns dias antes ou depois do previsto.
	start, end := i.Cycles().Bounds(month)
	credits, err := i.reader.SalaryCredits(ctx, start.AddDate(0, 0, -domain.SalaryMatchDays), end.AddDate(0, 0, domain.SalaryMatchDays))
	if err != nil {
		return SalaryView{}, err
	}
	status, extras := domain.SalaryMonth(parts, month, credits)
	return SalaryView{Configured: true, Parts: status, Extras: extras}, nil
}

// SetCardCloseDays liga a configuração de quantos dias antes do vencimento a fatura fecha (lida a cada consulta).
func (i *Insights) SetCardCloseDays(f func() int) { i.closeDays = f }

func NewInsights(reader ports.InsightsReader) *Insights {
	i := &Insights{reader: reader}
	if store, ok := reader.(ports.DebtStore); ok {
		i.debts = store
	}
	return i
}

// MonthStart é o primeiro dia do mês de t, no fuso de t (quem chama passa o instante já no fuso do dono), como data em UTC.
func MonthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// BankBalance soma o saldo das contas correntes; nil quando não há conta sincronizada ("em conta" é desconhecido, não zero).
func BankBalance(accounts []domain.Account) *float64 {
	var bank *float64
	for _, acc := range accounts {
		if acc.Type == domain.AccountBank {
			sum := acc.Balance + acc.ReservedTotal() // caixinhas são dinheiro da conta
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
func (i *Insights) MonthSheet(ctx context.Context, month, now time.Time) (planning.MonthSheet, error) {
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
	salary, err := i.salaryFor(ctx, month)
	if err != nil {
		return planning.MonthSheet{}, err
	}
	classes := planning.BuildBudget(rows, rules, from, month).Classes
	sheet := planning.BuildMonthSheet(month, lines, classes, salary.Parts)
	sheet.StartsOn, sheet.EndsOn = i.Cycles().Bounds(month)
	// A receber: as mesmas previstas da tela de Transações (salário e entradas recorrentes), não só o salário.
	expected, err := i.ExpectedIncomes(ctx, month, now)
	if err != nil {
		return planning.MonthSheet{}, err
	}
	pending := make([]planning.SheetPending, len(expected))
	for n, e := range expected {
		pending[n] = planning.SheetPending{Label: e.Label, Date: e.Date, Amount: e.Amount}
	}
	sheet.SetPending(pending)
	// O que sobrou do ciclo anterior (entradas − despesas dele) passa para este.
	prevLines, err := i.reader.MonthLines(ctx, month.AddDate(0, -1, 0))
	if err != nil {
		return planning.MonthSheet{}, err
	}
	var carry *float64
	if len(prevLines) > 0 {
		v := planning.BuildMonthSheet(month.AddDate(0, -1, 0), prevLines, nil, nil).Leftover
		carry = &v
	}
	sheet.SetCarryover(carry)
	return sheet, nil
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
	now := i.CycleKey(today)
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
		out[n] = planning.BuildGoalProgress(g, wealth, proj, cats, today, i.Cycles())
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
	view := balances.BuildBalances(accounts, institutions, now, days)
	if i.debts != nil {
		list, err := i.debts.Debts(ctx)
		if err != nil {
			return balances.BalancesView{}, err
		}
		coverOverdraft(&view, list)
	}
	return view, nil
}

// BalancesText é o comando /saldos: o mesmo cálculo do painel, em texto.
func (i *Insights) BalancesText(ctx context.Context, now time.Time) (string, error) {
	view, err := i.Balances(ctx, now)
	if err != nil {
		return "", err
	}
	text := balances.FormatBalances(view, now)
	if list, err := i.Debts(ctx, now); err == nil {
		text += DebtsText(list)
	}
	return text, nil
}
