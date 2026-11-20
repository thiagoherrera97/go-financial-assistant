package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func TestNotes(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	notes := NewNotesRepo(pg)
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM pending_notes`) })
	bank, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "nt-bank", ItemID: "nt", Type: "BANK", Name: "Banco NT"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, bank) })

	d10 := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	cardExpense(t, repo, pg, bank, "nt-1", "nt-teste Pix recebido MONICA", 5400, d10)
	cardExpense(t, repo, pg, bank, "nt-2", "nt-teste outra", 5400, d10.AddDate(0, 0, -20))

	found, err := notes.NoteCandidates(ctx, 5400, d10, d10)
	if err != nil || len(found) != 1 || found[0].Description != "nt-teste Pix recebido MONICA" {
		t.Fatalf("candidatas = %+v, %v", found, err)
	}
	if ok, err := notes.SetTransactionNote(ctx, found[0].PaymentID, "insulfilm"); err != nil || !ok {
		t.Fatal(err)
	}
	// A busca de Transações acha pela nota.
	txs, _, err := repo.DashboardRepo.Transactions(ctx, domain.TransactionFilter{Search: "insulf", Limit: 10})
	if err != nil || len(txs) != 1 || txs[0].Note != "insulfilm" {
		t.Errorf("busca pela nota = %+v, %v", txs, err)
	}

	// Nota que chega antes da transação: espera e é aplicada quando o banco a manda (2 dias de diferença).
	if err := notes.SavePendingNote(ctx, 393, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), "jantar pizza"); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.ApplyPendingNotes(ctx); err != nil || n != 0 {
		t.Errorf("sem transação ainda: %d %v", n, err)
	}
	pizza := cardExpense(t, repo, pg, bank, "nt-3", "nt-teste VILLA D ALDEA", 393, time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC))
	if n, err := repo.ApplyPendingNotes(ctx); err != nil || n != 1 {
		t.Fatalf("deveria aplicar 1: %d %v", n, err)
	}
	var note string
	pg.Pool.QueryRow(ctx, `SELECT note FROM payments WHERE purchase_id = $1`, pizza.ID).Scan(&note) //nolint:errcheck
	var left int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM pending_notes`).Scan(&left) //nolint:errcheck
	if note != "jantar pizza" || left != 0 {
		t.Errorf("nota = %q, pendentes = %d", note, left)
	}
}

func TestDeleteManualTransaction(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	desc := "dm-teste almoço"
	manual, _ := domain.NewPurchase(30, &desc, domain.CategoryFood, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
	pay := domain.NewPayment(manual.ID, 30, domain.PaymentStatusPaid)
	if err := repo.Save(ctx, manual, []domain.Payment{*pay}); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, manual)
	bank, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "dm-bank", ItemID: "dm", Type: "BANK", Name: "Banco DM"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, bank) })
	ext := cardExpense(t, repo, pg, bank, "dm-1", "dm-teste do banco", 30, time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC))

	var extPay uuid.UUID
	pg.Pool.QueryRow(ctx, `SELECT id FROM payments WHERE purchase_id = $1`, ext.ID).Scan(&extPay) //nolint:errcheck
	if ok, err := repo.DashboardRepo.DeleteManualTransaction(ctx, extPay); err != nil || ok {
		t.Errorf("lançamento do banco não pode ser apagado: %v %v", ok, err)
	}
	if ok, err := repo.DashboardRepo.DeleteManualTransaction(ctx, pay.ID); err != nil || !ok {
		t.Fatalf("manual deveria ser apagado: %v %v", ok, err)
	}
	var n int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM purchases WHERE id = $1`, manual.ID).Scan(&n) //nolint:errcheck
	if n != 0 {
		t.Error("a compra manual continuou no banco")
	}
}
