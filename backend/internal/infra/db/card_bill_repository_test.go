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

// A fatura informada pelo banco (billId) vence a estimativa do fechamento; estorno entra negativo e abate o mês.
func TestRecomputeCardBills_BankBillAndRefund(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	due := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "billid-test-card", ItemID: "billid-test", Type: "CREDIT", Name: "Cartão billid", Last4: "8888", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card) })
	if err := repo.SaveCardBills(ctx, card, []ports.ExternalBill{{ID: "billid-out", DueDate: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Total: 50}}); err != nil {
		t.Fatal(err)
	}

	// Compra em 05/10: pela estimativa (fecha 04/10) iria para novembro; o banco a pôs na fatura de outubro.
	day := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	desc := "billid-teste compra"
	p, _ := domain.NewPurchase(80, &desc, domain.CategoryShopping, domain.PaymentMethodCreditCard, domain.PurchaseTypeSingle, "[open finance: x | x]")
	pay := domain.NewPayment(p.ID, 80, domain.PaymentStatusPaid)
	ext, bill := "billid-t1", "billid-out"
	pay.DueDate, pay.ExternalID, pay.AccountID, pay.CardBillID = &day, &ext, &card, &bill
	if err := repo.SaveExternal(ctx, p, pay); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)

	rdesc := "billid-teste compra"
	r, err := domain.NewCardRefund(30, &rdesc, domain.CategoryShopping, "[open finance: x | x]")
	if err != nil {
		t.Fatal(err)
	}
	rpay := domain.NewPayment(r.ID, r.TotalAmount, domain.PaymentStatusPaid)
	rext := "billid-t2"
	rpay.DueDate, rpay.ExternalID, rpay.AccountID, rpay.CardBillID = &day, &rext, &card, &bill
	if err := repo.SaveExternal(ctx, r, rpay); err != nil {
		t.Fatalf("estorno negativo deveria ser aceito: %v", err)
	}
	cleanup(t, pg, r)

	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{p.ID, r.ID} {
		if got := billDueOf(t, pg, id); got == nil || got.Format("2006-01-02") != "2026-10-10" {
			t.Errorf("fatura do banco ignorada: %v", got)
		}
	}
	var oct float64
	if err := pg.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(pay.amount), 0) FROM payments pay JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.description LIKE 'billid-teste%' AND `+paymentMonth+` = '2026-10-01'`).Scan(&oct); err != nil {
		t.Fatal(err)
	}
	if oct != 50 {
		t.Errorf("outubro = 80 − 30 de estorno, got %.2f", oct)
	}

	// Compra pendente perto do fechamento, sem fatura informada: a estimativa a poria em outubro (fechada), mas o banco já
	// informa a fatura das transações deste cartão, então ela vai para a seguinte.
	pend := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	pdesc := "billid-teste pendente"
	pp, _ := domain.NewPurchase(12, &pdesc, domain.CategoryShopping, domain.PaymentMethodCreditCard, domain.PurchaseTypeSingle, "[open finance: x | x]")
	ppay := domain.NewPayment(pp.ID, 12, domain.PaymentStatusPending)
	pext := "billid-t3"
	ppay.DueDate, ppay.ExternalID, ppay.AccountID = &pend, &pext, &card
	if err := repo.SaveExternal(ctx, pp, ppay); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, pp)
	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}
	if got := billDueOf(t, pg, pp.ID); got == nil || got.Format("2006-01-02") != "2026-11-10" {
		t.Errorf("pendente sem fatura do banco deveria ir para 10/11, got %v", got)
	}

	// Parcela sem fatura informada, datada pela sincronização no mês da fatura de outubro (fechada): se fosse de outubro,
	// o banco teria dito. Vai para a seguinte; ficar em outubro a punha ao lado da parcela anterior e duplicava a previsão.
	inst := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	idesc := "billid-teste parcela (5/6)"
	ip, _ := domain.NewPurchase(40, &idesc, domain.CategoryShopping, domain.PaymentMethodCreditCard, domain.PurchaseTypeSingle, "[open finance: x | x]")
	ipay := domain.NewPayment(ip.ID, 40, domain.PaymentStatusPending)
	iext := "billid-t4"
	ipay.DueDate, ipay.ExternalID, ipay.AccountID = &inst, &iext, &card
	if err := repo.SaveExternal(ctx, ip, ipay); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, ip)
	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}
	if got := billDueOf(t, pg, ip.ID); got == nil || got.Format("2006-01-02") != "2026-11-10" {
		t.Errorf("parcela sem fatura do banco deveria ir para 10/11, got %v", got)
	}

	// O billId chega numa ressincronização de transação já gravada.
	if _, err := pg.Pool.Exec(ctx, `UPDATE payments SET card_bill_id = NULL WHERE external_id = 'billid-t1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RefreshExternal(ctx, ports.ExternalTransaction{ID: "billid-t1", Category: domain.CategoryShopping, Date: day, BillID: "billid-out"}, card); err != nil {
		t.Fatal(err)
	}
	var got *string
	pg.Pool.QueryRow(ctx, `SELECT card_bill_id FROM payments WHERE external_id = 'billid-t1'`).Scan(&got) //nolint:errcheck
	if got == nil || *got != "billid-out" {
		t.Errorf("RefreshExternal deveria gravar o billId: %v", got)
	}
}
