package ledger

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func recurringPurchase(dayOfMonth int) domain.Purchase {
	day := dayOfMonth
	desc := "Netflix"
	return domain.Purchase{
		ID:            uuid.New(),
		Description:   &desc,
		TotalAmount:   55.0,
		Category:      domain.CategoryEntertainment,
		PaymentMethod: domain.PaymentMethodCreditCard,
		Type:          domain.PurchaseTypeRecurring,
		IsActive:      true,
		DayOfMonth:    &day,
		RawInput:      "Netflix todo mês",
	}
}

// recurringFake guarda os pagamentos por recorrente e mês, de qualquer status, como o banco faz.
type recurringFake struct {
	existing map[string]domain.PaymentStatus
	saved    []*domain.Payment
}

func monthKey(id uuid.UUID, month time.Time) string { return id.String() + month.Format("2006-01-02") }

func newRecurringFake(t *testing.T, actives ...domain.Purchase) (*recurringFake, *mockLedger) {
	t.Helper()
	f := &recurringFake{existing: map[string]domain.PaymentStatus{}}
	repo := &mockLedger{
		findActiveRecurringFn: func(context.Context) ([]domain.Purchase, error) { return actives, nil },
		hasPaymentForMonthFn: func(_ context.Context, id uuid.UUID, month time.Time) (bool, error) {
			_, ok := f.existing[monthKey(id, month)]
			return ok, nil
		},
		savePaymentFn: func(_ context.Context, p *domain.Payment) error {
			f.existing[monthKey(p.PurchaseID, *p.ReferenceMonth)] = p.Status
			f.saved = append(f.saved, p)
			return nil
		},
	}
	return f, repo
}

// runRecurringAt roda a geração com o relógio parado em now (no fuso de São Paulo, o do dono).
func runRecurringAt(t *testing.T, repo *mockLedger, now time.Time) {
	t.Helper()
	uc := NewAnalyzeExpense(repo, &mockAnalyzer{}, slog.Default())
	uc.SetClock(domain.FixedClock{T: now.UTC()})
	uc.SetLocation(saoPaulo(t))
	if err := uc.GenerateRecurringExpenses(context.Background()); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestGenerateRecurringExpenses_RecoversWhenAppWasDownOnTheDay(t *testing.T) {
	loc := saoPaulo(t)
	rec := recurringPurchase(5)
	f, repo := newRecurringFake(t, rec)

	runRecurringAt(t, repo, time.Date(2026, 11, 10, 9, 0, 0, 0, loc))

	if len(f.saved) != 1 {
		t.Fatalf("deveria recuperar 1 lançamento, got %d", len(f.saved))
	}
	p := f.saved[0]
	if p.PurchaseID != rec.ID || p.Amount != 55.0 || p.Status != domain.PaymentStatusPaid || p.DueDate != nil {
		t.Errorf("formato diferente do lançamento do dia: %+v", p)
	}
	if want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC); !p.ReferenceMonth.Equal(want) {
		t.Errorf("reference_month = %v, esperado %v", p.ReferenceMonth, want)
	}
	day5 := time.Date(2026, 11, 5, 0, 0, 0, 0, loc)
	if !p.CreatedAt.Equal(day5) || p.PaidAt == nil || !p.PaidAt.Equal(day5) {
		t.Errorf("created_at/paid_at = %v/%v, esperado o dia programado %v", p.CreatedAt, p.PaidAt, day5)
	}
}

func TestGenerateRecurringExpenses_OnTheDayKeepsTheUsualStamp(t *testing.T) {
	loc := saoPaulo(t)
	f, repo := newRecurringFake(t, recurringPurchase(5))
	before := time.Now().Add(-time.Second)

	runRecurringAt(t, repo, time.Date(2026, 11, 5, 0, 0, 30, 0, loc))

	if len(f.saved) != 1 {
		t.Fatalf("deveria lançar 1 no dia, got %d", len(f.saved))
	}
	// O lançamento do dia continua como era: carimbo do momento da gravação.
	if f.saved[0].CreatedAt.Before(before) {
		t.Errorf("lançamento do dia não deveria ter o carimbo trocado: %v", f.saved[0].CreatedAt)
	}
}

func TestGenerateRecurringExpenses_SkipsBeforeTheDay(t *testing.T) {
	f, repo := newRecurringFake(t, recurringPurchase(5))
	runRecurringAt(t, repo, time.Date(2026, 11, 4, 23, 59, 0, 0, saoPaulo(t)))
	if len(f.saved) != 0 {
		t.Fatalf("antes do dia programado não lança, got %d", len(f.saved))
	}
}

func TestGenerateRecurringExpenses_NeverDuplicatesTheMonth(t *testing.T) {
	loc := saoPaulo(t)
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, status := range []domain.PaymentStatus{domain.PaymentStatusPaid, domain.PaymentStatusPending, domain.PaymentStatusCancelled} {
		rec := recurringPurchase(5)
		f, repo := newRecurringFake(t, rec)
		f.existing[monthKey(rec.ID, nov)] = status
		runRecurringAt(t, repo, time.Date(2026, 11, 10, 9, 0, 0, 0, loc))
		if len(f.saved) != 0 {
			t.Errorf("mês já tem pagamento %s: não deveria lançar, got %d", status, len(f.saved))
		}
	}

	// Duas execuções seguidas (subida e meia-noite): só um lançamento.
	f, repo := newRecurringFake(t, recurringPurchase(5))
	runRecurringAt(t, repo, time.Date(2026, 11, 10, 9, 0, 0, 0, loc))
	runRecurringAt(t, repo, time.Date(2026, 11, 11, 0, 0, 5, 0, loc))
	if len(f.saved) != 1 {
		t.Fatalf("duas execuções deveriam gerar 1 lançamento, got %d", len(f.saved))
	}
}

func TestGenerateRecurringExpenses_CreatedAfterTheDayIsNotLaunchedAgain(t *testing.T) {
	// Recorrente cadastrada no dia 10 com dia 5: já nasce com o pagamento do mês (process_recurring).
	rec := recurringPurchase(5)
	f, repo := newRecurringFake(t, rec)
	f.existing[monthKey(rec.ID, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))] = domain.PaymentStatusPaid
	runRecurringAt(t, repo, time.Date(2026, 11, 10, 0, 0, 5, 0, saoPaulo(t)))
	if len(f.saved) != 0 {
		t.Fatalf("não deveria lançar de novo, got %d", len(f.saved))
	}
}

func TestGenerateRecurringExpenses_OnlyTheCurrentMonth(t *testing.T) {
	// A do dia 30 não foi lançada em novembro; em 01/12 o dia 30 de dezembro ainda não chegou: nada é lançado,
	// nem novembro retroativo.
	f, repo := newRecurringFake(t, recurringPurchase(30))
	runRecurringAt(t, repo, time.Date(2026, 12, 1, 9, 0, 0, 0, saoPaulo(t)))
	if len(f.saved) != 0 {
		t.Fatalf("não retroage meses anteriores, got %d", len(f.saved))
	}
}

func TestGenerateRecurringExpenses_ShortMonthsFollowLastValidDay(t *testing.T) {
	loc := saoPaulo(t)
	day31, day30 := recurringPurchase(31), recurringPurchase(30)

	f, repo := newRecurringFake(t, day31, day30)
	runRecurringAt(t, repo, time.Date(2027, 2, 27, 12, 0, 0, 0, loc))
	if len(f.saved) != 0 {
		t.Fatalf("27/02 ainda é antes do alvo (28), got %d", len(f.saved))
	}

	runRecurringAt(t, repo, time.Date(2027, 2, 28, 0, 0, 5, 0, loc))
	if len(f.saved) != 2 {
		t.Fatalf("em 28/02 lança as do dia 30 e 31, got %d", len(f.saved))
	}

	// App fora em 28/02: não há dia depois do alvo em fevereiro; em 01/03 é outro mês e nada retroage.
	f, repo = newRecurringFake(t, day31)
	runRecurringAt(t, repo, time.Date(2027, 3, 1, 9, 0, 0, 0, loc))
	if len(f.saved) != 0 {
		t.Fatalf("fevereiro perdido não é lançado em março, got %d", len(f.saved))
	}

	// Abril (30 dias): em 30/04 a do dia 31 é lançada no próprio dia (alvo 30), sem carimbo retroativo.
	f, repo = newRecurringFake(t, day31)
	runRecurringAt(t, repo, time.Date(2027, 4, 30, 9, 0, 0, 0, loc))
	if len(f.saved) != 1 || f.saved[0].CreatedAt.Equal(time.Date(2027, 4, 30, 0, 0, 0, 0, loc)) {
		t.Fatalf("em 30/04 lança a do dia 31 no próprio dia: %+v", f.saved)
	}
}

func TestGenerateRecurringExpenses_MonthTurnInSaoPaulo(t *testing.T) {
	// 22:30 de 30/11 em São Paulo é 01:30Z de 01/12: tudo é de novembro.
	loc := saoPaulo(t)
	day1, day30 := recurringPurchase(1), recurringPurchase(30)
	f, repo := newRecurringFake(t, day1, day30)
	runRecurringAt(t, repo, time.Date(2026, 11, 30, 22, 30, 0, 0, loc))

	if len(f.saved) != 2 {
		t.Fatalf("deveria lançar a do dia 30 e recuperar a do dia 1, got %d", len(f.saved))
	}
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, p := range f.saved {
		if !p.ReferenceMonth.Equal(nov) {
			t.Errorf("reference_month = %v, esperado novembro", p.ReferenceMonth)
		}
		if p.PurchaseID == day1.ID && !p.CreatedAt.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, loc)) {
			t.Errorf("a do dia 1 recuperada deveria ter created_at em 01/11 local, got %v", p.CreatedAt)
		}
	}
}

func TestGenerateRecurringExpenses_WithoutDayLaunchesOncePerMonth(t *testing.T) {
	rec := recurringPurchase(1)
	rec.DayOfMonth = nil
	f, repo := newRecurringFake(t, rec)
	runRecurringAt(t, repo, time.Date(2026, 11, 17, 9, 0, 0, 0, saoPaulo(t)))
	runRecurringAt(t, repo, time.Date(2026, 11, 18, 0, 0, 5, 0, saoPaulo(t)))
	if len(f.saved) != 1 {
		t.Fatalf("sem dia: um lançamento por mês, got %d", len(f.saved))
	}
}

func TestGenerateRecurringExpenses_ErrorOnOneDoesNotStopTheOthers(t *testing.T) {
	a, b := recurringPurchase(5), recurringPurchase(5)
	var saved []uuid.UUID
	repo := &mockLedger{
		findActiveRecurringFn: func(context.Context) ([]domain.Purchase, error) { return []domain.Purchase{a, b}, nil },
		savePaymentFn: func(_ context.Context, p *domain.Payment) error {
			if p.PurchaseID == a.ID {
				return errors.New("falha")
			}
			saved = append(saved, p.PurchaseID)
			return nil
		},
	}
	runRecurringAt(t, repo, time.Date(2026, 11, 10, 9, 0, 0, 0, saoPaulo(t)))
	if len(saved) != 1 || saved[0] != b.ID {
		t.Fatalf("a segunda deveria ser lançada mesmo com erro na primeira: %v", saved)
	}
}

func TestLastValidDay_NormalDay(t *testing.T) {
	got := lastValidDay(2025, time.March, 15)
	if got != 15 {
		t.Errorf("esperava 15, got %d", got)
	}
}

func TestLastValidDay_DayExceedsMonth(t *testing.T) {
	got := lastValidDay(2025, time.February, 31)
	if got != 28 {
		t.Errorf("esperava 28 (último dia de fev/2025), got %d", got)
	}
}

func TestLastValidDay_LeapYear(t *testing.T) {
	got := lastValidDay(2024, time.February, 30)
	if got != 29 {
		t.Errorf("esperava 29 (último dia de fev/2024 bissexto), got %d", got)
	}
}

func TestLastValidDay_Day31InMonth30(t *testing.T) {
	got := lastValidDay(2025, time.April, 31)
	if got != 30 {
		t.Errorf("esperava 30 (último dia de abril), got %d", got)
	}
}
