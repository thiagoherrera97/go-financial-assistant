package db

import (
	"context"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestRebuildCardForecasts(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	day := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	due := day("2026-11-10")
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "fc-test-card", ItemID: "fc-test", Type: "CREDIT", Name: "SANTANDER TESTE", Last4: "8550", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pg.Pool.Exec(context.Background(), `DELETE FROM purchases WHERE id IN (SELECT purchase_id FROM payments WHERE account_id = $1)`, card)
		pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card)
	})

	// Parcela 3/10 na fatura de outubro (a sincronização data a parcela no mês da fatura) e uma compra à vista em novembro.
	cardExpense(t, repo, pg, card, "fc-t1", "fc-teste VIVARA (3/10)", 669, day("2026-10-15"))
	cardExpense(t, repo, pg, card, "fc-t2", "fc-teste mercado", 100, day("2026-10-20"))
	// Compra de 18/11 já na fatura aberta (10/12): o cartão está em uso até ela, então a previsão vai até dezembro.
	cardExpense(t, repo, pg, card, "fc-t0", "fc-teste padaria", 12, day("2026-11-18"))
	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveCardBills(ctx, card, []ports.ExternalBill{
		{ID: "fc-bill-nov", DueDate: day("2026-11-10"), Total: 1000},
		{ID: "fc-bill-out", DueDate: day("2026-10-10"), Total: 669}, // já coberta: sem ajuste
		{ID: "fc-bill-old", DueDate: day("2025-01-10"), Total: 500}, // antes do histórico: ignorada
	}); err != nil {
		t.Fatal(err)
	}

	// Hoje 20/11: a fatura aberta é a de 10/12, então prevê 4/10 (nov) e 5/10 (dez).
	f, adj, err := repo.RebuildCardForecasts(ctx, 6, day("2026-11-20"))
	if err != nil {
		t.Fatal(err)
	}
	if f != 2 || adj != 1 {
		t.Fatalf("esperava 2 previstas e 1 ajuste, got %d e %d", f, adj)
	}

	type row struct {
		desc   string
		amount float64
		status string
		bill   time.Time
	}
	read := func() []row {
		rows, err := pg.Pool.Query(ctx, `SELECT p.description, pay.amount, pay.status, pay.bill_due_date FROM payments pay
			JOIN purchases p ON p.id = pay.purchase_id WHERE pay.account_id = $1 AND pay.external_id LIKE 'finassist:%'
			ORDER BY pay.bill_due_date, p.description`, card)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.desc, &r.amount, &r.status, &r.bill); err != nil {
				t.Fatal(err)
			}
			out = append(out, r)
		}
		return out
	}
	got := read()
	if len(got) != 3 {
		t.Fatalf("esperava 3 lançamentos do app, got %+v", got)
	}
	// Novembro: mercado 100 + VIVARA 4/10 prevista 669 = 769; o banco cobrou 1000 → ajuste de 231.
	if got[0].desc != "Fatura SANTANDER TESTE 10/11 · não detalhado no Open Finance" || got[0].amount != 231 || got[0].status != "PAID" {
		t.Errorf("ajuste inesperado: %+v", got[0])
	}
	if got[1].desc != "fc-teste VIVARA (4/10)" || got[1].status != "PENDING" || !got[1].bill.Equal(day("2026-11-10")) {
		t.Errorf("prevista de novembro inesperada: %+v", got[1])
	}
	if got[2].desc != "fc-teste VIVARA (5/10)" || !got[2].bill.Equal(day("2026-12-10")) {
		t.Errorf("prevista de dezembro inesperada: %+v", got[2])
	}

	// A nota escrita num lançamento do app (ajuste da fatura ou parcela prevista) sobrevive ao refazer.
	if _, err := pg.Pool.Exec(ctx, `UPDATE payments SET note = 'carro 8/12 + alianças 7/10' WHERE external_id = 'finassist:bill:fc-bill-nov'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Pool.Exec(ctx, `UPDATE payments pay SET note = 'alianças 5/10' FROM purchases p
		WHERE p.id = pay.purchase_id AND pay.account_id = $1 AND p.description = 'fc-teste VIVARA (5/10)'`, card); err != nil {
		t.Fatal(err)
	}
	noteOf := func(where string) string {
		var n *string
		if err := pg.Pool.QueryRow(ctx, `SELECT pay.note FROM payments pay JOIN purchases p ON p.id = pay.purchase_id
			WHERE pay.account_id = $1 AND `+where, card).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == nil {
			return ""
		}
		return *n
	}

	// Refazer não duplica; quando a parcela real chega, a prevista some e o ajuste diminui.
	if _, _, err := repo.RebuildCardForecasts(ctx, 6, day("2026-11-20")); err != nil {
		t.Fatal(err)
	}
	if n := len(read()); n != 3 {
		t.Errorf("refazer deveria manter 3, got %d", n)
	}
	if got := noteOf(`pay.external_id = 'finassist:bill:fc-bill-nov'`); got != "carro 8/12 + alianças 7/10" {
		t.Errorf("nota do ajuste da fatura perdida ao refazer: %q", got)
	}
	if got := noteOf(`p.description = 'fc-teste VIVARA (5/10)'`); got != "alianças 5/10" {
		t.Errorf("nota da parcela prevista perdida ao refazer: %q", got)
	}
	cardExpense(t, repo, pg, card, "fc-t3", "fc-teste VIVARA (4/10)", 669, day("2026-11-15"))
	cardExpense(t, repo, pg, card, "fc-t4", "fc-teste farmácia", 31, day("2026-10-25"))
	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.RebuildCardForecasts(ctx, 6, day("2026-11-20")); err != nil {
		t.Fatal(err)
	}
	got = read()
	if len(got) != 2 || got[0].amount != 200 || got[1].desc != "fc-teste VIVARA (5/10)" {
		t.Errorf("com a 4/10 real, sobra só a 5/10 prevista e o ajuste cai para 200: %+v", got)
	}

	// O mês de novembro soma exatamente o total do banco.
	var nov float64
	if err := pg.Pool.QueryRow(ctx, `SELECT SUM(pay.amount) FROM payments pay WHERE pay.account_id = $1 AND `+paymentMonth+` = '2026-11-01'`, card).Scan(&nov); err != nil {
		t.Fatal(err)
	}
	if nov != 1000 {
		t.Errorf("novembro deveria bater com a fatura do banco (1000), got %.2f", nov)
	}

	// Quando o app passa do banco (crédito que não veio), o ajuste é negativo e o mês bate do mesmo jeito.
	if err := repo.SaveCardBills(ctx, card, []ports.ExternalBill{{ID: "fc-bill-out", DueDate: day("2026-10-10"), Total: 600}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.RebuildCardForecasts(ctx, 6, day("2026-11-20")); err != nil {
		t.Fatal(err)
	}
	var octDesc string
	var octAdj, oct float64
	if err := pg.Pool.QueryRow(ctx, `SELECT p.description, pay.amount FROM payments pay JOIN purchases p ON p.id = pay.purchase_id
		WHERE pay.external_id = 'finassist:bill:fc-bill-out'`).Scan(&octDesc, &octAdj); err != nil {
		t.Fatal(err)
	}
	if octAdj != -69 || octDesc != "Fatura SANTANDER TESTE 10/10 · créditos não detalhados no Open Finance" {
		t.Errorf("ajuste negativo inesperado: %q %.2f", octDesc, octAdj)
	}
	if err := pg.Pool.QueryRow(ctx, `SELECT SUM(pay.amount) FROM payments pay WHERE pay.account_id = $1 AND `+paymentMonth+` = '2026-10-01'`, card).Scan(&oct); err != nil {
		t.Fatal(err)
	}
	if oct != 600 {
		t.Errorf("outubro deveria bater com a fatura do banco (600), got %.2f", oct)
	}
}

func TestRebuildCardForecasts_IdleCardGetsNoForecast(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	day := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	due := day("2026-11-17")
	card, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "fc-idle-card", ItemID: "fc-idle", Type: "CREDIT", Name: "PARADO", Last4: "1473", DueDate: &due})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pg.Pool.Exec(context.Background(), `DELETE FROM purchases WHERE id IN (SELECT purchase_id FROM payments WHERE account_id = $1)`, card)
		pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, card)
	})
	// Última compra em agosto: o cartão parou; a 3/12 não vira previsão de setembro a dezembro.
	cardExpense(t, repo, pg, card, "fc-idle-1", "fc-teste ML (3/12)", 32.24, day("2026-08-05"))
	if _, err := repo.RecomputeCardBills(ctx, 6); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.RebuildCardForecasts(ctx, 6, day("2026-11-20")); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE account_id = $1 AND external_id LIKE 'finassist:%'`, card).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("cartão parado não deveria ganhar previsões, got %d", n)
	}
}
