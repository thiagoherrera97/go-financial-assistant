package domain

import (
	"testing"
	"time"
)

var cycleParts = []SalaryPart{{Day: 15, Amount: 5200}, {Day: 30, Amount: 4366.62}}

func TestBuildCyclesHolidayStart(t *testing.T) {
	got := BuildCycles(cycleParts, nil, ymd(2026, 11, 5), ymd(2026, 12, 20))
	want := []CycleStart{{Key: ymd(2026, 11, 1), Start: ymd(2026, 11, 13)}, {Key: ymd(2026, 12, 1), Start: ymd(2026, 12, 15)}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if !got[i].Key.Equal(want[i].Key) || !got[i].Start.Equal(want[i].Start) {
			t.Errorf("[%d] got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestBuildCyclesEarlyCredit(t *testing.T) {
	credits := []SalaryCredit{{Date: ymd(2026, 12, 11), Amount: 5200}}
	got := BuildCycles(cycleParts, credits, ymd(2026, 11, 1), ymd(2026, 12, 1))
	if len(got) != 2 || !got[1].Start.Equal(ymd(2026, 12, 11)) {
		t.Fatalf("got %v, want December starting 2026-12-11", got)
	}
}

func TestBuildCyclesNoSalary(t *testing.T) {
	if got := BuildCycles(nil, nil, ymd(2026, 11, 1), ymd(2026, 12, 1)); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func testCycles() Cycles {
	return NewCycles([]CycleStart{{Key: ymd(2026, 11, 1), Start: ymd(2026, 11, 13)}, {Key: ymd(2026, 12, 1), Start: ymd(2026, 12, 15)}})
}

func TestCyclesKeyOf(t *testing.T) {
	c := testCycles()
	cases := []struct{ day, want time.Time }{
		{ymd(2026, 12, 14), ymd(2026, 11, 1)},
		{ymd(2026, 12, 15), ymd(2026, 12, 1)},
		{ymd(2026, 11, 12), ymd(2026, 11, 1)},
	}
	for _, tc := range cases {
		if got := c.KeyOf(tc.day); !got.Equal(tc.want) {
			t.Errorf("KeyOf(%v) = %v, want %v", tc.day, got, tc.want)
		}
	}
	if got := NewCycles(nil).KeyOf(ymd(2026, 12, 14)); !got.Equal(ymd(2026, 12, 1)) {
		t.Errorf("empty KeyOf = %v, want 2026-12-01", got)
	}
}

func TestCyclesBounds(t *testing.T) {
	start, end := testCycles().Bounds(ymd(2026, 11, 1))
	if !start.Equal(ymd(2026, 11, 13)) || !end.Equal(ymd(2026, 12, 14)) {
		t.Errorf("Bounds = %v..%v, want 2026-11-13..2026-12-14", start, end)
	}
	start, end = NewCycles(nil).Bounds(ymd(2026, 11, 1))
	if !start.Equal(ymd(2026, 11, 1)) || !end.Equal(ymd(2026, 11, 30)) {
		t.Errorf("empty Bounds = %v..%v, want 2026-11-01..2026-11-30", start, end)
	}
}

func TestCyclesDayOf(t *testing.T) {
	n, total := testCycles().DayOf(ymd(2026, 11, 13))
	if n != 1 || total != 32 {
		t.Errorf("DayOf = %d/%d, want 1/32", n, total)
	}
}
