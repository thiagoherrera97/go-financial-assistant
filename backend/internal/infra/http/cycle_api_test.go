package httpserver

import (
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func TestMonthStartIsCycle(t *testing.T) {
	cyc := domain.NewCycles([]domain.CycleStart{
		{Key: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Start: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)},
		{Key: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), Start: time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)},
	})
	a := &api{now: func() time.Time { return time.Date(2026, 12, 14, 12, 0, 0, 0, time.UTC) }, cycles: func() domain.Cycles { return cyc }}
	if got := a.monthStart(); !got.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("monthStart em 14/dez = %v, want o ciclo de novembro", got)
	}
	a.cycles = nil
	if got := a.monthStart(); !got.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("sem ciclos, monthStart = %v, want dezembro", got)
	}
}
