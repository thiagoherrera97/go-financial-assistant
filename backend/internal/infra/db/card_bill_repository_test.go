package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func cardExpense(t *testing.T, repo *testRepo, pg *DB, account uuid.UUID, extID, desc string, amount float64, day time.Time) *domain.Purchase {
	t.Helper()
	p, err := domain.NewPurchase(amount, &desc, domain.CategoryFood, domain.PaymentMethodCreditCard, domain.PurchaseTypeSingle, "[open finance: teste | teste]")
	if err != nil {
		t.Fatal(err)
	}
	pay := domain.NewPayment(p.ID, amount, domain.PaymentStatusPaid)
	pay.DueDate, pay.PaidAt, pay.ExternalID, pay.AccountID = &day, &day, &extID, &account
	if err := repo.SaveExternal(context.Background(), p, pay); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)
	return p
}

func billDueOf(t *testing.T, pg *DB, purchaseID uuid.UUID) *time.Time {
	t.Helper()
	var due *time.Time
	if err := pg.Pool.QueryRow(context.Background(), `SELECT bill_due_date FROM payments WHERE purchase_id = $1`, purchaseID).Scan(&due); err != nil {
		t.Fatal(err)
	}
	return due
}

func TestRecomputeCardBills(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	due := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "bill-test-card", ItemID: "bill-test", Type: "CREDIT", Name: "Cartão teste", Last4: "9999", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card) })

	oct20 := cardExpense(t, repo, pg, card, "bill-t1", "bill-teste depois do fechamento", 100, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))
	nov03 := cardExpense(t, repo, pg, card, "bill-t2", "bill-teste antes do fechamento", 30, time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC))
	nov04 := cardExpense(t, repo, pg, card, "bill-t3", "bill-teste no fechamento", 7, time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC))
	parcela := cardExpense(t, repo, pg, card, "bill-t4", "bill-teste parcela (2/10)", 50, time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC))

	n, err := repo.RecomputeCardBills(ctx, 6)
	if err != nil {
		t.Fatal(err)
	}
	if n < 4 {
		t.Errorf("esperava ao menos 4 linhas calculadas, got %d", n)
	}
	want := map[*domain.Purchase]string{oct20: "2026-11-10", nov03: "2026-11-10", nov04: "2026-12-10", parcela: "2026-12-10"}
	for p, w := range want {
		got := billDueOf(t, pg, p.ID)
		if got == nil || got.Format("2006-01-02") != w {
			t.Errorf("%s: fatura esperada %s, got %v", *p.Description, w, got)
		}
	}

	// O mês do pagamento passa a ser o da fatura; o dia mostrado continua sendo o da compra.
	var nov float64
	if err := pg.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(pay.amount), 0) FROM payments pay JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.description LIKE 'bill-teste%' AND `+paymentMonth+` = '2026-11-01'`).Scan(&nov); err != nil {
		t.Fatal(err)
	}
	if nov != 130 {
		t.Errorf("novembro deveria somar 100 + 30 (fatura de 10/11), got %.2f", nov)
	}
	var day time.Time
	if err := pg.Pool.QueryRow(ctx, `SELECT `+txDate+` FROM payments pay WHERE pay.purchase_id = $1`, oct20.ID).Scan(&day); err != nil {
		t.Fatal(err)
	}
	if day.Format("2006-01-02") != "2026-10-20" {
		t.Errorf("o dia mostrado deveria continuar o da compra, got %s", day.Format("2006-01-02"))
	}

	// Idempotente: sem mudanças, nada é regravado.
	if n, err := repo.RecomputeCardBills(ctx, 6); err != nil || n != 0 {
		t.Errorf("segunda passada não deveria mudar nada, got %d, %v", n, err)
	}

	// Mudou o fechamento: a compra de 03/11 passa a cair depois dele (10 − 8 = 02/11) e vai para dezembro.
	if _, err := repo.RecomputeCardBills(ctx, 8); err != nil {
		t.Fatal(err)
	}
	if got := billDueOf(t, pg, nov03.ID); got == nil || got.Format("2006-01-02") != "2026-12-10" {
		t.Errorf("com 8 dias, 03/11 deveria ir para 10/12, got %v", got)
	}

	// O fechamento informado pelo banco vale mais que a configuração.
	closeDate := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	if _, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "bill-test-card", ItemID: "bill-test", Type: "CREDIT", Name: "Cartão teste", Last4: "9999", DueDate: &due, CloseDate: &closeDate}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RecomputeCardBills(ctx, 8); err != nil {
		t.Fatal(err)
	}
	if got := billDueOf(t, pg, nov04.ID); got == nil || got.Format("2006-01-02") != "2026-11-10" {
		t.Errorf("com fechamento do banco em 05/11, a compra de 04/11 deveria ficar em 10/11, got %v", got)
	}
}
