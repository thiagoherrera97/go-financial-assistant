package db

import (
	"context"
	"fmt"
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

func TestNotesSpreadAndLearned(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	notes := NewNotesRepo(pg)
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "nl-card", ItemID: "nl", Type: "CREDIT", Name: "Cartão NL"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card) })
	noteOf := func(p *domain.Purchase) string {
		var n *string
		pg.Pool.QueryRow(ctx, `SELECT note FROM payments WHERE purchase_id = $1`, p.ID).Scan(&n) //nolint:errcheck
		if n == nil {
			return "<nula>"
		}
		return *n
	}
	payOf := func(p *domain.Purchase) uuid.UUID {
		var id uuid.UUID
		pg.Pool.QueryRow(ctx, `SELECT id FROM payments WHERE purchase_id = $1`, p.ID).Scan(&id) //nolint:errcheck
		return id
	}
	d := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)

	// Parcelas: a nota de uma vale para as outras da mesma compra (mesmo valor e total), não para outra compra.
	p3 := cardExpense(t, repo, pg, card, "nl-1", "NLTESTE*Canaa Filmes (3/10)", 153, d)
	p4 := cardExpense(t, repo, pg, card, "nl-2", "NLTESTE*Canaa Filmes (4/10)", 153, d.AddDate(0, 1, 0))
	other := cardExpense(t, repo, pg, card, "nl-3", "NLTESTE*Canaa Filmes (2/5)", 80, d)
	if ok, err := notes.SetTransactionNote(ctx, payOf(p3), "Insufilm"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if noteOf(p4) != "Insufilm" || noteOf(other) != "<nula>" {
		t.Errorf("parcelas: 4/10 = %q, outra compra = %q", noteOf(p4), noteOf(other))
	}
	// Mudar a nota numa parcela muda nas outras.
	if _, err := notes.SetTransactionNote(ctx, payOf(p4), "Insulfilm do carro"); err != nil {
		t.Fatal(err)
	}
	if noteOf(p3) != "Insulfilm do carro" {
		t.Errorf("parcela 3/10 = %q", noteOf(p3))
	}

	// Mesmo comércio e valor: as sem nota herdam; valor diferente, não.
	e1 := cardExpense(t, repo, pg, card, "nl-4", "NLTESTE ESTAPAR MONO SEDE", 25, d)
	e2 := cardExpense(t, repo, pg, card, "nl-5", "NLTESTE ESTAPAR MONO SEDE", 25, d.AddDate(0, 0, 1))
	e3 := cardExpense(t, repo, pg, card, "nl-6", "NLTESTE ESTAPAR MONO SEDE", 40, d.AddDate(0, 0, 2))
	if _, err := notes.SetTransactionNote(ctx, payOf(e1), "Estacionamento trabalho"); err != nil {
		t.Fatal(err)
	}
	if noteOf(e2) != "Estacionamento trabalho" || noteOf(e3) != "<nula>" {
		t.Errorf("aprendida: e2 = %q, e3 = %q", noteOf(e2), noteOf(e3))
	}
	// A que chega depois herda na sincronização.
	e4 := cardExpense(t, repo, pg, card, "nl-7", "NLTESTE ESTAPAR MONO SEDE", 25, d.AddDate(0, 0, 3))
	if _, err := repo.ApplyPendingNotes(ctx); err != nil {
		t.Fatal(err)
	}
	if noteOf(e4) != "Estacionamento trabalho" {
		t.Errorf("nova = %q", noteOf(e4))
	}
	// Apagada pela pessoa continua apagada.
	if _, err := notes.SetTransactionNote(ctx, payOf(e4), ""); err != nil {
		t.Fatal(err)
	}
	repo.ApplyPendingNotes(ctx) //nolint:errcheck
	if noteOf(e4) != "" {
		t.Errorf("apagada voltou: %q", noteOf(e4))
	}
	// Uma nota diferente no meio não atrapalha: a mais usada (3 de 4, sem diferenciar maiúsculas) é herdada.
	for i, n := range []string{"ESTACIONAMENTO TRABALHO", "Estacionamento 18/02"} {
		e := cardExpense(t, repo, pg, card, fmt.Sprintf("nl-m%d", i), "NLTESTE ESTAPAR MONO SEDE", 25, d.AddDate(0, 0, 10+i))
		if _, err := notes.SetTransactionNote(ctx, payOf(e), n); err != nil {
			t.Fatal(err)
		}
	}
	e6 := cardExpense(t, repo, pg, card, "nl-9", "NLTESTE ESTAPAR MONO SEDE", 25, d.AddDate(0, 0, 20))
	repo.ApplyPendingNotes(ctx) //nolint:errcheck
	if noteOf(e6) != "Estacionamento trabalho" {
		t.Errorf("maioria: %q", noteOf(e6))
	}
	pg.Pool.Exec(ctx, `UPDATE payments SET note = '' WHERE purchase_id = $1`, e6.ID) //nolint:errcheck

	// Notas divididas no mesmo comércio e valor: dúvida, nada é herdado.
	if _, err := notes.SetTransactionNote(ctx, payOf(e2), "Médico"); err != nil {
		t.Fatal(err)
	}
	e5 := cardExpense(t, repo, pg, card, "nl-8", "NLTESTE ESTAPAR MONO SEDE", 25, d.AddDate(0, 0, 4))
	repo.ApplyPendingNotes(ctx) //nolint:errcheck
	if noteOf(e5) != "<nula>" {
		t.Errorf("com dúvida herdou: %q", noteOf(e5))
	}
}
