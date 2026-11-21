package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func subcategoryOf(t *testing.T, pg *DB, purchaseID uuid.UUID) (string, *bool, string) {
	t.Helper()
	var sub, cat string
	var ess *bool
	if err := pg.Pool.QueryRow(context.Background(),
		`SELECT COALESCE(subcategory, ''), essential, category FROM purchases WHERE id = $1`, purchaseID).Scan(&sub, &ess, &cat); err != nil {
		t.Fatal(err)
	}
	return sub, ess, cat
}

func paymentOf(t *testing.T, pg *DB, purchaseID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pg.Pool.QueryRow(context.Background(), `SELECT id FROM payments WHERE purchase_id = $1`, purchaseID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSubcategories(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	bank, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "sc-bank", ItemID: "sc", Type: "CREDIT", Name: "Cartão SC"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, bank)
		pg.Pool.Exec(context.Background(), `DELETE FROM category_rules WHERE key LIKE 'sc teste%'`)
	})

	nov := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	ifood := cardExpense(t, repo, pg, bank, "sc-1", "sc-teste IFOOD pedido", 50, nov)
	wash := cardExpense(t, repo, pg, bank, "sc-2", "sc-teste MARMITEX DO GORDINHO", 38, nov)
	wash2 := cardExpense(t, repo, pg, bank, "sc-3", "sc-teste MARMITEX DO GORDINHO", 38, nov.AddDate(0, -1, 0))
	cardExpense(t, repo, pg, bank, "sc-4", "sc-teste IFOOD pedido", 30, nov.AddDate(0, -1, 0))

	if _, err := repo.RefreshSubcategories(ctx); err != nil {
		t.Fatal(err)
	}
	if sub, ess, _ := subcategoryOf(t, pg, ifood.ID); sub != "FOOD_DELIVERY" || ess == nil || *ess {
		t.Errorf("iFood = %s %v, quer delivery supérfluo", sub, ess)
	}
	if sub, ess, _ := subcategoryOf(t, pg, wash.ID); sub != "FOOD_DAILY" || ess == nil || !*ess {
		t.Errorf("marmitex = %s %v, quer refeição essencial", sub, ess)
	}
	// Rodar de novo não muda nada.
	if n, err := repo.RefreshSubcategories(ctx); err != nil || n != 0 {
		t.Errorf("segunda passada mudou %d, %v", n, err)
	}

	// Era o lava-rápido: corrige só esta, e marca como essencial à mão.
	yes := true
	found, others, err := repo.SetClassification(ctx, paymentOf(t, pg, wash.ID), domain.Classification{
		Category: domain.CategoryTransport, Subcategory: "TRANSPORT_WASH", Essential: &yes})
	if err != nil || !found || others != 0 {
		t.Fatalf("correção = %v %d %v", found, others, err)
	}
	if sub, ess, cat := subcategoryOf(t, pg, wash.ID); cat != "TRANSPORT" || sub != "TRANSPORT_WASH" || ess == nil || !*ess {
		t.Errorf("corrigida = %s %s %v", cat, sub, ess)
	}
	if _, _, cat := subcategoryOf(t, pg, wash2.ID); cat != "FOOD" {
		t.Errorf("a outra não devia mudar: %s", cat)
	}
	if n, _ := repo.RefreshSubcategories(ctx); n != 0 {
		t.Errorf("a correção à mão foi desfeita (%d mudaram)", n)
	}

	// Agora para todas do estabelecimento, deixando o essencial com o padrão (lava-rápido é supérfluo).
	found, others, err = repo.SetClassification(ctx, paymentOf(t, pg, wash.ID), domain.Classification{
		Category: domain.CategoryTransport, Subcategory: "TRANSPORT_WASH", ApplyToAll: true})
	if err != nil || !found || others != 1 {
		t.Fatalf("aplicar a todas = %v %d %v", found, others, err)
	}
	for _, p := range []*domain.Purchase{wash, wash2} {
		if sub, ess, cat := subcategoryOf(t, pg, p.ID); cat != "TRANSPORT" || sub != "TRANSPORT_WASH" || ess == nil || *ess {
			t.Errorf("estabelecimento = %s %s %v, quer lava-rápido supérfluo", cat, sub, ess)
		}
	}
	// E a próxima compra dele já chega certa.
	wash3 := cardExpense(t, repo, pg, bank, "sc-5", "sc-teste MARMITEX DO GORDINHO", 40, nov)
	if _, err := repo.RefreshSubcategories(ctx); err != nil {
		t.Fatal(err)
	}
	if sub, _, cat := subcategoryOf(t, pg, wash3.ID); cat != "TRANSPORT" || sub != "TRANSPORT_WASH" {
		t.Errorf("próxima = %s %s", cat, sub)
	}

	// Filtro e lista de transações.
	no := "false"
	txs, _, err := repo.DashboardRepo.Transactions(ctx, domain.TransactionFilter{Search: "sc-teste", Essential: no, Limit: 20})
	if err != nil || len(txs) != 5 {
		t.Fatalf("supérfluos = %d, %v", len(txs), err)
	}
	txs, _, _ = repo.DashboardRepo.Transactions(ctx, domain.TransactionFilter{Search: "sc-teste", Subcategory: "FOOD_DELIVERY", Limit: 20})
	if len(txs) != 2 || txs[0].Subcategory != "FOOD_DELIVERY" || txs[0].Essential == nil || *txs[0].Essential {
		t.Errorf("delivery = %+v", txs)
	}

	// Soma do mês por subcategoria, com a média dos 3 meses anteriores.
	spend, err := repo.SpendBySubcategory(ctx, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	got := map[domain.Subcategory]domain.SubcategorySpend{}
	for _, s := range spend {
		got[s.Subcategory] = s
	}
	if d := got["FOOD_DELIVERY"]; d.Total < 50 || d.PrevAverage < 10 {
		t.Errorf("delivery no mês = %+v", d)
	}
	if w := got["TRANSPORT_WASH"]; w.Total < 78 {
		t.Errorf("lava-rápido no mês = %+v", w)
	}
}
