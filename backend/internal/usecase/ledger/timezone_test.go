package ledger

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func saoPaulo(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// 2026-11-30 22:30 em São Paulo é 2026-12-01 01:30Z: em UTC já seria dezembro.
func TestGenerateRecurringExpenses_UsesTheOwnersTimezone(t *testing.T) {
	loc := saoPaulo(t)
	now := time.Date(2026, 11, 30, 22, 30, 0, 0, loc)

	day30, day1 := recurringPurchase(30), recurringPurchase(1)
	var saved []*domain.Payment
	repo := &mockLedger{
		findActiveRecurringFn: func(context.Context) ([]domain.Purchase, error) {
			return []domain.Purchase{day30, day1}, nil
		},
		hasPaymentForMonthFn: func(context.Context, uuid.UUID, time.Time) (bool, error) { return false, nil },
		savePaymentFn: func(_ context.Context, p *domain.Payment) error {
			saved = append(saved, p)
			return nil
		},
	}

	uc := NewAnalyzeExpense(repo, &mockAnalyzer{}, slog.Default())
	uc.SetClock(domain.FixedClock{T: now.UTC()}) // o relógio devolve o instante em UTC; o fuso vem do SetLocation
	uc.SetLocation(loc)
	if err := uc.GenerateRecurringExpenses(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A do dia 30 é lançada no dia; a do dia 1, sem pagamento em novembro, é recuperada (H4). As duas em novembro.
	if len(saved) != 2 {
		t.Fatalf("deveria lançar as duas em novembro, got %d lançamentos", len(saved))
	}
	for _, p := range saved {
		if want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC); !p.ReferenceMonth.Equal(want) {
			t.Errorf("reference_month = %v, esperado %v", p.ReferenceMonth, want)
		}
	}
}

func TestGenerateRecurringExpenses_UTCOwnerIsUnchanged(t *testing.T) {
	// Sem fuso configurado (UTC), 01:30Z de 01/12 é dezembro: lança a do dia 1, não a do dia 30.
	day30, day1 := recurringPurchase(30), recurringPurchase(1)
	var saved []*domain.Payment
	repo := &mockLedger{
		findActiveRecurringFn: func(context.Context) ([]domain.Purchase, error) {
			return []domain.Purchase{day30, day1}, nil
		},
		hasPaymentForMonthFn: func(context.Context, uuid.UUID, time.Time) (bool, error) { return false, nil },
		savePaymentFn:        func(_ context.Context, p *domain.Payment) error { saved = append(saved, p); return nil },
	}
	uc := NewAnalyzeExpense(repo, &mockAnalyzer{}, slog.Default())
	uc.SetClock(domain.FixedClock{T: time.Date(2026, 12, 1, 1, 30, 0, 0, time.UTC)})
	if err := uc.GenerateRecurringExpenses(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].PurchaseID != day1.ID || !saved[0].ReferenceMonth.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("em UTC deveria lançar só a do dia 1 em dezembro: %+v", saved)
	}
}

func TestProcessExportCSV_DefaultMonthIsTheOwnersMonth(t *testing.T) {
	loc := saoPaulo(t)
	uc := NewAnalyzeExpense(&mockLedger{}, &mockAnalyzer{}, slog.Default())
	uc.SetClock(domain.FixedClock{T: time.Date(2026, 12, 1, 1, 30, 0, 0, time.UTC)}) // 22:30 de 30/11 em São Paulo
	uc.SetLocation(loc)

	out, err := uc.processExportCSV(&ports.ExpenseAnalysis{})
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC); !out.ExportMonthTime.Equal(want) {
		t.Errorf("CSV sem mês deveria ser de novembro, got %v", out.ExportMonthTime)
	}

	uc.SetLocation(time.UTC)
	out, _ = uc.processExportCSV(&ports.ExpenseAnalysis{})
	if want := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC); !out.ExportMonthTime.Equal(want) {
		t.Errorf("em UTC continua dezembro, got %v", out.ExportMonthTime)
	}
}

func TestResolveQueryMonth_UsesGivenNow(t *testing.T) {
	loc := saoPaulo(t)
	now := time.Date(2026, 12, 1, 1, 30, 0, 0, time.UTC).In(loc)
	if got := resolveQueryMonth(nil, now); !got.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("got %v", got)
	}
}

func TestPreviousMonth(t *testing.T) {
	loc := saoPaulo(t)
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"1º de dezembro 00:05 local pede novembro", time.Date(2026, 12, 1, 0, 5, 0, 0, loc), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
		{"virada do ano", time.Date(2027, 1, 1, 0, 5, 0, 0, loc), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)},
		{"meio do mês", time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		if got := previousMonth(tc.now); !got.Equal(tc.want) {
			t.Errorf("%s: got %v, esperado %v", tc.name, got, tc.want)
		}
	}
}

func TestMonthlyReport_AsksForThePreviousMonthInTheOwnersTimezone(t *testing.T) {
	loc := saoPaulo(t)
	var asked time.Time
	exporter := &mockCSVExporterUC{executeFn: func(_ context.Context, m time.Time) ([]byte, string, *ExportSummary, error) {
		asked = m
		return nil, "", nil, nil
	}}
	r := NewMonthlyReport(exporter, &mockMessengerUC{}, "x", silentReportLogger())
	r.SetLocation(loc)

	r.SetClock(domain.FixedClock{T: time.Date(2026, 12, 1, 3, 5, 0, 0, time.UTC)}) // 00:05 de 01/12 em São Paulo
	if err := r.Send(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC); !asked.Equal(want) {
		t.Errorf("deveria pedir novembro, got %v", asked)
	}

	// 21:30 de 30/11 em São Paulo (00:30Z de 01/12) ainda é novembro lá: o mês anterior é outubro.
	r.SetClock(domain.FixedClock{T: time.Date(2026, 12, 1, 0, 30, 0, 0, time.UTC)})
	_ = r.Send(context.Background())
	if want := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC); !asked.Equal(want) {
		t.Errorf("às 21:30 de 30/11 local o anterior é outubro, got %v", asked)
	}
}
