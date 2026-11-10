package app

import (
	"context"
	"errors"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/insights"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"
)

// startJobs liga o que usa o chat (avisos de segurança, código do segundo fator, resumo semanal, relatório mensal) ao
// canal trocável, e a rotina diária. Enquanto não há canal no ar, os envios falham com ports.ErrNoChannel; quando o canal
// liga (no boot, numa nova tentativa ou ao concluir o setup), passam a funcionar sem reiniciar nada.
func (a *app) startJobs() {
	if !a.cfg.DashboardTwoFactor {
		a.logger.Warn("dashboard sem segundo fator: o login pede só a senha", "motivo", "DASHBOARD_2FA=off")
	}
	// O código do segundo fator vai ao chat. Diferente dos avisos, o envio precisa dizer se deu certo: sem o código
	// ninguém entra.
	a.secondFactor.Send = func(ctx context.Context, text string) error {
		_, err := a.channel.SendText(ctx, "", text)
		return err
	}
	a.secondFactor.Configured = a.channelConfigured.Load
	// Avisos de segurança das configurações (senha errada, segredo trocado) vão para o mesmo chat.
	a.settingsDeps.Notify = func(ctx context.Context, text string) {
		if _, err := a.channel.SendText(ctx, "", text); err != nil && !errors.Is(err, ports.ErrNoChannel) {
			a.logger.Error("erro ao enviar aviso de segurança", "error", err)
		}
	}
	go insights.NewDigestJob(a.insights, &a.channel, "", a.logger, a.settings.Digest, a.onChange()).Run(a.ctx)
	report := ledger.NewMonthlyReport(a.exportCSV, &a.channel, "", a.logger)
	report.SetLocation(a.location())
	go a.runDaily(report)
}

// runDaily gera as despesas recorrentes à meia-noite do fuso do dono (DIGEST_TIMEZONE) e, no dia 1 dele, envia o
// relatório mensal do mês que acabou.
func (a *app) runDaily(monthlyReport *ledger.MonthlyReport) {
	loc := a.location()
	clock := domain.SystemClock{}
	for {
		next := nextMidnight(clock.Now(), loc)
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(time.Until(next)):
			if err := a.analyzeExpense.GenerateRecurringExpenses(a.ctx); err != nil {
				a.logger.Error("erro ao gerar despesas recorrentes", "error", err)
			}
			if isMonthStart(clock.Now(), loc) {
				if err := monthlyReport.Send(a.ctx); err != nil {
					a.logger.Error("erro ao enviar relatório mensal", "error", err)
				}
			}
		}
	}
}

// location é o fuso do dono; sem um configurado, UTC.
func (a *app) location() *time.Location {
	if a.cfg.DigestLocation != nil {
		return a.cfg.DigestLocation
	}
	return time.UTC
}

// nextMidnight é a próxima meia-noite depois de now, no fuso loc. Montar a data pelo calendário (e não somar 24 h) é seguro
// na mudança de horário.
func nextMidnight(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
}

// isMonthStart diz se now é o dia 1 do calendário em loc.
func isMonthStart(now time.Time, loc *time.Location) bool {
	return now.In(loc).Day() == 1
}
