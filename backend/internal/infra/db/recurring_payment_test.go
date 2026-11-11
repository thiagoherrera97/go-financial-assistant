package db

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// newRecurring grava uma recorrente ativa (dia 5) sem pagamentos.
func newRecurring(t *testing.T, repo *testRepo, pg *DB) *domain.Purchase {
	t.Helper()
	desc := "teste-h4-recorrente"
	p, err := domain.NewPurchase(55, &desc, domain.CategoryEntertainment, domain.PaymentMethodCreditCard, domain.PurchaseTypeRecurring, "teste")
	if err != nil {
		t.Fatal(err)
	}
	day := 5
	p.DayOfMonth = &day
	if err := repo.Save(context.Background(), p, nil); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)
	return p
}

func recurringPayment(p *domain.Purchase, month time.Time) *domain.Payment {
	pay := domain.NewPayment(p.ID, p.TotalAmount, domain.PaymentStatusPaid)
	pay.ReferenceMonth = &month
	return pay
}

func countMonthPayments(t *testing.T, pg *DB, p *domain.Purchase, month time.Time) int {
	t.Helper()
	var n int
	if err := pg.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM payments WHERE purchase_id = $1 AND reference_month = $2`, p.ID, month).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Várias execuções ao mesmo tempo (subida, meia-noite, outra instância) gravam um único pagamento no mês.
func TestSaveRecurringPaymentIfMissing_ConcurrentCallsInsertOnce(t *testing.T) {
	repo, pg := newTestRepo(t)
	p := newRecurring(t, repo, pg)
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	const callers = 8
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		saved int
		errs  []error
	)
	start := make(chan struct{})
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, err := repo.SaveRecurringPaymentIfMissing(context.Background(), recurringPayment(p, month))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
			}
			if ok {
				saved++
			}
		}()
	}
	close(start)
	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("erros: %v", errs)
	}
	if saved != 1 {
		t.Errorf("só uma chamada deveria gravar, got %d", saved)
	}
	if n := countMonthPayments(t, pg, p, month); n != 1 {
		t.Errorf("o mês deveria ter 1 pagamento, got %d", n)
	}
}

func TestSaveRecurringPaymentIfMissing_CancelledPaymentBlocks(t *testing.T) {
	repo, pg := newTestRepo(t)
	p := newRecurring(t, repo, pg)
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	cancelled := recurringPayment(p, month)
	cancelled.Status, cancelled.PaidAt = domain.PaymentStatusCancelled, nil
	if err := repo.SavePayment(context.Background(), cancelled); err != nil {
		t.Fatal(err)
	}

	ok, err := repo.SaveRecurringPaymentIfMissing(context.Background(), recurringPayment(p, month))
	if err != nil || ok {
		t.Fatalf("mês com pagamento CANCELLED não recebe outro: ok=%v err=%v", ok, err)
	}
	// Outro mês continua livre.
	dec := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	if ok, err := repo.SaveRecurringPaymentIfMissing(context.Background(), recurringPayment(p, dec)); err != nil || !ok {
		t.Fatalf("dezembro deveria ser gravado: ok=%v err=%v", ok, err)
	}
}

func TestSaveRecurringPaymentIfMissing_InactiveRecurringIsSkipped(t *testing.T) {
	repo, pg := newTestRepo(t)
	p := newRecurring(t, repo, pg)
	if _, err := pg.Pool.Exec(context.Background(), `UPDATE purchases SET is_active = FALSE, cancelled_at = NOW() WHERE id = $1`, p.ID); err != nil {
		t.Fatal(err)
	}
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	ok, err := repo.SaveRecurringPaymentIfMissing(context.Background(), recurringPayment(p, month))
	if err != nil || ok {
		t.Fatalf("recorrente cancelada não recebe pagamento: ok=%v err=%v", ok, err)
	}
	if n := countMonthPayments(t, pg, p, month); n != 0 {
		t.Errorf("got %d pagamentos", n)
	}
	actives, err := repo.FindActiveRecurring(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actives {
		if a.ID == p.ID {
			t.Error("recorrente cancelada não deveria estar entre as ativas")
		}
	}
}

func TestSaveRecurringPaymentIfMissing_KeepsTheScheduledStamp(t *testing.T) {
	repo, pg := newTestRepo(t)
	p := newRecurring(t, repo, pg)
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	pay := recurringPayment(p, month)
	day5 := time.Date(2026, 11, 5, 3, 0, 0, 0, time.UTC) // 00:00 de 05/11 em São Paulo
	pay.CreatedAt, pay.PaidAt = day5, &day5
	if ok, err := repo.SaveRecurringPaymentIfMissing(context.Background(), pay); err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	var created time.Time
	var due *time.Time
	if err := pg.Pool.QueryRow(context.Background(), `SELECT created_at, due_date FROM payments WHERE id = $1`, pay.ID).Scan(&created, &due); err != nil {
		t.Fatal(err)
	}
	if !created.Equal(day5) || due != nil {
		t.Errorf("created_at=%v due_date=%v", created, due)
	}
}
