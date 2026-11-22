package insights

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/format"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/balances"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/debts"
	"github.com/google/uuid"
)

// balanceHistoryDays é até onde o histórico de uso do limite é lido.
const balanceHistoryDays = 120

// DebtView é uma dívida com a sua situação calculada.
type DebtView struct {
	Debt         domain.Debt
	AccountName  string // OVERDRAFT: banco e conta do limite
	Installments *debts.InstallmentView
	Limit        *debts.LimitUse
	LastCycle    *debts.LastCycle // OVERDRAFT: quanto do último salário o buraco engoliu
	Forecast     *debts.Forecast
	// Transfers são os Pix e TED enviados e recebidos com quem recebe a dívida (PayeeMatch), do mais recente ao mais
	// antigo, nos últimos transfersDays dias.
	Transfers []domain.MoneyMovement
}

// transfersDays é até onde vão as transferências com o credor mostradas na dívida.
const transfersDays = 90

func (i *Insights) debtStore() ports.DebtStore { return i.debts }

// FlowItem é uma recorrência com a sua situação.
type FlowItem struct {
	Flow domain.RecurringFlow
	View debts.FlowView
}

// Debts calcula a situação de cada dívida no instante now (dia civil do fuso de now).
func (i *Insights) Debts(ctx context.Context, now time.Time) ([]DebtView, error) {
	views, _, err := i.DebtsAndFlows(ctx, now)
	return views, err
}

// RecurringFlows reconhece as transações de cada recorrência cadastrada.
func (i *Insights) RecurringFlows(ctx context.Context, now time.Time) ([]FlowItem, error) {
	store := i.debtStore()
	if store == nil {
		return nil, nil
	}
	flows, err := store.RecurringFlows(ctx)
	if err != nil {
		return nil, err
	}
	today := civilDay(now)
	out := make([]FlowItem, len(flows))
	for n, f := range flows {
		moves, err := store.MovementsWith(ctx, f.PayeeMatch, f.StartMonth.AddDate(0, 0, -7))
		if err != nil {
			return nil, err
		}
		out[n] = FlowItem{Flow: f, View: debts.FlowStatus(f, moves, today)}
	}
	return out, nil
}

// DebtsAndFlows calcula as dívidas e as recorrências; a previsão do cheque especial usa as recorrências cadastradas.
func (i *Insights) DebtsAndFlows(ctx context.Context, now time.Time) ([]DebtView, []FlowItem, error) {
	store := i.debtStore()
	if store == nil {
		return nil, nil, nil
	}
	flows, err := i.RecurringFlows(ctx, now)
	if err != nil {
		return nil, nil, err
	}
	views, err := i.debtViews(ctx, now, flows)
	return views, flows, err
}

func (i *Insights) debtViews(ctx context.Context, now time.Time, flows []FlowItem) ([]DebtView, error) {
	store := i.debtStore()
	list, err := store.Debts(ctx)
	if err != nil {
		return nil, err
	}
	today := civilDay(now)
	var accounts []domain.Account
	if len(list) > 0 {
		if accounts, err = i.reader.Accounts(ctx); err != nil {
			return nil, err
		}
	}
	views := make([]DebtView, len(list))
	installments := map[uuid.UUID]*debts.InstallmentView{}
	for n, d := range list {
		views[n].Debt = d
		if d.Kind != domain.DebtInstallments {
			continue
		}
		from := d.FirstDue.AddDate(0, -1, 0)
		if t := today.AddDate(0, 0, -transfersDays); t.Before(from) {
			from = t
		}
		moves, err := store.MovementsWith(ctx, d.PayeeMatch, from)
		if err != nil {
			return nil, err
		}
		for k := len(moves) - 1; k >= 0; k-- {
			if !moves[k].Date.Before(today.AddDate(0, 0, -transfersDays)) {
				views[n].Transfers = append(views[n].Transfers, moves[k])
			}
		}
		v := debts.Installments(d, moves, today)
		views[n].Installments, installments[d.ID] = &v, &v
	}
	for n, d := range list {
		if d.Kind != domain.DebtOverdraft || d.AccountID == nil {
			continue
		}
		if err := i.overdraftView(ctx, &views[n], list, installments, flows, accounts, now); err != nil {
			return nil, err
		}
	}
	return views, nil
}

func (i *Insights) overdraftView(ctx context.Context, v *DebtView, all []domain.Debt, inst map[uuid.UUID]*debts.InstallmentView,
	flows []FlowItem, accounts []domain.Account, now time.Time) error {
	store, d, today := i.debtStore(), v.Debt, civilDay(now)
	var acc *domain.Account
	for n := range accounts {
		if accounts[n].ID == *d.AccountID {
			acc = &accounts[n]
		}
	}
	if acc == nil {
		return nil // a conta sumiu (conexão removida): a dívida fica sem situação até ser editada
	}
	v.AccountName = acc.Name
	days, err := store.BalanceDays(ctx, acc.ID, today.AddDate(0, 0, -balanceHistoryDays))
	if err != nil {
		return err
	}
	use := debts.Usage(acc.Balance, days, today)
	if due, last, ok := debts.DueBalance(acc.Balance, days, today, d.DueDay); ok {
		use.AtDue, use.DueDate = math.Round(max(0, -last)*100)/100, &due
	}
	v.Limit = &use

	dip := debts.NextDip(today, d.DueDay)
	in := debts.ForecastInput{Today: today, Dip: dip, Balance: acc.Balance, FreeDays: d.FreeDays}
	if in.Salaries, err = i.salaryDue(ctx, now, dip); err != nil {
		return err
	}
	// O fundo do poço é a véspera do primeiro salário depois do mergulho: o que sai entre o dia do mergulho e o salário
	// (ex.: uma parcela no dia 13) também conta.
	until := dip
	for _, s := range in.Salaries {
		if s.Date != nil && s.Date.After(dip) {
			until = s.Date.AddDate(0, 0, -1)
			break
		}
	}
	closeDays := domain.DefaultCardCloseDaysBeforeDue
	if i.closeDays != nil {
		closeDays = i.closeDays()
	}
	// Dinheiro guardado nas outras contas e nas caixinhas: pode tapar o buraco.
	var saved float64
	var savedWhere []string
	for _, a := range accounts {
		if a.Type != domain.AccountBank {
			continue
		}
		amount := a.ReservedTotal()
		if a.ID != acc.ID {
			amount += a.Balance
		}
		if amount > 0.005 {
			saved += amount
			savedWhere = append(savedWhere, a.Name)
		}
	}
	if saved > 0 {
		in.Bills = append(in.Bills, debts.Line{Label: "Guardado em outras contas e caixinhas (" + strings.Join(savedWhere, ", ") + ")", Amount: saved})
	}
	credits, err := i.reader.SalaryCredits(ctx, today.AddDate(0, 0, -45), today)
	if err != nil {
		return err
	}
	v.LastCycle = debts.BuildLastCycle(credits, acc.Balance, saved, today)
	// Faturas dos cartões do mesmo banco: saem desta conta. Até o fechamento ainda entram compras: estimadas pelo que se
	// gastou à vista no cartão no mesmo período do mês passado.
	for _, c := range accounts {
		if c.Type != domain.AccountCredit || c.DueDate == nil || acc.InstitutionID == nil || c.InstitutionID == nil || *c.InstitutionID != *acc.InstitutionID {
			continue
		}
		due := domain.DebtDueIn(until, c.DueDate.Day())
		if due.After(until) {
			due = domain.DebtDueIn(until.AddDate(0, -1, 0), c.DueDate.Day())
		}
		if !due.After(today) {
			continue
		}
		total, err := store.CardBillTotal(ctx, c.ID, due)
		if err != nil {
			return err
		}
		if total > 0 {
			in.Bills = append(in.Bills, debts.Line{Label: "Fatura " + balancesCardName(c) + " (já lançado)", Amount: -total, Date: &due})
		}
		closing := domain.CardCloseEstimate(due, closeDays)
		if c.CloseDate != nil && c.DueDate != nil {
			if gap := int(c.DueDate.Sub(*c.CloseDate).Hours() / 24); gap > 0 && gap < 28 {
				closing = domain.CardCloseEstimate(due, gap)
			}
		}
		if closing.After(today) {
			more, err := store.CardSpend(ctx, c.ID, today.AddDate(0, -1, 0), closing.AddDate(0, -1, -1))
			if err != nil {
				return err
			}
			if more > 0 {
				in.Bills = append(in.Bills, debts.Line{Label: fmt.Sprintf("Compras no %s até o fechamento de %s (estimativa: o mesmo do mês passado)",
					balancesCardName(c), closing.Format("02/01")), Amount: -more, Date: &due, Estimate: true})
			}
		}
	}
	var payees []string
	for _, o := range all {
		if o.Kind != domain.DebtInstallments {
			continue
		}
		payees = append(payees, o.PayeeMatch)
		if iv := inst[o.ID]; iv != nil && iv.NextDue != nil && !iv.NextDue.After(until) {
			due := *iv.NextDue
			if due.Before(today) {
				due = today
			}
			in.Bills = append(in.Bills, debts.Line{Label: "Parcela " + o.Name, Amount: -o.InstallmentAmount, Date: &due})
		}
	}
	// Recorrências: as cadastradas valem; sem nenhuma cadastrada, o app adivinha pelo extrato.
	if len(flows) > 0 {
		for _, f := range flows {
			in.Bills = append(in.Bills, debts.FlowLines(f.Flow, f.View, today, until)...)
		}
	} else {
		// Sem cadastro, o de costume: o que entrou e saiu das contas no mesmo período do mês passado e também no mês anterior (financiamento,
		// seguro, Pix recorrentes), menos as parcelas de dívidas, que já entram acima.
		from, to := today.AddDate(0, -1, 1), until.AddDate(0, -1, 0)
		last, err := store.BankMovements(ctx, from, to)
		if err != nil {
			return err
		}
		// Uma semana de folga no mês anterior: a conta de um dia 10 pode ter caído no dia 8.
		before, err := store.BankMovements(ctx, from.AddDate(0, -1, -7), to.AddDate(0, -1, 7))
		if err != nil {
			return err
		}
		keep := func(list []domain.MoneyMovement) []domain.MoneyMovement {
			var out []domain.MoneyMovement
			for _, o := range list {
				skip := false
				for _, p := range payees {
					skip = skip || !o.In && domain.MatchesPayee(o.Label, p)
				}
				if !skip {
					out = append(out, o)
				}
			}
			return out
		}
		in.Bills = append(in.Bills, debts.Recurring(keep(last), keep(before))...)
	}
	in.Until = until
	f := debts.BuildForecast(in)
	v.Forecast = &f
	return nil
}

// salaryDue são as partes do salário que ainda vão cair de hoje até um mês depois de until, em ordem de data.
func (i *Insights) salaryDue(ctx context.Context, now, until time.Time) ([]debts.Line, error) {
	parts := i.salaryParts()
	if len(parts) == 0 {
		return nil, nil
	}
	today := civilDay(now)
	view, err := i.Salary(ctx, now)
	if err != nil {
		return nil, err
	}
	var out []debts.Line
	for _, s := range view.Parts {
		if s.Received == nil && !s.Expected.Before(today) {
			e := s.Expected
			out = append(out, debts.Line{Label: fmt.Sprintf("Salário dia %d", s.Day), Amount: s.Amount, Date: &e})
		}
	}
	limit := until.AddDate(0, 1, 0)
	// As partes de view são as do ciclo de hoje; daqui em diante, os ciclos seguintes.
	for m := i.CycleKey(today).AddDate(0, 1, 0); !m.After(limit); m = m.AddDate(0, 1, 0) {
		for _, p := range parts {
			e := domain.SalaryPayDate(m.Year(), m.Month(), p.Day)
			if !e.After(limit) {
				out = append(out, debts.Line{Label: fmt.Sprintf("Salário dia %d", p.Day), Amount: p.Amount, Date: &e})
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Date.Before(*out[b].Date) })
	return out, nil
}

// coverOverdraft esconde o negativo das contas cujo limite cobre uma dívida: a conta aparece zerada e o uso do limite vai
// para Overdraft, com o nome da dívida. Os totais deixam de descontar o negativo, que já aparece como dívida.
func coverOverdraft(view *balances.BalancesView, list []domain.Debt) {
	labels := map[string]string{}
	for _, d := range list {
		if d.Kind == domain.DebtOverdraft && d.AccountID != nil {
			labels[d.AccountID.String()] = d.Name
		}
	}
	if len(labels) == 0 {
		return
	}
	var added float64
	for n := range view.Institutions {
		in := &view.Institutions[n]
		for k := range in.Accounts {
			a := &in.Accounts[k]
			label, ok := labels[a.ID]
			if !ok || a.Balance >= 0 {
				continue
			}
			a.Overdraft, a.OverdraftDebt = -a.Balance, label
			in.Total -= a.Balance
			added -= a.Balance
			a.Balance = 0
		}
	}
	if added == 0 || view.TotalInAccount == nil {
		return
	}
	total := *view.TotalInAccount + added
	view.TotalInAccount = &total
	view.Recompute()
}

func balancesCardName(c domain.Account) string {
	if c.Last4 != "" {
		return "cartão ·· " + c.Last4
	}
	return "do cartão"
}

func civilDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DebtsText resume as dívidas para o /saldos; vazio se não há nenhuma.
func DebtsText(views []DebtView) string {
	if len(views) == 0 {
		return ""
	}
	out := "\n\n🤝 *Dívidas*\n"
	for _, v := range views {
		d := v.Debt
		switch {
		case v.Installments != nil:
			iv := v.Installments
			out += fmt.Sprintf("• %s: %d de %d pagas · falta %s", d.Name, iv.Paid, iv.Total, format.FormatBRL(iv.Remaining))
			if iv.NextDue != nil {
				out += " · próxima " + iv.NextDue.Format("02/01")
			}
		case v.Limit != nil && v.Limit.InLimit > 0:
			out += fmt.Sprintf("• %s: %s no limite (dia %d de %d sem juros)", d.Name, format.FormatBRL(v.Limit.InLimit), v.Limit.DaysUsed, d.FreeDays)
		case v.LastCycle != nil:
			out += fmt.Sprintf("• %s: o salário de %s cobriu %s do buraco", d.Name, v.LastCycle.SalaryDate.Format("02/01"), format.FormatBRL(v.LastCycle.Amount))
			if v.Forecast != nil && v.Forecast.CommittedInLimit > 0 {
				out += fmt.Sprintf(" · previsão %s: %s no limite", v.Forecast.Date.Format("02/01"), format.FormatBRL(v.Forecast.CommittedInLimit))
			}
		case v.Forecast != nil && v.Forecast.CommittedInLimit > 0:
			out += fmt.Sprintf("• %s: previsão %s: %s no limite", d.Name, v.Forecast.Date.Format("02/01"), format.FormatBRL(v.Forecast.CommittedInLimit))
		default:
			out += fmt.Sprintf("• %s: coberta", d.Name)
		}
		out += "\n"
	}
	return out
}
