package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"strings"
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

func TestMonthAPIPeriod(t *testing.T) {
	cyc := domain.NewCycles([]domain.CycleStart{
		{Key: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Start: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)},
		{Key: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), Start: time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)},
	})
	r := &fakeReader{monthLines: []domain.MonthLine{{Key: "salario", Label: "LIQUIDO", Category: "SALARY", Kind: "INCOME", Amount: 5200}}}
	s := NewServer(0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.SetCycles(func() domain.Cycles { return cyc })
	if err := s.MountAPI(StaticPassword(testPassword), r); err != nil {
		t.Fatal(err)
	}
	rec := do(s, "GET", "/api/month?month=2026-11", "", nil, login(t, s))
	var got struct {
		StartsOn  string   `json:"startsOn"`
		EndsOn    string   `json:"endsOn"`
		Carryover *float64 `json:"carryover"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.StartsOn != "2026-11-13" || got.EndsOn != "2026-12-14" || got.Carryover == nil || *got.Carryover != 5200 {
		t.Errorf("%s", rec.Body.String())
	}
	for _, path := range []string{"/api/summary?month=2026-11", "/api/budget?month=2026-11"} {
		rec := do(s, "GET", path, "", nil, login(t, s))
		if !strings.Contains(rec.Body.String(), `"startsOn":"2026-11-13"`) {
			t.Errorf("%s sem o período: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestMeReportsCurrentCycle(t *testing.T) {
	cyc := domain.NewCycles([]domain.CycleStart{
		{Key: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Start: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)},
		{Key: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), Start: time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)},
	})
	s := NewServer(0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.SetCycles(func() domain.Cycles { return cyc })
	if err := s.mountAPI(StaticPassword(testPassword), &fakeReader{}, func() time.Time { return time.Date(2026, 12, 14, 12, 0, 0, 0, time.UTC) }); err != nil {
		t.Fatal(err)
	}
	rec := do(s, "GET", "/api/me", "", nil, login(t, s))
	want := `"cycle":{"endsOn":"2026-12-14","month":"2026-11","startsOn":"2026-11-13"}`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("/api/me = %s, want %s", rec.Body.String(), want)
	}
}
