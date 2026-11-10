package db

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/google/uuid"
)

// Teste de integração: roda só com TEST_DATABASE_URL apontando para um Postgres com as
// migrations aplicadas. Ex.: TEST_DATABASE_URL=$DATABASE_URL go test ./internal/infra/db
func newTestRepo(t *testing.T) (*testRepo, *DB) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	pg, err := NewPostgres(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)
	return &testRepo{LedgerRepo: &LedgerRepo{db: pg}, DashboardRepo: NewDashboardRepo(pg)}, pg
}

func cleanup(t *testing.T, pg *DB, purchases ...*domain.Purchase) {
	t.Helper()
	t.Cleanup(func() {
		for _, p := range purchases {
			pg.Pool.Exec(context.Background(), `DELETE FROM purchases WHERE id = $1`, p.ID)
		}
	})
}

func manualExpense(t *testing.T, repo *testRepo, pg *DB, amount float64, typ domain.PurchaseType) *domain.Purchase {
	t.Helper()
	desc := "teste-open-finance"
	p, err := domain.NewPurchase(amount, &desc, domain.CategoryFood, domain.PaymentMethodPix, typ, "teste")
	if err != nil {
		t.Fatal(err)
	}
	pay := domain.NewPayment(p.ID, amount, domain.PaymentStatusPaid)
	if typ == domain.PurchaseTypeRecurring {
		first := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC)
		pay.ReferenceMonth = &first
	}
	if err := repo.Save(context.Background(), p, []domain.Payment{*pay}); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)
	return p
}

func externalTx(id string, amount float64, date time.Time) ports.ExternalTransaction {
	return ports.ExternalTransaction{ID: id, Date: date, Amount: amount, Kind: domain.KindExpense}
}

// testAccount cria (e remove ao final) uma conta de teste e devolve seu ID.
func testAccount(t *testing.T, repo *testRepo, pg *DB) uuid.UUID {
	t.Helper()
	id, err := repo.UpsertAccount(context.Background(), ports.ExternalAccount{ID: "of-test-acc", ItemID: "of-test", Type: "BANK", Name: "Conta teste", Last4: "1234"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, id) })
	return id
}

func externalIDOf(t *testing.T, pg *DB, purchaseID any) *string {
	t.Helper()
	var id *string
	if err := pg.Pool.QueryRow(context.Background(), `SELECT external_id FROM payments WHERE purchase_id = $1`, purchaseID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestReconcileExternal_LinksManualSingle(t *testing.T) {
	repo, pg := newTestRepo(t)
	acc := testAccount(t, repo, pg)
	ctx := context.Background()
	manual := manualExpense(t, repo, pg, 987.65, domain.PurchaseTypeSingle)

	// Fora da janela de 3 dias, de outro valor ou de outro tipo: não concilia.
	for name, tx := range map[string]ports.ExternalTransaction{
		"data distante":   externalTx("of-far", 987.65, time.Now().AddDate(0, 0, -10)),
		"valor diferente": externalTx("of-amount", 987.66, time.Now()),
		"outro tipo":      {ID: "of-kind", Date: time.Now(), Amount: 987.65, Kind: domain.KindIncome},
	} {
		if ok, err := repo.ReconcileExternal(ctx, tx, acc); err != nil || ok {
			t.Fatalf("%s: não deveria conciliar (ok=%v, err=%v)", name, ok, err)
		}
	}

	ok, err := repo.ReconcileExternal(ctx, externalTx("of-1", 987.65, time.Now().AddDate(0, 0, -2)), acc)
	if err != nil || !ok {
		t.Fatalf("deveria conciliar: ok=%v err=%v", ok, err)
	}
	if id := externalIDOf(t, pg, manual.ID); id == nil || *id != "of-1" {
		t.Errorf("external_id não gravado: %v", id)
	}

	// Cada lançamento manual só pode ser conciliado uma vez.
	if ok, _ := repo.ReconcileExternal(ctx, externalTx("of-2", 987.65, time.Now()), acc); ok {
		t.Error("um lançamento já conciliado não pode casar de novo")
	}
}

func TestReconcileExternal_LinksManualRecurringInSameMonth(t *testing.T) {
	repo, pg := newTestRepo(t)
	acc := testAccount(t, repo, pg)
	manual := manualExpense(t, repo, pg, 543.21, domain.PurchaseTypeRecurring)

	now := time.Now()
	ok, err := repo.ReconcileExternal(context.Background(), externalTx("of-rec", 543.21, time.Date(now.Year(), now.Month(), 20, 0, 0, 0, 0, time.UTC)), acc)
	if err != nil || !ok {
		t.Fatalf("recorrência do mês deveria conciliar: ok=%v err=%v", ok, err)
	}
	if id := externalIDOf(t, pg, manual.ID); id == nil || *id != "of-rec" {
		t.Errorf("external_id não gravado: %v", id)
	}
}

func TestSaveExternal_LinkAndAccount(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc := testAccount(t, repo, pg)

	save := func() error {
		desc := "teste-open-finance"
		p, _ := domain.NewPurchase(12.34, &desc, domain.CategoryMarket, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "[open finance]")
		cleanup(t, pg, p)
		id := "of-save-1"
		pay := domain.NewPayment(p.ID, 12.34, domain.PaymentStatusPaid)
		pay.ExternalID = &id
		return repo.SaveExternal(ctx, p, pay)
	}

	if found, _ := repo.RefreshExternal(ctx, externalTx("of-save-1", 12.34, time.Now()), acc); found {
		t.Fatal("banco de teste sujo: of-save-1 já existe")
	}
	if err := save(); err != nil {
		t.Fatal(err)
	}
	// Salvo antes da tabela de contas (sem conta): a sincronização seguinte vincula.
	if found, err := repo.RefreshExternal(ctx, externalTx("of-save-1", 12.34, time.Now()), acc); err != nil || !found {
		t.Errorf("deveria existir: %v %v", found, err)
	}
	var linked *uuid.UUID
	pg.Pool.QueryRow(ctx, `SELECT account_id FROM payments WHERE external_id = 'of-save-1'`).Scan(&linked)
	if linked == nil || *linked != acc {
		t.Errorf("conta não vinculada: %v", linked)
	}
	// O índice único é a última barreira contra duplicidade, e a compra não pode ficar órfã.
	if err := save(); err == nil {
		t.Error("segunda gravação do mesmo external_id deveria falhar")
	}
	var n int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM purchases WHERE description = 'teste-open-finance' AND total_amount = 12.34`).Scan(&n)
	if n != 1 {
		t.Errorf("a transação falha deveria ter sido revertida por inteiro, compras=%d", n)
	}
}

func TestUpsertAccount_UpdatesInPlace(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	first := testAccount(t, repo, pg)

	limit := 5000.0
	second, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-acc", ItemID: "of-test", Type: "CREDIT", Name: "Cartão", Last4: "8670", Balance: 99.9, CreditLimit: &limit})
	if err != nil || second != first {
		t.Fatalf("deveria atualizar a mesma conta: %v %v %v", first, second, err)
	}
	var name string
	var bal float64
	pg.Pool.QueryRow(ctx, `SELECT name, balance FROM accounts WHERE id = $1`, first).Scan(&name, &bal)
	if name != "Cartão" || bal != 99.9 {
		t.Errorf("conta não atualizada: %s %v", name, bal)
	}
}

func TestRefreshExternal_PromotesOtherExpenseCategory(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc := testAccount(t, repo, pg)

	save := func(extID string, cat domain.Category) *domain.Purchase {
		t.Helper()
		desc := "teste-open-finance"
		p, _ := domain.NewPurchase(7.77, &desc, cat, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "[open finance]")
		cleanup(t, pg, p)
		pay := domain.NewPayment(p.ID, 7.77, domain.PaymentStatusPaid)
		pay.ExternalID = &extID
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		return p
	}
	categoryOf := func(p *domain.Purchase) string {
		var c string
		pg.Pool.QueryRow(ctx, `SELECT category FROM purchases WHERE id = $1`, p.ID).Scan(&c)
		return c
	}
	refresh := func(id string, cat domain.Category) bool {
		t.Helper()
		found, err := repo.RefreshExternal(ctx, ports.ExternalTransaction{ID: id, Category: cat}, acc)
		if err != nil {
			t.Fatal(err)
		}
		return found
	}

	other, set := save("of-cat-other", domain.CategoryOther), save("of-cat-food", domain.CategoryFood)

	// Receitas e transferências importadas sem categoria também são promovidas.
	income := save("of-cat-income", domain.CategoryOther)
	pg.Pool.Exec(ctx, `UPDATE purchases SET kind = 'INCOME' WHERE id = $1`, income.ID)
	if !refresh("of-cat-income", domain.CategorySalary) || categoryOf(income) != "SALARY" {
		t.Errorf("receita OTHER deveria virar SALARY: %s", categoryOf(income))
	}

	if !refresh("of-cat-other", domain.CategoryTransport) || categoryOf(other) != "TRANSPORT" {
		t.Errorf("OTHER deveria ser promovida: %s", categoryOf(other))
	}
	refresh("of-cat-food", domain.CategoryMarket)
	if categoryOf(set) != "FOOD" {
		t.Errorf("categoria já definida não pode ser trocada: %s", categoryOf(set))
	}
	other2 := save("of-cat-other2", domain.CategoryOther)
	refresh("of-cat-other2", domain.CategoryOther)
	if categoryOf(other2) != "OTHER" {
		t.Errorf("OTHER -> OTHER não muda nada: %s", categoryOf(other2))
	}
	if refresh("of-cat-inexistente", domain.CategoryFood) {
		t.Error("transação desconhecida não pode ser reportada como existente")
	}
}

func TestInstitutions_UpsertLogoAndAccounts(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	const item = "of-test-inst"
	t.Cleanup(func() {
		pg.Pool.Exec(ctx, `DELETE FROM accounts WHERE item_id = $1`, item)
		pg.Pool.Exec(ctx, `DELETE FROM institutions WHERE item_id = $1`, item)
	})

	at := time.Date(2026, 10, 2, 17, 42, 0, 0, time.UTC)
	id, needsLogo, err := repo.UpsertInstitution(ctx, item, ports.ExternalInstitution{Name: "Banco Teste", Color: "ec0000", UpdatedAt: &at})
	if err != nil || !needsLogo {
		t.Fatalf("primeira vez deveria pedir o logo: %v %v", needsLogo, err)
	}
	if id2, _, _ := repo.UpsertInstitution(ctx, item, ports.ExternalInstitution{Name: "Banco Teste 2"}); id2 != id {
		t.Error("a instituição não pode duplicar")
	}

	// Falha na busca: registra a tentativa e só repete depois de 1 dia.
	if err := repo.SaveInstitutionLogo(ctx, id, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, needs, _ := repo.UpsertInstitution(ctx, item, ports.ExternalInstitution{Name: "Banco Teste"}); needs {
		t.Error("logo com busca recente não deveria ser pedido de novo")
	}
	if _, _, found, _ := repo.InstitutionLogo(ctx, id); found {
		t.Error("sem logo não deveria ser encontrado")
	}

	if err := repo.SaveInstitutionLogo(ctx, id, []byte("<svg/>"), "image/svg+xml"); err != nil {
		t.Fatal(err)
	}
	data, mime, found, err := repo.InstitutionLogo(ctx, id)
	if err != nil || !found || mime != "image/svg+xml" || string(data) != "<svg/>" {
		t.Fatalf("logo: %q %q %v %v", data, mime, found, err)
	}
	// Uma falha posterior não apaga o logo que já existe.
	repo.SaveInstitutionLogo(ctx, id, nil, "") //nolint:errcheck
	if _, _, found, _ := repo.InstitutionLogo(ctx, id); !found {
		t.Error("tentativa falha não pode apagar o logo")
	}
	if err := repo.SaveInstitutionLogo(ctx, id, []byte("x"), "text/html"); err == nil {
		t.Error("tipo fora da lista deveria ser recusado pelo banco")
	}

	// Conta com instituição e dados do cartão.
	due := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	min := 120.0
	if _, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-card", ItemID: item, Type: "CREDIT", Name: "Cartão",
		Brand: "VISA", DueDate: &due, MinimumPayment: &min, InstitutionID: &id}); err != nil {
		t.Fatal(err)
	}
	// Sync sem a instituição (conector falhou) não desvincula a conta.
	if _, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-card", ItemID: item, Type: "CREDIT", Name: "Cartão", Brand: "VISA", DueDate: &due}); err != nil {
		t.Fatal(err)
	}
	accounts, err := repo.Accounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range accounts {
		if a.ItemID == item {
			if a.InstitutionID == nil || *a.InstitutionID != id || a.Brand != "VISA" || a.DueDate == nil || !a.DueDate.Equal(due) {
				t.Errorf("conta inesperada: %+v", a)
			}
			insts, err := repo.Institutions(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, i := range insts {
				if i.ID == id && (!i.HasLogo || i.Color != "ec0000") {
					t.Errorf("instituição inesperada: %+v", i)
				}
				// Um sync sem a data (conector falhou) não apaga a que já existia.
				if i.ID == id && (i.SourceUpdatedAt == nil || !i.SourceUpdatedAt.Equal(at)) {
					t.Errorf("data do Pluggy deveria ser mantida: %v", i.SourceUpdatedAt)
				}
			}
			return
		}
	}
	t.Fatal("conta não encontrada")
}

func TestInstitutions_ColorCheck(t *testing.T) {
	repo, pg := newTestRepo(t)
	const item = "of-test-color"
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM institutions WHERE item_id = $1`, item) })
	if _, _, err := repo.UpsertInstitution(context.Background(), item, ports.ExternalInstitution{Name: "B", Color: "red;x"}); err == nil {
		t.Error("cor fora do padrão hexadecimal deveria ser recusada pelo banco")
	}
}

// As grafias do mesmo comércio no débito do Itaú caem na mesma conta, e o rótulo mostra o comércio.
func TestExpenseKey_DebitPrefix(t *testing.T) {
	_, pg := newTestRepo(t)
	ctx := context.Background()
	for desc, want := range map[string]string{
		"DEBITO VISA ELECTRON BRASIL   20/09 NETFLIX ENTRETENIME": "debito netflix",
		"DEBITO VISA ELECTRON BRASIL   20/08 NETFLIX.COM":         "debito netflix",
		"DEBITO VISA ELECTRON BRASIL   13/07 EBN         .SPOTIF": "debito ebn",
		"Pix enviado FULANO DE TAL":                               "pix enviado fulano de tal",
		"OTICA VENDRAMEBIRI01/12 (1/12)":                          "otica vendramebiri",
	} {
		var key, label string
		if err := pg.Pool.QueryRow(ctx, `SELECT `+expenseKey+`, `+cleanDescription+` FROM (SELECT $1::text AS description) p`, desc).Scan(&key, &label); err != nil {
			t.Fatal(err)
		}
		if key != want {
			t.Errorf("%q: chave %q, quer %q", desc, key, want)
		}
		if strings.HasPrefix(desc, "DEBITO") && strings.Contains(label, "DEBITO") {
			t.Errorf("%q: rótulo ainda tem o prefixo: %q", desc, label)
		}
	}
}

// Parcela já gravada na data da compra passa para o mês da fatura quando a sincronização a reconhece de novo.
func TestRefreshExternal_MovesInstallmentDate(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-inst-acc", ItemID: "of-test", Type: "CREDIT", Name: "Cartão"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM accounts WHERE external_id = 'of-test-inst-acc'`) })

	purchase := manualExpense(t, repo, pg, 143.08, domain.PurchaseTypeSingle)
	buy := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	if _, err := pg.Pool.Exec(ctx, `UPDATE payments SET external_id = 'of-test-parcela', due_date = $2::date, paid_at = $2::timestamptz WHERE purchase_id = $1`, purchase.ID, buy); err != nil {
		t.Fatal(err)
	}

	moved := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	if ok, err := repo.RefreshExternal(ctx, ports.ExternalTransaction{ID: "of-test-parcela", Date: moved, Installment: true, Category: domain.CategoryOther}, acc); err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	var due, paid time.Time
	if err := pg.Pool.QueryRow(ctx, `SELECT due_date, paid_at FROM payments WHERE external_id = 'of-test-parcela'`).Scan(&due, &paid); err != nil {
		t.Fatal(err)
	}
	if due.Format("2006-01-02") != "2026-09-05" || paid.UTC().Format("2006-01-02") != "2026-09-05" {
		t.Errorf("due=%v paid=%v", due, paid)
	}

	// Lançamento comum não muda de data.
	if _, err := repo.RefreshExternal(ctx, ports.ExternalTransaction{ID: "of-test-parcela", Date: buy, Installment: false, Category: domain.CategoryOther}, acc); err != nil {
		t.Fatal(err)
	}
	pg.Pool.QueryRow(ctx, `SELECT due_date FROM payments WHERE external_id = 'of-test-parcela'`).Scan(&due) //nolint:errcheck
	if due.Format("2006-01-02") != "2026-09-05" {
		t.Errorf("só parcela é movida: %v", due)
	}
}

func TestSettingsStore_RoundTrip(t *testing.T) {
	_, pg := newTestRepo(t)
	ctx := context.Background()
	st := NewSettingsStore(pg)
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM settings WHERE key = 'DIGEST_HOUR'`) })

	if err := st.SaveSetting(ctx, settings.Row{Key: "DIGEST_HOUR", Value: "7"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSetting(ctx, settings.Row{Key: "DIGEST_HOUR", Value: "8", Secret: true}); err != nil { // atualiza no lugar
		t.Fatal(err)
	}
	rows, err := st.LoadSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found int
	for _, r := range rows {
		if r.Key == "DIGEST_HOUR" {
			found++
			if r.Value != "8" || !r.Secret {
				t.Errorf("linha: %+v", r)
			}
		}
	}
	if found != 1 {
		t.Errorf("a chave não pode duplicar: %d", found)
	}
	if err := st.SaveSetting(ctx, settings.Row{Key: "chave minuscula", Value: "x"}); err == nil {
		t.Error("o banco recusa chaves fora do padrão")
	}
	if err := st.DeleteSetting(ctx, "DIGEST_HOUR"); err != nil {
		t.Fatal(err)
	}
}

func TestOwnTransferCandidatesAndCancel(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-own-acc", ItemID: "of-test", Type: "BANK", Name: "Conta"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM accounts WHERE external_id = 'of-test-own-acc'`) })

	mk := func(extID, desc string, kind domain.PurchaseKind) *domain.Purchase {
		var p *domain.Purchase
		var err error
		if kind == domain.KindIncome {
			p, err = domain.NewIncome(10, &desc, domain.CategoryOther, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
		} else {
			p, err = domain.NewPurchase(10, &desc, domain.CategoryOther, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
		}
		if err != nil {
			t.Fatal(err)
		}
		day := time.Date(1999, 3, 1, 0, 0, 0, 0, time.UTC)
		pay := domain.NewPayment(p.ID, 10, domain.PaymentStatusPaid)
		pay.DueDate, pay.PaidAt, pay.ExternalID, pay.AccountID = &day, &day, &extID, &acc
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		cleanup(t, pg, p)
		return p
	}
	mk("of-own-1", "Pix enviado ZZ PESSOA TESTE", domain.KindExpense)
	mk("of-own-2", "Pix recebido ZZ PESSOA TESTE", domain.KindIncome)
	mk("of-own-3", "Compra débito PADARIA ZZ", domain.KindExpense) // não é transferência

	cands, err := repo.OwnTransferCandidates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	for _, c := range cands {
		if strings.Contains(c.Description, "ZZ PESSOA TESTE") {
			ids = append(ids, c.PaymentID)
		}
		if strings.Contains(c.Description, "PADARIA ZZ") {
			t.Error("compra comum não é candidata")
		}
	}
	if len(ids) != 2 {
		t.Fatalf("esperava 2 candidatas, got %d", len(ids))
	}
	n, err := repo.CancelPayments(ctx, ids)
	if err != nil || n != 2 {
		t.Fatalf("cancelados %d, %v", n, err)
	}
	if again, _ := repo.CancelPayments(ctx, ids); again != 0 {
		t.Errorf("cancelar de novo não muda nada: %d", again)
	}
	after, _ := repo.OwnTransferCandidates(ctx)
	for _, c := range after {
		if strings.Contains(c.Description, "ZZ PESSOA TESTE") {
			t.Error("cancelada não é mais candidata")
		}
	}
}

// h1Expense grava uma despesa externa em OTHER com a categoria do Pluggy no raw_input. A descrição só tem letras
// (é o que forma a chave da regra de categoria).
func h1Expense(t *testing.T, repo *testRepo, pg *DB, extID, desc, pluggyCat string, amount float64, daysAgo int) *domain.Purchase {
	t.Helper()
	ctx := context.Background()
	p, _ := domain.NewPurchase(amount, &desc, domain.CategoryOther, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "[open finance: "+pluggyCat+" | "+desc+"]")
	cleanup(t, pg, p)
	pay := domain.NewPayment(p.ID, amount, domain.PaymentStatusPaid)
	day := time.Now().UTC().AddDate(0, 0, -daysAgo)
	pay.DueDate, pay.PaidAt = &day, &day
	if extID != "" {
		pay.ExternalID = &extID
	}
	if err := repo.SaveExternal(ctx, p, pay); err != nil {
		t.Fatal(err)
	}
	return p
}

func h1Rule(t *testing.T, pg *DB, key string, cat domain.Category) {
	t.Helper()
	ctx := context.Background()
	if _, err := pg.Pool.Exec(ctx, `INSERT INTO category_rules (key, category) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET category = $2`, key, string(cat)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM category_rules WHERE key = $1`, key) })
}

func h1Category(t *testing.T, pg *DB, p *domain.Purchase) string {
	t.Helper()
	var c string
	pg.Pool.QueryRow(context.Background(), `SELECT category FROM purchases WHERE id = $1`, p.ID).Scan(&c)
	return c
}

func TestRefreshExternal_RespectsCategoryRules(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc := testAccount(t, repo, pg)

	keepOther := h1Expense(t, repo, pg, "of-h1-keep", "teste h um manter outros", "Housing", 10, 5)
	other := h1Expense(t, repo, pg, "of-h1-other", "teste h um regra outra", "Housing", 10, 5)
	free := h1Expense(t, repo, pg, "of-h1-free", "teste h um sem regra", "Housing", 10, 5)
	h1Rule(t, pg, "teste h um manter outros", domain.CategoryOther)
	h1Rule(t, pg, "teste h um regra outra", domain.CategoryFood)

	for _, id := range []string{"of-h1-keep", "of-h1-other", "of-h1-free"} {
		if found, err := repo.RefreshExternal(ctx, ports.ExternalTransaction{ID: id, Category: domain.CategoryHousing}, acc); err != nil || !found {
			t.Fatalf("%s: found=%v err=%v", id, found, err)
		}
	}
	if c := h1Category(t, pg, keepOther); c != "OTHER" {
		t.Errorf("regra 'manter em Outros' foi sobrescrita: %s", c)
	}
	if c := h1Category(t, pg, other); c != "OTHER" {
		t.Errorf("chave com regra de outra categoria não deve ser promovida pelo refresh: %s", c)
	}
	if c := h1Category(t, pg, free); c != "HOUSING" {
		t.Errorf("sem regra deveria virar HOUSING: %s", c)
	}
}

func TestRecategorizeExternalOther(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	mapping := map[string]domain.Category{
		"housing": domain.CategoryHousing, "services": domain.CategoryBills, "bookstore": domain.CategoryEducation,
	}

	old := h1Expense(t, repo, pg, "of-h1-old", "teste h um antiga moradia", "Housing", 100.10, 400) // fora da janela da sync
	svc := h1Expense(t, repo, pg, "of-h1-svc", "teste h um servico", "Services", 50.05, 10)
	digital := h1Expense(t, repo, pg, "of-h1-dig", "teste h um digital", "Digital services", 20, 10)
	unknown := h1Expense(t, repo, pg, "of-h1-unk", "teste h um desconhecida", "Whatever", 30, 10)
	manual := h1Expense(t, repo, pg, "", "teste h um manual", "Housing", 40, 10) // sem external_id
	ruled := h1Expense(t, repo, pg, "of-h1-rule", "teste h um com regra", "Housing", 60, 10)
	h1Rule(t, pg, "teste h um com regra", domain.CategoryOther)
	income := h1Expense(t, repo, pg, "of-h1-inc", "teste h um receita", "Housing", 70, 10)
	transfer := h1Expense(t, repo, pg, "of-h1-trf", "teste h um transferencia", "Housing", 80, 10)
	pg.Pool.Exec(ctx, `UPDATE purchases SET kind = 'INCOME' WHERE id = $1`, income.ID)
	pg.Pool.Exec(ctx, `UPDATE purchases SET kind = 'TRANSFER', transfer_direction = 'OUT' WHERE id = $1`, transfer.ID)

	sum := func() (total float64, n int) {
		pg.Pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(pay.amount), 0), COUNT(*) FROM payments pay JOIN purchases p ON p.id = pay.purchase_id
			WHERE p.kind = 'EXPENSE' AND pay.status <> 'CANCELLED' AND p.description LIKE 'teste h um %'`).Scan(&total, &n)
		return
	}
	totalBefore, nBefore := sum()

	changed, err := repo.RecategorizeExternalOther(ctx, mapping)
	if err != nil {
		t.Fatal(err)
	}
	// O banco de teste pode ter outras linhas reais de teste; as nossas são 2 (antiga e serviço).
	if changed < 2 {
		t.Errorf("esperava ao menos 2 linhas alteradas, got %d", changed)
	}
	want := map[*domain.Purchase]string{
		old: "HOUSING", svc: "BILLS", digital: "OTHER", unknown: "OTHER", manual: "OTHER", ruled: "OTHER", income: "OTHER", transfer: "OTHER",
	}
	for p, c := range want {
		if got := h1Category(t, pg, p); got != c {
			t.Errorf("%s: got %s, esperado %s", *p.Description, got, c)
		}
	}

	// A soma das despesas não muda (só a coluna category).
	if totalAfter, nAfter := sum(); totalAfter != totalBefore || nAfter != nBefore {
		t.Errorf("a soma mudou: antes %v (%d), depois %v (%d)", totalBefore, nBefore, totalAfter, nAfter)
	}

	// Idempotente: a segunda rodada não altera nada das nossas linhas.
	again, err := repo.RecategorizeExternalOther(ctx, mapping)
	if err != nil || again != 0 {
		t.Errorf("segunda rodada deveria alterar 0 linhas, got %d err=%v", again, err)
	}
}

func TestSaveExternal_CategoryRuleAlwaysWinsOnExpenses(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()

	save := func(extID, desc string, kind domain.PurchaseKind, cat domain.Category) *domain.Purchase {
		t.Helper()
		var p *domain.Purchase
		switch kind {
		case domain.KindIncome:
			p, _ = domain.NewIncome(15, &desc, cat, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "[open finance]")
		default:
			p, _ = domain.NewPurchase(15, &desc, cat, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "[open finance]")
		}
		cleanup(t, pg, p)
		pay := domain.NewPayment(p.ID, 15, domain.PaymentStatusPaid)
		pay.ExternalID = &extID
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		return p
	}

	h1Rule(t, pg, "teste h um save manter outros", domain.CategoryOther)
	h1Rule(t, pg, "teste h um save regra comida", domain.CategoryFood)
	h1Rule(t, pg, "teste h um save receita", domain.CategoryFood)

	cases := []struct {
		name string
		p    *domain.Purchase
		want string
	}{
		{"regra OTHER vence o Housing do Pluggy", save("of-h1s-1", "teste h um save manter outros", domain.KindExpense, domain.CategoryHousing), "OTHER"},
		{"regra FOOD vence o Utilities (Contas) do Pluggy", save("of-h1s-2", "teste h um save regra comida", domain.KindExpense, domain.CategoryBills), "FOOD"},
		{"regra FOOD também vale sobre OTHER", save("of-h1s-3", "teste h um save regra comida", domain.KindExpense, domain.CategoryOther), "FOOD"},
		{"sem regra, Housing continua HOUSING", save("of-h1s-4", "teste h um save sem regra", domain.KindExpense, domain.CategoryHousing), "HOUSING"},
		{"receita não é afetada pela regra", save("of-h1s-5", "teste h um save receita", domain.KindIncome, domain.CategorySalary), "SALARY"},
	}
	for _, tc := range cases {
		if got := h1Category(t, pg, tc.p); got != tc.want {
			t.Errorf("%s: got %s, esperado %s", tc.name, got, tc.want)
		}
	}
}
