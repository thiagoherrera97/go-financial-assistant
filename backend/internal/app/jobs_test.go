package app

import (
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestNextMidnight(t *testing.T) {
	sp, ny := mustLoc(t, "America/Sao_Paulo"), mustLoc(t, "America/New_York")
	cases := []struct {
		name string
		now  time.Time
		loc  *time.Location
		want time.Time
	}{
		{"meio do mês em UTC", time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC), time.UTC, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)},
		{"virada do mês em UTC", time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC), time.UTC, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)},
		{"virada do ano em UTC", time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC), time.UTC, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
		// 22:30 de 30/11 em São Paulo (01:30Z de 01/12): a próxima meia-noite local é 00:00 de 01/12 (03:00Z).
		{"São Paulo antes da virada", time.Date(2026, 12, 1, 1, 30, 0, 0, time.UTC), sp, time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)},
		// Instante UTC já no dia seguinte, mas ainda dia 30 em São Paulo: não pula uma meia-noite.
		{"São Paulo passada a meia-noite UTC", time.Date(2026, 12, 1, 0, 30, 0, 0, time.UTC), sp, time.Date(2026, 12, 1, 3, 0, 0, 0, time.UTC)},
		// Mudança de horário (EUA, 08/03/2026): o dia local tem 23 h e a data é montada pelo calendário.
		{"dia de início do horário de verão", time.Date(2026, 3, 8, 5, 30, 0, 0, time.UTC), ny, time.Date(2026, 3, 9, 4, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		if got := nextMidnight(tc.now, tc.loc); !got.Equal(tc.want) {
			t.Errorf("%s: got %v, esperado %v", tc.name, got.UTC(), tc.want.UTC())
		}
	}
}

func TestIsMonthStart(t *testing.T) {
	sp := mustLoc(t, "America/Sao_Paulo")
	cases := []struct {
		name string
		now  time.Time
		loc  *time.Location
		want bool
	}{
		{"00:30Z de 01/12 ainda é 30/11 em São Paulo (21:30)", time.Date(2026, 12, 1, 0, 30, 0, 0, time.UTC), sp, false},
		{"00:05 de 01/12 em São Paulo", time.Date(2026, 12, 1, 3, 5, 0, 0, time.UTC), sp, true},
		{"mesmo instante em UTC é dia 1", time.Date(2026, 12, 1, 0, 30, 0, 0, time.UTC), time.UTC, true},
		{"dia 2", time.Date(2026, 12, 2, 12, 0, 0, 0, time.UTC), sp, false},
	}
	for _, tc := range cases {
		if got := isMonthStart(tc.now, tc.loc); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}
