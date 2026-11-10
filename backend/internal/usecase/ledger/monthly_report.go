package ledger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

type MonthlyReport struct {
	exporter  CSVExporter
	messenger ports.Messenger
	phone     string
	logger    *slog.Logger
	clock     domain.Clock
	loc       *time.Location // fuso do dono: o "mês anterior" é o do calendário dele
}

func NewMonthlyReport(exporter CSVExporter, messenger ports.Messenger, phone string, logger *slog.Logger) *MonthlyReport {
	return &MonthlyReport{
		exporter:  exporter,
		messenger: messenger,
		phone:     phone,
		logger:    logger,
		clock:     domain.SystemClock{},
		loc:       time.UTC,
	}
}

// SetClock troca o relógio (testes).
func (r *MonthlyReport) SetClock(c domain.Clock) { r.clock = c }

// SetLocation define o fuso do dono. O padrão é UTC.
func (r *MonthlyReport) SetLocation(loc *time.Location) {
	if loc != nil {
		r.loc = loc
	}
}

func (r *MonthlyReport) Send(ctx context.Context) error {
	prevMonth := previousMonth(r.clock.Now().In(r.loc))

	data, filename, summary, err := r.exporter.Execute(ctx, prevMonth)
	if err != nil {
		return fmt.Errorf("erro ao gerar relatório mensal: %w", err)
	}

	if data == nil {
		r.logger.Info("sem lançamentos no mês anterior, relatório não enviado",
			"month", prevMonth.Format("01/2006"))
		return nil
	}

	caption := BuildExportCaption(prevMonth, summary)

	if _, err := r.messenger.SendDocument(ctx, r.phone, filename, data, caption); err != nil {
		return fmt.Errorf("erro ao enviar relatório mensal: %w", err)
	}

	r.logger.Info("relatório mensal enviado", "month", prevMonth.Format("01/2006"))
	return nil
}

// previousMonth devolve o primeiro dia do mês anterior ao de now, como data (meia-noite UTC, para a coluna DATE).
func previousMonth(now time.Time) time.Time {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, -1, 0)
}
