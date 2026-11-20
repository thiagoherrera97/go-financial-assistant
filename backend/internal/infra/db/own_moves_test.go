package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func TestReclassifyOwnMoves(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc := func(id, typ, name string) uuid.UUID {
		got, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: id, ItemID: "om-test", Type: domain.AccountType(typ), Name: name})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, got) })
		return got
	}
	bank := acc("om-bank", "BANK", "Mercado Pago OM")
	acc("om-card", "CREDIT", "SANTANDER OM UNIQUE VISA")
	acc("om-card2", "CREDIT", "Mercado Pago OM") // mesmo nome da conta corrente: não serve para casar
	day := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	save := func(ext, desc string, kind domain.PurchaseKind, amount float64) uuid.UUID {
		var p *domain.Purchase
		if kind == domain.KindIncome {
			p, _ = domain.NewIncome(amount, &desc, domain.CategoryOther, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
		} else {
			p, _ = domain.NewPurchase(amount, &desc, domain.CategoryBills, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
		}
		pay := domain.NewPayment(p.ID, amount, domain.PaymentStatusPaid)
		pay.DueDate, pay.ExternalID, pay.AccountID = &day, &ext, &bank
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		cleanup(t, pg, p)
		return p.ID
	}
	reservado := save("om-1", "Dinheiro reservado Dívidas mensais", domain.KindExpense, 1000)
	retirado := save("om-2", "Dinheiro retirado Dívidas mensais", domain.KindIncome, 900)
	fatura := save("om-3", "Pagamento de conta SANTANDER OM UNIQUE VISA", domain.KindExpense, 8583.09)
	parcela := save("om-4", "Pagamento de parcela Empréstimos Mercado Pago OM", domain.KindExpense, 91.36)

	savings, cards, err := repo.ReclassifyOwnMoves(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if savings != 2 || cards != 1 {
		t.Errorf("esperava 2 da caixinha e 1 fatura, got %d e %d", savings, cards)
	}
	read := func(id uuid.UUID) (kind, dir, status string) {
		var d *string
		pg.Pool.QueryRow(ctx, `SELECT p.kind, p.transfer_direction, pay.status FROM purchases p JOIN payments pay ON pay.purchase_id = p.id WHERE p.id = $1`, id).Scan(&kind, &d, &status) //nolint:errcheck
		if d != nil {
			dir = *d
		}
		return
	}
	if k, d, _ := read(reservado); k != "TRANSFER" || d != "OUT" {
		t.Errorf("reservado = %s %s", k, d)
	}
	if k, d, _ := read(retirado); k != "TRANSFER" || d != "IN" {
		t.Errorf("retirado = %s %s", k, d)
	}
	if _, _, s := read(fatura); s != "CANCELLED" {
		t.Errorf("pagamento da fatura do cartão deveria ser cancelado, got %s", s)
	}
	if k, _, s := read(parcela); k != "EXPENSE" || s != "PAID" {
		t.Errorf("parcela de empréstimo não é fatura: %s %s", k, s)
	}
	if s, c, err := repo.ReclassifyOwnMoves(ctx); err != nil || s != 0 || c != 0 {
		t.Errorf("segunda passada não muda nada: %d %d %v", s, c, err)
	}
}
