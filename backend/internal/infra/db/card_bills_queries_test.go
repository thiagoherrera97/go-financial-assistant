package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestCardBillsAndLines(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	due := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "cbv-card", ItemID: "cbv", Type: "CREDIT", Name: "Cartão fatura", Last4: "4321", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pg.Pool.Exec(context.Background(), `DELETE FROM payments WHERE account_id = $1`, card)
		pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card)
	})

	cardExpense(t, repo, pg, card, "cbv-1", "cbv mercado", 100, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))
	cardExpense(t, repo, pg, card, "cbv-2", "cbv farmácia", 30, time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC))
	cardExpense(t, repo, pg, card, "cbv-3", "cbv cinema", 7, time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC))
	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}
	// O banco fechou a fatura de 10/11 com 150: 20 a mais do que o app conhece (100 + 30).
	jan := time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC) // zerada e sem lançamento: some da lista
	if err := repo.SaveCardBills(ctx, card, []ports.ExternalBill{{ID: "cbv-bill-nov", DueDate: due, Total: 150}, {ID: "cbv-bill-jan", DueDate: jan, Total: 0}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.RebuildCardForecasts(ctx, 6, time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	bills, err := repo.CardBills(ctx, card)
	if err != nil {
		t.Fatal(err)
	}
	if len(bills) != 2 {
		t.Fatalf("esperava as faturas de nov e dez, got %+v", bills)
	}
	nov, dec := bills[0], bills[1]
	if nov.Due.Format(time.DateOnly) != "2026-11-10" || nov.BankTotal == nil || *nov.BankTotal != 150 ||
		nov.Detailed != 130 || nov.Gap != 20 || nov.Count != 2 || nov.Difference() != 20 {
		t.Errorf("novembro: %+v", nov)
	}
	if dec.Due.Format(time.DateOnly) != "2026-12-10" || dec.BankTotal != nil || dec.Detailed != 7 || dec.Count != 1 {
		t.Errorf("dezembro: %+v", dec)
	}

	lines, err := repo.CardBillLines(ctx, card, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("esperava 2 compras e o ajuste, got %+v", lines)
	}
	if lines[0].Description != "cbv farmácia" || lines[1].Description != "cbv mercado" || lines[0].Origin != domain.CardBillOriginBank {
		t.Errorf("compras da mais recente para a mais antiga: %+v", lines[:2])
	}
	if lines[2].Origin != domain.CardBillOriginGap || lines[2].Amount != 20 {
		t.Errorf("o ajuste vem por último: %+v", lines[2])
	}
}
