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

	fidcDesc, aplicDesc := "Pagamento com Pix AIBR FIDC RESP LTDA", "Aplicação CDB"
	transfer := func(ext string, desc *string) uuid.UUID {
		p, _ := domain.NewTransfer(48.54, desc, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw", domain.TransferDirectionOut)
		pay := domain.NewPayment(p.ID, 48.54, domain.PaymentStatusPaid)
		pay.DueDate, pay.ExternalID, pay.AccountID = &day, &ext, &bank
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		cleanup(t, pg, p)
		return p.ID
	}
	fidc := transfer("om-5", &fidcDesc)
	aplic := transfer("om-6", &aplicDesc)

	savings, cards, funds, err := repo.ReclassifyOwnMoves(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if savings != 2 || cards != 1 || funds != 1 {
		t.Errorf("esperava 2 da caixinha, 1 fatura e 1 pagamento, got %d, %d e %d", savings, cards, funds)
	}
	read := func(id uuid.UUID) (kind, dir, status string) {
		var d *string
		var cat string
		pg.Pool.QueryRow(ctx, `SELECT p.kind, p.transfer_direction, pay.status, p.category FROM purchases p JOIN payments pay ON pay.purchase_id = p.id WHERE p.id = $1`, id).Scan(&kind, &d, &status, &cat) //nolint:errcheck
		if kind == "TRANSFER" && cat == "RESERVE" {
			kind = "RESERVE"
		}
		if d != nil {
			dir = *d
		}
		return
	}
	if k, d, _ := read(reservado); k != "RESERVE" || d != "OUT" {
		t.Errorf("reservado = %s %s", k, d)
	}
	if k, d, _ := read(retirado); k != "RESERVE" || d != "IN" {
		t.Errorf("retirado = %s %s", k, d)
	}
	if _, _, s := read(fatura); s != "CANCELLED" {
		t.Errorf("pagamento da fatura do cartão deveria ser cancelado, got %s", s)
	}
	if k, _, s := read(parcela); k != "EXPENSE" || s != "PAID" {
		t.Errorf("parcela de empréstimo não é fatura: %s %s", k, s)
	}
	if k, d, _ := read(fidc); k != "EXPENSE" || d != "" {
		t.Errorf("pagamento com Pix para FIDC = %s %s", k, d)
	}
	if k, _, _ := read(aplic); k != "TRANSFER" {
		t.Errorf("aplicação de verdade continua aplicação: %s", k)
	}
	if s, c, f, err := repo.ReclassifyOwnMoves(ctx); err != nil || s != 0 || c != 0 || f != 0 {
		t.Errorf("segunda passada não muda nada: %d %d %d %v", s, c, f, err)
	}
}

func TestCancelRefundedCharges(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "rf-card", ItemID: "rf-test", Type: domain.AccountType("CREDIT"), Name: "Cartão RF"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card) })
	dec, jan := time.Date(1999, 12, 10, 0, 0, 0, 0, time.UTC), time.Date(2000, 1, 10, 0, 0, 0, 0, time.UTC)
	n := 0
	save := func(desc string, amount float64, day, bill time.Time) uuid.UUID {
		t.Helper()
		n++
		ext := fmt.Sprintf("rf-%d", n)
		var p *domain.Purchase
		if amount < 0 {
			p, _ = domain.NewCardRefund(-amount, &desc, domain.CategoryOther, "raw")
		} else {
			p, _ = domain.NewPurchase(amount, &desc, domain.CategoryOther, domain.PaymentMethodCreditCard, domain.PurchaseTypeSingle, "raw")
		}
		pay := domain.NewPayment(p.ID, amount, domain.PaymentStatusPending)
		pay.DueDate, pay.ExternalID, pay.AccountID = &day, &ext, &card
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		if _, err := pg.Pool.Exec(ctx, `UPDATE payments SET bill_due_date = $2 WHERE purchase_id = $1`, p.ID, bill); err != nil {
			t.Fatal(err)
		}
		cleanup(t, pg, p)
		return p.ID
	}
	charge := save("RF RECARGA", 150.69, dec, dec)
	refund := save("RF RECARGA", -150.69, dec.AddDate(0, 0, 1), dec) // mesma fatura: os dois somem
	real := save("RF RECARGA", 29.97, dec, dec)                      // a recarga de verdade fica
	twice1 := save("RF POSTO", 100, dec, dec)                        // duas cobranças, um estorno:
	twice2 := save("RF POSTO", 100, dec.AddDate(0, 0, 2), dec)       // só um par some
	save("RF POSTO", -100, dec.AddDate(0, 0, 3), dec)
	crossC := save("RF HOTEL", 300, dec, dec) // faturas diferentes: os dois ficam
	crossR := save("RF HOTEL", -300, jan, jan)
	other := save("RF OUTRA LOJA", -150.69, dec, dec) // outra descrição não casa

	got, err := repo.CancelRefundedCharges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != 4 {
		t.Errorf("dois pares = 4 lançamentos cancelados, veio %d", got)
	}
	status := func(id uuid.UUID) string {
		var s string
		if err := pg.Pool.QueryRow(ctx, `SELECT status FROM payments WHERE purchase_id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	for name, c := range map[string]struct {
		id   uuid.UUID
		want string
	}{
		"cobrança": {charge, "CANCELLED"}, "estorno": {refund, "CANCELLED"}, "recarga de verdade": {real, "PENDING"},
		"primeira do posto": {twice1, "CANCELLED"}, "segunda do posto": {twice2, "PENDING"},
		"hotel em dezembro": {crossC, "PENDING"}, "estorno do hotel em janeiro": {crossR, "PENDING"}, "outra loja": {other, "PENDING"},
	} {
		if s := status(c.id); s != c.want {
			t.Errorf("%s: %s, queria %s", name, s, c.want)
		}
	}
	if again, _ := repo.CancelRefundedCharges(ctx); again != 0 {
		t.Errorf("rodar de novo não cancela mais nada: %d", again)
	}
}
