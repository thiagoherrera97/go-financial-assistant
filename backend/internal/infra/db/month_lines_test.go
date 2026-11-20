package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestMonthLines(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	due := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "ml-test-card", ItemID: "ml-test", Type: "CREDIT", Name: "CARTAO ML", Last4: "1111", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card) })

	cardExpense(t, repo, pg, card, "ml-t1", "ml-teste ifood", 80, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))
	desc := "ml-teste aluguel"
	p, _ := domain.NewPurchase(1100, &desc, domain.CategoryHousing, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
	pay := domain.NewPayment(p.ID, 1100, domain.PaymentStatusPending)
	day := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	pay.DueDate = &day
	if err := repo.SaveExternal(ctx, p, pay); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)
	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}

	lines, err := (&BudgetRepo{db: pg}).MonthLines(ctx, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.MonthLine{}
	for _, l := range lines {
		got[l.Label] = l
	}
	if l := got["ml-teste ifood"]; !l.Card || l.AccountName != "CARTAO ML" || l.Amount != 80 || l.Synthetic != "" || l.Key != "ml teste ifood" {
		t.Errorf("compra no cartão (fatura de novembro): %+v", l)
	}
	if l := got["ml-teste aluguel"]; l.Card || !l.Pending || l.AccountID != "" || l.Kind != "EXPENSE" {
		t.Errorf("aluguel sem conta: %+v", l)
	}
}
