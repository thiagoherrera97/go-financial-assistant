package db

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// trSeed grava um lançamento no formato do que vem do banco ("[open finance: <categoria> | ...]"); extID vazio é manual.
func trSeed(t *testing.T, repo *testRepo, pg *DB, kind domain.PurchaseKind, desc, pluggyCat, extID string, amount float64) (*domain.Purchase, uuid.UUID) {
	t.Helper()
	raw := "[open finance: " + pluggyCat + " | " + desc + "]"
	var (
		p   *domain.Purchase
		err error
	)
	switch kind {
	case domain.KindIncome:
		p, err = domain.NewIncome(amount, &desc, domain.CategoryOther, domain.PaymentMethodPix, domain.PurchaseTypeSingle, raw)
	case domain.KindTransfer:
		p, err = domain.NewTransfer(amount, &desc, domain.PaymentMethodPix, domain.PurchaseTypeSingle, raw, domain.TransferDirectionOut)
	default:
		p, err = domain.NewPurchase(amount, &desc, domain.CategoryOther, domain.PaymentMethodPix, domain.PurchaseTypeSingle, raw)
	}
	if err != nil {
		t.Fatal(err)
	}
	pay := domain.NewPayment(p.ID, amount, domain.PaymentStatusPaid)
	pay.DueDate = &mar1999
	if extID != "" {
		pay.ExternalID = &extID
	}
	if extID == "" {
		if err := repo.Save(context.Background(), p, []domain.Payment{*pay}); err != nil {
			t.Fatal(err)
		}
	} else if err := repo.SaveExternal(context.Background(), p, pay); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)
	return p, pay.ID
}

func trPendingIDs(t *testing.T, repo *testRepo) map[uuid.UUID]domain.TransferReviewItem {
	t.Helper()
	items, _, err := repo.PendingTransfers(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	out := map[uuid.UUID]domain.TransferReviewItem{}
	for _, it := range items {
		if strings.HasPrefix(it.Description, "teste h dois") {
			out[it.PaymentID] = it
		}
	}
	return out
}

func trCategory(t *testing.T, pg *DB, id uuid.UUID) (status, category string) {
	t.Helper()
	if err := pg.Pool.QueryRow(context.Background(), `SELECT pay.status, p.category FROM payments pay JOIN purchases p ON p.id = pay.purchase_id WHERE pay.id = $1`, id).Scan(&status, &category); err != nil {
		t.Fatal(err)
	}
	return
}

func TestTransferReview_ListOnlyExternalTransfers(t *testing.T) {
	repo, pg := newTestRepo(t)
	_, out := trSeed(t, repo, pg, domain.KindExpense, "teste h dois pix saida", "Transfer - PIX", "of-h2-out", 120)
	_, in := trSeed(t, repo, pg, domain.KindIncome, "teste h dois pix entrada", "Transfer - Internal", "of-h2-in", 80)
	_, manual := trSeed(t, repo, pg, domain.KindExpense, "teste h dois manual", "Transfer - PIX", "", 50)
	_, other := trSeed(t, repo, pg, domain.KindExpense, "teste h dois mercado", "Groceries", "of-h2-mkt", 30)
	_, invest := trSeed(t, repo, pg, domain.KindTransfer, "teste h dois aplicacao", "Transfer - PIX", "of-h2-inv", 500)

	got := trPendingIDs(t, repo)
	if _, ok := got[out]; !ok || got[out].Kind != "EXPENSE" || got[out].Amount != 120 {
		t.Errorf("a saída deveria estar na lista: %+v", got[out])
	}
	if _, ok := got[in]; !ok || got[in].Kind != "INCOME" {
		t.Errorf("a entrada deveria estar na lista: %+v", got[in])
	}
	for name, id := range map[string]uuid.UUID{"manual": manual, "outra categoria": other, "investimento (kind TRANSFER)": invest} {
		if _, ok := got[id]; ok {
			t.Errorf("%s não pode aparecer na lista", name)
		}
	}
}

// Cada decisão muda só o que deve, e desfazer devolve status e categoria exatos e a lista volta ao que era.
func TestTransferReview_DecisionsAndUndoRestoreExactly(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, a := trSeed(t, repo, pg, domain.KindExpense, "teste h dois a", "Transfer - PIX", "of-h2-a", 100)
	_, b := trSeed(t, repo, pg, domain.KindExpense, "teste h dois b", "Transfer - PIX", "of-h2-b", 200)
	_, c := trSeed(t, repo, pg, domain.KindExpense, "teste h dois c", "Transfer - PIX", "of-h2-c", 300)
	_, inc := trSeed(t, repo, pg, domain.KindIncome, "teste h dois d", "Transfer - PIX", "of-h2-d", 400)

	if err := repo.DecideTransfer(ctx, a, domain.TransferOwn, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.DecideTransfer(ctx, b, domain.TransferPerson, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.DecideTransfer(ctx, c, domain.TransferPayment, "HOUSING"); err != nil {
		t.Fatal(err)
	}
	if err := repo.DecideTransfer(ctx, inc, domain.TransferPerson, ""); err != nil {
		t.Fatal(err)
	}
	if s, cat := trCategory(t, pg, a); s != "CANCELLED" || cat != "OTHER" {
		t.Errorf("entre contas minhas: status=%s categoria=%s", s, cat)
	}
	if s, cat := trCategory(t, pg, b); s != "PAID" || cat != "PEOPLE" {
		t.Errorf("para pessoa: status=%s categoria=%s", s, cat)
	}
	if s, cat := trCategory(t, pg, c); s != "PAID" || cat != "HOUSING" {
		t.Errorf("pagamento: status=%s categoria=%s", s, cat)
	}
	var kind string
	pg.Pool.QueryRow(ctx, `SELECT p.kind FROM payments pay JOIN purchases p ON p.id = pay.purchase_id WHERE pay.id = $1`, inc).Scan(&kind)
	if s, cat := trCategory(t, pg, inc); kind != "INCOME" || cat != "PEOPLE" || s != "PAID" {
		t.Errorf("entrada para pessoa mantém o tipo: kind=%s categoria=%s", kind, cat)
	}
	if left := trPendingIDs(t, repo); len(left) != 0 {
		t.Errorf("decididas saem da lista: %d", len(left))
	}
	decided, err := repo.DecidedTransfers(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uuid.UUID]domain.TransferDecision{}
	for _, d := range decided {
		seen[d.PaymentID] = d.Decision
		if d.DecidedAt == nil {
			t.Error("decidida sem data")
		}
	}
	if seen[a] != domain.TransferOwn || seen[b] != domain.TransferPerson || seen[c] != domain.TransferPayment {
		t.Errorf("decididas: %v", seen)
	}

	// Já decidida: não decide de novo.
	if err := repo.DecideTransfer(ctx, a, domain.TransferPerson, ""); !errors.Is(err, domain.ErrTransferNotPending) {
		t.Errorf("segunda decisão deveria falhar: %v", err)
	}
	// Pagamento vale só para saídas; e a falha não deixa decisão pela metade.
	_, inc2 := trSeed(t, repo, pg, domain.KindIncome, "teste h dois e", "Transfer - PIX", "of-h2-e", 10)
	if err := repo.DecideTransfer(ctx, inc2, domain.TransferPayment, "FOOD"); !errors.Is(err, domain.ErrTransferPaymentOnIncome) {
		t.Errorf("Pagamento em entrada: %v", err)
	}
	if _, ok := trPendingIDs(t, repo)[inc2]; !ok {
		t.Error("a entrada recusada continua na lista")
	}

	for _, id := range []uuid.UUID{a, b, c, inc} {
		if err := repo.UndoTransferDecision(ctx, id); err != nil {
			t.Fatal(err)
		}
		if s, cat := trCategory(t, pg, id); s != "PAID" || cat != "OTHER" {
			t.Errorf("desfazer deveria restaurar PAID/OTHER, got %s/%s", s, cat)
		}
	}
	if left := trPendingIDs(t, repo); len(left) != 5 {
		t.Errorf("as 4 desfeitas voltam à lista (com a entrada recusada): %d", len(left))
	}
	if err := repo.UndoTransferDecision(ctx, a); !errors.Is(err, domain.ErrTransferNoDecision) {
		t.Errorf("desfazer sem decisão: %v", err)
	}
	if err := repo.DecideTransfer(ctx, uuid.New(), domain.TransferOwn, ""); !errors.Is(err, domain.ErrTransferNotPending) {
		t.Errorf("id inexistente: %v", err)
	}
}

// Desfazer restaura também o estado original quando o pagamento estava pendente (fatura aberta).
func TestTransferReview_UndoRestoresPendingStatus(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, id := trSeed(t, repo, pg, domain.KindExpense, "teste h dois pendente", "Transfer - PIX", "of-h2-pend", 70)
	pg.Pool.Exec(ctx, `UPDATE payments SET status = 'PENDING', paid_at = NULL WHERE id = $1`, id)
	if err := repo.DecideTransfer(ctx, id, domain.TransferOwn, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.UndoTransferDecision(ctx, id); err != nil {
		t.Fatal(err)
	}
	if s, _ := trCategory(t, pg, id); s != "PENDING" {
		t.Errorf("status restaurado: %s", s)
	}
}

// "Entre contas minhas" tira o valor da Visão geral, de Gastos (por categoria) e da Projeção (despesas e renda por conta);
// desfazer devolve os valores exatos de antes.
func TestTransferReview_OwnDecisionRemovesFromTotalsAndUndoRestores(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, outID := trSeed(t, repo, pg, domain.KindExpense, "teste h dois totais saida", "Transfer - PIX", "of-h2-t1", 123.45)
	_, inID := trSeed(t, repo, pg, domain.KindIncome, "teste h dois totais entrada", "Transfer - PIX", "of-h2-t2", 67.89)

	type snapshot struct {
		income, expense, breakdown float64
		keys                       map[string]float64
		incomeKeys                 map[string]float64
	}
	take := func() snapshot {
		t.Helper()
		var s snapshot
		totals, err := repo.MonthlyTotals(ctx, mar1999, mar1999)
		if err != nil || len(totals) != 1 {
			t.Fatalf("totais: %v %v", totals, err)
		}
		s.income, s.expense = totals[0].Income, totals[0].Expense
		items, err := repo.ExpenseBreakdown(ctx, mar1999, domain.BreakdownByCategory)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			s.breakdown += it.Total
		}
		keys, err := repo.ExpenseKeyMonths(ctx, mar1999, mar1999)
		if err != nil {
			t.Fatal(err)
		}
		s.keys = map[string]float64{}
		for _, k := range keys {
			if strings.HasPrefix(k.Key, "teste h dois") {
				s.keys[k.Key] += k.Total
			}
		}
		inc, err := repo.IncomePayments(ctx, mar1999, mar1999)
		if err != nil {
			t.Fatal(err)
		}
		s.incomeKeys = map[string]float64{}
		for _, i := range inc {
			if strings.HasPrefix(i.Key, "teste h dois") {
				s.incomeKeys[i.Key] += i.Amount
			}
		}
		return s
	}
	near := func(a, b float64) bool { d := a - b; return d < 0.005 && d > -0.005 }

	before := take()
	if len(before.keys) != 1 || len(before.incomeKeys) != 1 {
		t.Fatalf("a Projeção deveria enxergar as duas antes: %+v", before)
	}
	if err := repo.DecideTransfer(ctx, outID, domain.TransferOwn, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.DecideTransfer(ctx, inID, domain.TransferOwn, ""); err != nil {
		t.Fatal(err)
	}
	after := take()
	if !near(before.expense-after.expense, 123.45) || !near(before.income-after.income, 67.89) {
		t.Errorf("Visão geral: despesa %v -> %v, renda %v -> %v", before.expense, after.expense, before.income, after.income)
	}
	if !near(before.breakdown-after.breakdown, 123.45) {
		t.Errorf("Gastos por categoria: %v -> %v", before.breakdown, after.breakdown)
	}
	if len(after.keys) != 0 || len(after.incomeKeys) != 0 {
		t.Errorf("a Projeção não pode enxergar as transferências entre contas minhas: %+v", after)
	}

	for _, id := range []uuid.UUID{outID, inID} {
		if err := repo.UndoTransferDecision(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	restored := take()
	if !near(restored.expense, before.expense) || !near(restored.income, before.income) || !near(restored.breakdown, before.breakdown) ||
		len(restored.keys) != 1 || len(restored.incomeKeys) != 1 {
		t.Errorf("desfazer deveria devolver os totais de antes: antes=%+v depois=%+v", before, restored)
	}
}

// "Para pessoa" e "Pagamento" não mudam o total, só a categoria; e a sincronização seguinte não desfaz a decisão.
func TestTransferReview_PersonAndPaymentKeepTotalsAndSurviveRefresh(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, id := trSeed(t, repo, pg, domain.KindExpense, "teste h dois sync", "Transfer - PIX", "of-h2-sync", 55.55)
	_, other := trSeed(t, repo, pg, domain.KindExpense, "teste h dois sync b", "Transfer - PIX", "of-h2-sync-b", 44.44)
	acc := testAccount(t, repo, pg)

	totalBefore := func() float64 {
		totals, _ := repo.MonthlyTotals(ctx, mar1999, mar1999)
		return totals[0].Expense
	}
	before := totalBefore()
	if err := repo.DecideTransfer(ctx, id, domain.TransferPerson, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.DecideTransfer(ctx, other, domain.TransferPayment, "BILLS"); err != nil {
		t.Fatal(err)
	}
	if after := totalBefore(); after != before {
		t.Errorf("o total não muda com Pessoa/Pagamento: %v -> %v", before, after)
	}
	// A sincronização seguinte relê as transferências (RefreshExternal): a categoria escolhida fica.
	for _, ext := range []string{"of-h2-sync", "of-h2-sync-b"} {
		if _, err := repo.RefreshExternal(ctx, externalTx(ext, 1, time.Now()), acc); err != nil {
			t.Fatal(err)
		}
	}
	if _, cat := trCategory(t, pg, id); cat != "PEOPLE" {
		t.Errorf("a decisão não pode ser desfeita pela sincronização: %s", cat)
	}
	if _, cat := trCategory(t, pg, other); cat != "BILLS" {
		t.Errorf("a decisão não pode ser desfeita pela sincronização: %s", cat)
	}
}

// "Para pessoa" e depois a limpeza do OWN_NAMES cancela o pagamento: desfazer devolve a categoria e NÃO o reativa.
func TestTransferReview_UndoPersonDoesNotReactivateCancelledPayment(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, id := trSeed(t, repo, pg, domain.KindExpense, "teste h dois pessoa cancelada", "Transfer - PIX", "of-h2-pc", 90)
	if err := repo.DecideTransfer(ctx, id, domain.TransferPerson, ""); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.CancelPayments(ctx, []uuid.UUID{id}); err != nil || n != 1 {
		t.Fatalf("CancelPayments: n=%d err=%v", n, err)
	}
	if err := repo.UndoTransferDecision(ctx, id); err != nil {
		t.Fatal(err)
	}
	if s, cat := trCategory(t, pg, id); s != "CANCELLED" || cat != "OTHER" {
		t.Errorf("esperado CANCELLED/OTHER, got %s/%s", s, cat)
	}
}

// "Pagamento" e depois a categoria trocada em outra tela: desfazer não a sobrescreve (nem mexe no status).
func TestTransferReview_UndoPaymentKeepsCategoryChangedLater(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, id := trSeed(t, repo, pg, domain.KindExpense, "teste h dois pagamento trocado", "Transfer - PIX", "of-h2-pt", 60)
	if err := repo.DecideTransfer(ctx, id, domain.TransferPayment, "HOUSING"); err != nil {
		t.Fatal(err)
	}
	var applied *string
	if err := pg.Pool.QueryRow(ctx, `SELECT applied_category FROM transfer_decisions WHERE payment_id = $1`, id).Scan(&applied); err != nil || applied == nil || *applied != "HOUSING" {
		t.Fatalf("applied_category gravada: %v %v", applied, err)
	}
	if _, err := pg.Pool.Exec(ctx, `UPDATE purchases SET category = 'FOOD' WHERE id = (SELECT purchase_id FROM payments WHERE id = $1)`, id); err != nil {
		t.Fatal(err)
	}
	if err := repo.UndoTransferDecision(ctx, id); err != nil {
		t.Fatal(err)
	}
	if s, cat := trCategory(t, pg, id); s != "PAID" || cat != "FOOD" {
		t.Errorf("esperado PAID/FOOD, got %s/%s", s, cat)
	}
	if _, ok := trPendingIDs(t, repo)[id]; !ok {
		t.Error("desfeita volta à lista de pendentes")
	}
}

// "Entre contas minhas" sem categoria aplicada; desfazer com o status ainda CANCELLED volta ao anterior (PAID e PENDING).
func TestTransferReview_UndoOwnRestoresStatusWhileStillCancelled(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	for _, prev := range []string{"PAID", "PENDING"} {
		_, id := trSeed(t, repo, pg, domain.KindExpense, "teste h dois own "+prev, "Transfer - PIX", "of-h2-own-"+prev, 40)
		pg.Pool.Exec(ctx, `UPDATE payments SET status = $2 WHERE id = $1`, id, prev) //nolint:errcheck
		if err := repo.DecideTransfer(ctx, id, domain.TransferOwn, ""); err != nil {
			t.Fatal(err)
		}
		var applied *string
		pg.Pool.QueryRow(ctx, `SELECT applied_category FROM transfer_decisions WHERE payment_id = $1`, id).Scan(&applied) //nolint:errcheck
		if applied != nil {
			t.Errorf("OWN não aplica categoria: %v", *applied)
		}
		if err := repo.UndoTransferDecision(ctx, id); err != nil {
			t.Fatal(err)
		}
		if s, cat := trCategory(t, pg, id); s != prev || cat != "OTHER" {
			t.Errorf("esperado %s/OTHER, got %s/%s", prev, s, cat)
		}
	}
}

// "Entre contas minhas" e depois o status mudou por outra via: desfazer não mexe nele.
func TestTransferReview_UndoOwnLeavesStatusChangedLater(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, id := trSeed(t, repo, pg, domain.KindExpense, "teste h dois own mudado", "Transfer - PIX", "of-h2-own-mud", 45)
	pg.Pool.Exec(ctx, `UPDATE payments SET status = 'PENDING' WHERE id = $1`, id) //nolint:errcheck
	if err := repo.DecideTransfer(ctx, id, domain.TransferOwn, ""); err != nil {
		t.Fatal(err)
	}
	pg.Pool.Exec(ctx, `UPDATE payments SET status = 'PAID' WHERE id = $1`, id) //nolint:errcheck
	if err := repo.UndoTransferDecision(ctx, id); err != nil {
		t.Fatal(err)
	}
	if s, _ := trCategory(t, pg, id); s != "PAID" {
		t.Errorf("status mudado depois deveria ficar PAID, got %s", s)
	}
	var n int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM transfer_decisions WHERE payment_id = $1`, id).Scan(&n) //nolint:errcheck
	if n != 0 {
		t.Error("a decisão sempre é apagada")
	}
}

// Duas decisões ao mesmo tempo sobre o mesmo pagamento: uma grava, a outra recebe ErrTransferNotPending (nunca 500).
func TestTransferReview_ConcurrentDecisionsOneWins(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	decisions := []domain.TransferDecision{domain.TransferPerson, domain.TransferPayment, domain.TransferPerson, domain.TransferPayment}
	for round := range 5 {
		_, id := trSeed(t, repo, pg, domain.KindExpense, "teste h dois corrida", "Transfer - PIX", "of-h2-race-"+string(rune('a'+round)), 25)
		var (
			wg   sync.WaitGroup
			mu   sync.Mutex
			ok   int
			errs []error
		)
		start := make(chan struct{})
		for _, d := range decisions {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				cat := ""
				if d == domain.TransferPayment {
					cat = "HOUSING"
				}
				err := repo.DecideTransfer(ctx, id, d, cat)
				mu.Lock()
				defer mu.Unlock()
				if err == nil {
					ok++
				} else if !errors.Is(err, domain.ErrTransferNotPending) {
					errs = append(errs, err)
				}
			}()
		}
		close(start)
		wg.Wait()
		if len(errs) > 0 || ok != 1 {
			t.Fatalf("rodada %d: gravadas=%d erros inesperados=%v", round, ok, errs)
		}
		var n int
		pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM transfer_decisions WHERE payment_id = $1`, id).Scan(&n) //nolint:errcheck
		if n != 1 {
			t.Fatalf("rodada %d: %d decisões gravadas", round, n)
		}
	}
}

// Reproduz a corrida do QA de forma determinística: a 2ª decisão espera a trava do pagamento enquanto a 1ª grava a
// decisão sem alterar a linha de payments; ao acordar, o NOT EXISTS (snapshot antigo) passa e o INSERT bate na chave.
func TestTransferReview_DecisionRaceOnPrimaryKeyIsNotPending(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, id := trSeed(t, repo, pg, domain.KindExpense, "teste h dois corrida pk", "Transfer - PIX", "of-h2-race-pk", 33)

	first, err := pg.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(ctx) //nolint:errcheck
	if _, err := first.Exec(ctx, `SELECT 1 FROM payments WHERE id = $1 FOR UPDATE`, id); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- repo.DecideTransfer(ctx, id, domain.TransferPerson, "") }()

	// Espera a 2ª ficar bloqueada na trava.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query LIKE '%transfer_decisions d WHERE d.payment_id%'`).Scan(&waiting) //nolint:errcheck
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a segunda decisão não chegou a esperar a trava")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := first.Exec(ctx, `INSERT INTO transfer_decisions (payment_id, decision, previous_status, previous_category, applied_category)
		VALUES ($1, 'PERSON', 'PAID', 'OTHER', 'PEOPLE')`, id); err != nil {
		t.Fatal(err)
	}
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, domain.ErrTransferNotPending) {
		t.Fatalf("a segunda decisão deveria receber ErrTransferNotPending, got %v", err)
	}
}

// A parcela de "Outros" usa a mesma conta (e os mesmos filtros) do total de despesas da Visão geral: só EXPENSE, sem CANCELLED.
func TestMonthlyTotals_OtherExpenseUsesTheSameFiltersAsExpense(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryOther, desc: "oth-a", amount: 30, date: mar1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "oth-b", amount: 70, date: mar1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryOther, desc: "oth-cancelled", amount: 999, date: mar1999, status: domain.PaymentStatusCancelled})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategoryOther, desc: "oth-income", amount: 500, date: mar1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindTransfer, dir: domain.TransferDirectionOut, desc: "oth-transfer", amount: 40, date: mar1999})

	totals, err := repo.MonthlyTotals(ctx, mar1999.AddDate(0, -1, 0), mar1999)
	if err != nil || len(totals) != 2 {
		t.Fatalf("totais: %v %v", totals, err)
	}
	empty, m := totals[0], totals[1]
	if m.Expense != 100 || m.OtherExpense != 30 {
		t.Errorf("despesa 100 (30 em Outros), sem a cancelada, a renda nem a transferência: %+v", m)
	}
	if m.Income != 500 || m.Applied != 40 || m.Redeemed != 0 {
		t.Errorf("os outros totais não mudam: %+v", m)
	}
	if share := m.OtherShare(); share == nil || *share != 0.3 {
		t.Errorf("parcela = %v, esperado 0,3", share)
	}
	if empty.Expense != 0 || empty.OtherShare() != nil {
		t.Errorf("mês sem despesas: parcela nula, got %+v", empty)
	}
}

// Decidir uma transferência (H2a) muda a parcela: "para uma pessoa" sai de Outros e "entre contas minhas" some das despesas.
func TestMonthlyTotals_OtherShareFollowsTransferDecisions(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	_, a := trSeed(t, repo, pg, domain.KindExpense, "teste h dois share a", "Transfer - PIX", "of-h2c-a", 60)
	_, b := trSeed(t, repo, pg, domain.KindExpense, "teste h dois share b", "Transfer - PIX", "of-h2c-b", 40)
	share := func() (float64, float64, float64) {
		totals, err := repo.MonthlyTotals(ctx, mar1999, mar1999)
		if err != nil || len(totals) != 1 {
			t.Fatal(totals, err)
		}
		s := totals[0].OtherShare()
		if s == nil {
			t.Fatal("deveria haver despesas")
		}
		return *s, totals[0].Expense, totals[0].OtherExpense
	}
	if s, e, o := share(); s != 1 || e != 100 || o != 100 {
		t.Fatalf("antes: %v %v %v", s, e, o)
	}
	if err := repo.DecideTransfer(ctx, a, domain.TransferPerson, ""); err != nil {
		t.Fatal(err)
	}
	if s, e, o := share(); e != 100 || o != 40 || s != 0.4 {
		t.Errorf("Pessoa tira de Outros sem mudar o total: %v %v %v", s, e, o)
	}
	if err := repo.DecideTransfer(ctx, b, domain.TransferOwn, ""); err != nil {
		t.Fatal(err)
	}
	if s, e, o := share(); e != 60 || o != 0 || s != 0 {
		t.Errorf("Entre contas minhas sai das despesas: %v %v %v", s, e, o)
	}
}
