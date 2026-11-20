package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestForecastCardInstallments(t *testing.T) {
	acc, other := uuid.New(), uuid.New()
	known := []CardInstallment{
		// Alianças: 2/10 em set, 3/10 em out. Faltam 4/10 (nov) e 5/10 (dez) até o horizonte.
		{AccountID: acc, Base: "VIVARA", Number: 2, Total: 10, Amount: 669, Category: CategoryShopping, BillDue: d("2026-09-10")},
		{AccountID: acc, Base: "VIVARA", Number: 3, Total: 10, Amount: 669, Category: CategoryShopping, BillDue: d("2026-10-10")},
		// Duas compras iguais no mesmo mês: duas cópias de cada parcela (a caixa da descrição não importa).
		{AccountID: acc, Base: "SEM PARAR", Number: 7, Total: 18, Amount: 18.67, BillDue: d("2026-10-10")},
		{AccountID: acc, Base: "sem parar", Number: 7, Total: 18, Amount: 18.67, BillDue: d("2026-10-10")},
		// Terminou em outubro: nada a prever.
		{AccountID: acc, Base: "MOBLY", Number: 10, Total: 10, Amount: 341.24, BillDue: d("2026-10-10")},
		// Mesma descrição e valor, mas começou em outro mês: é outra compra.
		{AccountID: acc, Base: "VIVARA", Number: 1, Total: 10, Amount: 669, BillDue: d("2026-10-10")},
		// Última parcela: 5/6 em out, só falta 6/6 em nov.
		{AccountID: acc, Base: "AMAZON", Number: 5, Total: 6, Amount: 65.95, BillDue: d("2026-10-10")},
		// Parou em agosto (terminou ou mudou de nome no banco): não se prevê, porque outubro já tem parcelas.
		{AccountID: acc, Base: "PAROU", Number: 2, Total: 10, Amount: 50, BillDue: d("2026-08-10")},
		// Cartão sem horizonte: ignorado.
		{AccountID: other, Base: "X", Number: 1, Total: 3, Amount: 10, BillDue: d("2026-10-17")},
	}
	got := ForecastCardInstallments(known, map[uuid.UUID]int{acc: 10, other: 17}, map[uuid.UUID]time.Time{acc: d("2026-12-10")})

	count := map[string]int{}
	var nov float64
	for _, f := range got {
		count[f.Description()+"@"+f.BillDue.Format("2006-01-02")]++
		if f.BillDue.Equal(d("2026-11-10")) {
			nov += f.Amount
		}
		if f.Base == "PAROU" {
			t.Errorf("compra que parou antes da última fatura com parcelas não deveria ser prevista: %+v", f)
		}
		if f.AccountID == other {
			t.Errorf("cartão sem horizonte não deveria ter previsão: %+v", f)
		}
	}
	want := map[string]int{
		"VIVARA (4/10)@2026-11-10":    1,
		"VIVARA (5/10)@2026-12-10":    1,
		"VIVARA (2/10)@2026-11-10":    1,
		"VIVARA (3/10)@2026-12-10":    1,
		"SEM PARAR (8/18)@2026-11-10": 2,
		"SEM PARAR (9/18)@2026-12-10": 2,
		"AMAZON (6/6)@2026-11-10":     1,
	}
	for k, n := range want {
		if count[k] != n {
			t.Errorf("%s: esperava %d, got %d (todas: %v)", k, n, count[k], count)
		}
	}
	if len(got) != 9 {
		t.Errorf("esperava 9 previstas, got %d: %v", len(got), count)
	}
	if nov != 669+669+18.67*2+65.95 {
		t.Errorf("novembro previsto errado: %.2f", nov)
	}

	// A chave é estável e única.
	keys := map[string]bool{}
	for _, f := range got {
		if keys[f.Key] {
			t.Errorf("chave repetida: %s", f.Key)
		}
		keys[f.Key] = true
	}
	again := ForecastCardInstallments(known, map[uuid.UUID]int{acc: 10, other: 17}, map[uuid.UUID]time.Time{acc: d("2026-12-10")})
	for i := range got {
		if got[i].Key != again[i].Key {
			t.Fatalf("a chave deveria ser a mesma entre execuções: %s x %s", got[i].Key, again[i].Key)
		}
	}
}

func TestForecastCardInstallments_DueDayClamp(t *testing.T) {
	acc := uuid.New()
	got := ForecastCardInstallments([]CardInstallment{{AccountID: acc, Base: "X", Number: 1, Total: 3, Amount: 10, BillDue: d("2027-01-31")}},
		map[uuid.UUID]int{acc: 31}, map[uuid.UUID]time.Time{acc: d("2027-03-31")})
	if len(got) != 2 || !got[0].BillDue.Equal(d("2027-02-28")) || !got[1].BillDue.Equal(d("2027-03-31")) {
		t.Errorf("vencimento dia 31 deveria virar 28/02 e 31/03, got %+v", got)
	}
}

func TestCardBillGap(t *testing.T) {
	cases := []struct {
		total, known, want float64
	}{
		{8583.09, 8000, 583.09},
		{100, 100.2, 0},
		{100, 99.7, 0},
		{100, 99.4, 0.6},
		{100, 150, -50},
		{100, 100.4, 0},
	}
	for _, c := range cases {
		if got := CardBillGap(c.total, c.known); got != c.want {
			t.Errorf("CardBillGap(%.2f, %.2f) = %.2f, esperado %.2f", c.total, c.known, got, c.want)
		}
	}
}

func TestForecastCardInstallments_OnlyAfterLastDeliveredBill(t *testing.T) {
	acc := uuid.New()
	// O banco já entregou a 4/6 em novembro: nada se prevê para novembro, só dezembro (5/6).
	known := []CardInstallment{
		{AccountID: acc, Base: "A", Number: 3, Total: 6, Amount: 10, BillDue: d("2026-10-10")},
		{AccountID: acc, Base: "A", Number: 4, Total: 6, Amount: 10, BillDue: d("2026-11-10")},
	}
	got := ForecastCardInstallments(known, map[uuid.UUID]int{acc: 10}, map[uuid.UUID]time.Time{acc: d("2026-12-10")})
	if len(got) != 1 || got[0].Number != 5 || !got[0].BillDue.Equal(d("2026-12-10")) {
		t.Errorf("esperava só a 5/6 em dezembro, got %+v", got)
	}
}
