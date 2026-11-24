package cycles

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

type fakeStore struct {
	saved  []domain.CycleStart
	writes int
	oldest time.Time
}

func (f *fakeStore) ReplaceCycles(_ context.Context, s []domain.CycleStart) error {
	f.saved, f.writes = s, f.writes+1
	return nil
}
func (f *fakeStore) LoadCycles(context.Context) ([]domain.CycleStart, error) { return f.saved, nil }
func (f *fakeStore) OldestPaymentDate(context.Context) (time.Time, bool, error) {
	return f.oldest, !f.oldest.IsZero(), nil
}

func day(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

func noCredits(context.Context, time.Time, time.Time) ([]domain.SalaryCredit, error) { return nil, nil }

func TestRefreshWritesCycles(t *testing.T) {
	store := &fakeStore{oldest: day(2026, 1, 5)}
	parts := []domain.SalaryPart{{Day: 15, Amount: 5200}, {Day: 30, Amount: 4366.62}}
	s := New(store, noCredits, func() []domain.SalaryPart { return parts }, nil, func() time.Time { return day(2026, 11, 21) })
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := s.Current().KeyOf(day(2026, 12, 14)); !got.Equal(day(2026, 11, 1)) {
		t.Errorf("KeyOf(2026-12-14) = %v", got)
	}
	if len(store.saved) == 0 || !store.saved[0].Key.Equal(day(2026, 1, 1)) {
		t.Errorf("ciclos gravados devem começar no mês do lançamento mais antigo: %v", store.saved)
	}
	// Sem mudança, não regrava.
	if err := s.Refresh(context.Background()); err != nil || store.writes != 1 {
		t.Errorf("segunda Refresh sem mudança: writes=%d err=%v", store.writes, err)
	}
}

func TestRefreshNoSalaryClears(t *testing.T) {
	store := &fakeStore{saved: []domain.CycleStart{{Key: day(2026, 11, 1), Start: day(2026, 11, 13)}}}
	s := New(store, noCredits, func() []domain.SalaryPart { return nil }, nil, func() time.Time { return day(2026, 11, 21) })
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.saved) != 0 || store.writes != 1 {
		t.Errorf("sem salário a tabela fica vazia: %v (writes=%d)", store.saved, store.writes)
	}
	if got := s.Current().KeyOf(day(2026, 12, 14)); !got.Equal(day(2026, 12, 1)) {
		t.Errorf("KeyOf sem salário = %v", got)
	}
}

func TestRefreshFixedDay(t *testing.T) {
	store := &fakeStore{oldest: day(2026, 1, 5)}
	rule := func() domain.CycleRule { return domain.CycleRule{Mode: domain.CycleFixedDay, Day: 5} }
	s := New(store, noCredits, func() []domain.SalaryPart { return nil }, rule, func() time.Time { return day(2026, 11, 21) })
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	// 5/dez/2026 é sábado: o ciclo de dezembro começa sexta, 4/dez; 3/dez ainda é novembro.
	if got := s.Current().KeyOf(day(2026, 12, 3)); !got.Equal(day(2026, 11, 1)) {
		t.Errorf("KeyOf(2026-12-03) = %v", got)
	}
	if got := s.Current().KeyOf(day(2026, 12, 4)); !got.Equal(day(2026, 12, 1)) {
		t.Errorf("KeyOf(2026-12-04) = %v", got)
	}
}

func TestRefreshCalendarIgnoresSalary(t *testing.T) {
	store := &fakeStore{oldest: day(2026, 1, 5)}
	parts := []domain.SalaryPart{{Day: 15, Amount: 1000}}
	rule := func() domain.CycleRule { return domain.CycleRule{Mode: domain.CycleCalendar} }
	s := New(store, noCredits, func() []domain.SalaryPart { return parts }, rule, func() time.Time { return day(2026, 11, 21) })
	if err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := s.Current().KeyOf(day(2026, 12, 14)); !got.Equal(day(2026, 12, 1)) {
		t.Errorf("calendário: KeyOf(2026-12-14) = %v", got)
	}
}
