package insights

import (
	"context"
	"fmt"
	"sort"
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
	Forecast     *debts.Forecast
}

func (i *Insights) debtStore() ports.DebtStore { return i.debts }

// Debts calcula a situação de cada dívida no instante now (dia civil do fuso de now).
func (i *Insights) Debts(ctx context.Context, now time.Time) ([]DebtView, error) {
	store := i.debtStore()
	if store == nil {
		return nil, nil
	}
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
		hits, err := store.PaymentsMatching(ctx, d.PayeeMatch, d.FirstDue.AddDate(0, -1, 0))
		if err != nil {
			return nil, err
		}
		v := debts.Installments(d, hits, today)
		views[n].Installments, installments[d.ID] = &v, &v
	}
	for n, d := range list {
		if d.Kind != domain.DebtOverdraft || d.AccountID == nil {
			continue
		}
		if err := i.overdraftView(ctx, &views[n], list, installments, accounts, now); err != nil {
			return nil, err
		}
	}
	return views, nil
}

func (i *Insights) overdraftView(ctx context.Context, v *DebtView, all []domain.Debt, inst map[uuid.UUID]*debts.InstallmentView,
	accounts []domain.Account, now time.Time) error {
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
	v.Limit = &use

	dip := debts.NextDip(today, d.DueDay)
	in := debts.ForecastInput{Today: today, Dip: dip, Balance: acc.Balance, FreeDays: d.FreeDays}
	if in.Salaries, err = i.salaryDue(ctx, now, dip); err != nil {
		return err
	}
	// Faturas dos cartões do mesmo banco: saem desta conta.
	for _, c := range accounts {
		if c.Type != domain.AccountCredit || c.DueDate == nil || acc.InstitutionID == nil || c.InstitutionID == nil || *c.InstitutionID != *acc.InstitutionID {
			continue
		}
		due := domain.DebtDueIn(dip, c.DueDate.Day())
		if due.After(dip) {
			due = domain.DebtDueIn(dip.AddDate(0, -1, 0), c.DueDate.Day())
		}
		if !due.After(today) {
			continue
		}
		total, err := store.CardBillTotal(ctx, c.ID, due)
		if err != nil {
			return err
		}
		if total > 0 {
			in.Bills = append(in.Bills, debts.Line{Label: "Fatura " + balancesCardName(c), Amount: -total, Date: &due})
		}
	}
	var payees []string
	for _, o := range all {
		if o.Kind != domain.DebtInstallments {
			continue
		}
		payees = append(payees, o.PayeeMatch)
		if iv := inst[o.ID]; iv != nil && iv.NextDue != nil && !iv.NextDue.After(dip) {
			due := *iv.NextDue
			if due.Before(today) {
				due = today
			}
			in.Bills = append(in.Bills, debts.Line{Label: "Parcela " + o.Name, Amount: -o.InstallmentAmount, Date: &due})
		}
	}
	// O de costume: o que saiu desta conta no mesmo período do mês passado (financiamento, boletos, Pix do dia a dia).
	outs, err := store.AccountOutflows(ctx, acc.ID, today.AddDate(0, -1, 1), dip.AddDate(0, -1, 0))
	if err != nil {
		return err
	}
	var usual float64
	for _, o := range outs {
		skip := false
		for _, p := range payees {
			skip = skip || debts.Matches(o.Label, p)
		}
		if !skip {
			usual += o.Amount
		}
	}
	if usual > 0 {
		from := today.AddDate(0, -1, 1)
		in.Usual = &debts.Line{Label: fmt.Sprintf("Saídas de costume (%s a %s do mês passado)", from.Format("02/01"),
			dip.AddDate(0, -1, 0).Format("02/01")), Amount: -usual}
	}
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
	for m := MonthStart(today).AddDate(0, 1, 0); !m.After(limit); m = m.AddDate(0, 1, 0) {
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
		case v.Forecast != nil && v.Forecast.InLimit > 0:
			out += fmt.Sprintf("• %s: coberta agora · previsão %s: %s no limite", d.Name, v.Forecast.Date.Format("02/01"), format.FormatBRL(v.Forecast.InLimit))
		default:
			out += fmt.Sprintf("• %s: coberta", d.Name)
		}
		out += "\n"
	}
	return out
}
