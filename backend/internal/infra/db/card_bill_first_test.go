package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// Parcela futura que o banco manda com a data da compra e sem previsão de fatura (VivoEasy 8/12 da FREE VISA): cairia
// antes da 1/12 e vai para 7 meses depois da fatura dela.
func TestRecomputeCardBills_InstallmentNotBeforeFirst(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	due := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "first-test-card", ItemID: "first-test", Type: "CREDIT", Name: "Cartão série", Last4: "6360", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card) })

	first := cardExpense(t, repo, pg, card, "first-1", "first-teste assinatura (1/12)", 30, time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC))
	fourth := cardExpense(t, repo, pg, card, "first-4", "first-teste assinatura (4/12)", 30, time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC))
	eighth := cardExpense(t, repo, pg, card, "first-8", "first-teste assinatura (8/12)", 30, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
	other := cardExpense(t, repo, pg, card, "first-x", "first-teste outra (8/12)", 99, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))

	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}
	for p, w := range map[string]string{first.ID.String(): "2026-09-12", fourth.ID.String(): "2026-12-12", eighth.ID.String(): "2027-04-12", other.ID.String(): "2026-08-12"} {
		var got time.Time
		if err := pg.Pool.QueryRow(ctx, `SELECT bill_due_date FROM payments WHERE purchase_id = $1`, p).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != w {
			t.Errorf("%s: fatura %s, esperava %s", p, got.Format("2006-01-02"), w)
		}
	}
}
