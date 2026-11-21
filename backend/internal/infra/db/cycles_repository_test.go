package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func cycDay(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

var novDecCycles = []domain.CycleStart{{Key: cycDay(2026, 11, 1), Start: cycDay(2026, 11, 13)}, {Key: cycDay(2026, 12, 1), Start: cycDay(2026, 12, 15)}}

// withCycles grava os ciclos para o teste e devolve os que havia no fim.
func withCycles(t *testing.T, repo *CycleRepo, starts []domain.CycleStart) {
	t.Helper()
	ctx := context.Background()
	before, err := repo.LoadCycles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceCycles(ctx, starts); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.ReplaceCycles(context.Background(), before) })
}

func TestReplaceCyclesRoundTrip(t *testing.T) {
	_, pg := newTestRepo(t)
	repo := NewCycleRepo(pg)
	withCycles(t, repo, novDecCycles)
	got, err := repo.LoadCycles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Start.Equal(cycDay(2026, 11, 13)) || !got[1].Key.Equal(cycDay(2026, 12, 1)) {
		t.Fatalf("got %v", got)
	}
	if err := repo.ReplaceCycles(context.Background(), novDecCycles[:1]); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.LoadCycles(context.Background()); len(got) != 1 {
		t.Fatalf("depois de regravar com 1: %v", got)
	}
}

func dateQuery(t *testing.T, pg *DB, sql string, args ...any) time.Time {
	t.Helper()
	var d time.Time
	if err := pg.Pool.QueryRow(context.Background(), sql, args...).Scan(&d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCycleOfFallback(t *testing.T) {
	_, pg := newTestRepo(t)
	withCycles(t, NewCycleRepo(pg), nil)
	if got := dateQuery(t, pg, `SELECT cycle_of('2026-12-14')`); !got.Equal(cycDay(2026, 12, 1)) {
		t.Errorf("cycle_of sem ciclos = %v", got)
	}
}

func TestCycleOfWithCycles(t *testing.T) {
	_, pg := newTestRepo(t)
	withCycles(t, NewCycleRepo(pg), novDecCycles)
	cases := map[string]time.Time{
		`SELECT cycle_of('2026-12-14')`:         cycDay(2026, 11, 1),
		`SELECT cycle_of('2026-11-12')`:         cycDay(2026, 11, 1),
		`SELECT cycle_next_start('2026-11-01')`: cycDay(2026, 12, 15),
		`SELECT cycle_next_start('2027-03-01')`: cycDay(2027, 4, 1),
	}
	for q, want := range cases {
		if got := dateQuery(t, pg, q); !got.Equal(want) {
			t.Errorf("%s = %v, want %v", q, got, want)
		}
	}
}

func TestPaymentMonthReferenceMonth(t *testing.T) {
	repo, pg := newTestRepo(t)
	withCycles(t, NewCycleRepo(pg), novDecCycles)
	ctx := context.Background()
	save := func(desc string, set func(*domain.Payment)) *domain.Purchase {
		p, _ := domain.NewPurchase(100, &desc, domain.CategoryHousing, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
		pay := domain.NewPayment(p.ID, 100, domain.PaymentStatusPaid)
		set(pay)
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		cleanup(t, pg, p)
		return p
	}
	ref, pix, cardDay := cycDay(2026, 11, 1), cycDay(2026, 12, 10), cycDay(2026, 12, 20)
	financing := save("cyc financiamento", func(p *domain.Payment) { p.ReferenceMonth = &ref })
	transfer := save("cyc pix", func(p *domain.Payment) { p.PaidAt = &pix })
	card := save("cyc cartao", func(p *domain.Payment) { p.DueDate, p.PaidAt = &cardDay, &cardDay })
	if _, err := pg.Pool.Exec(ctx, `UPDATE payments SET bill_due_date = '2027-01-10' WHERE purchase_id = $1`, card.ID); err != nil {
		t.Fatal(err)
	}
	month := func(p *domain.Purchase) time.Time {
		return dateQuery(t, pg, `SELECT `+paymentMonth+` FROM payments pay WHERE pay.purchase_id = $1`, p.ID)
	}
	if got := month(financing); !got.Equal(cycDay(2026, 11, 1)) {
		t.Errorf("financiamento (reference_month 2026-11) = %v", got)
	}
	if got := month(transfer); !got.Equal(cycDay(2026, 11, 1)) {
		t.Errorf("pix em 2026-12-10 = %v", got)
	}
	if got := month(card); !got.Equal(cycDay(2026, 12, 1)) {
		t.Errorf("cartão com fatura 2027-01-10 = %v", got)
	}
}
