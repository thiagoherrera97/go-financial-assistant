package domain

import (
	"testing"
	"time"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCardBillDue(t *testing.T) {
	cases := []struct {
		name     string
		purchase string
		dueDay   int
		close    int
		want     string
	}{
		{"depois do fechamento vai para o mês seguinte", "2026-10-20", 10, 6, "2026-11-10"},
		{"antes do fechamento fica na fatura do mês", "2026-11-03", 10, 6, "2026-11-10"},
		{"no dia do fechamento vai para a fatura seguinte", "2026-11-04", 10, 6, "2026-12-10"},
		{"fim do mês", "2026-11-30", 10, 6, "2026-12-10"},
		{"virada do ano", "2026-12-31", 10, 6, "2027-01-10"},
		{"cartão que vence dia 17, antes do fechamento", "2026-11-10", 17, 6, "2026-11-17"},
		{"cartão que vence dia 17, no fechamento", "2026-11-11", 17, 6, "2026-12-17"},
		{"vencimento no começo do mês fecha no mês anterior", "2026-10-26", 3, 6, "2026-11-03"},
		{"vencimento no começo do mês, no fechamento", "2026-10-28", 3, 6, "2026-12-03"},
		{"dia 31 vira o último dia de fevereiro", "2027-02-10", 31, 6, "2027-02-28"},
		{"hora do dia não importa", "2026-11-03T23:59:00Z", 10, 6, "2026-11-10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := time.Parse(time.RFC3339, tc.purchase)
			if err != nil {
				p = d(tc.purchase)
			}
			if got := CardBillDue(p, tc.dueDay, tc.close); !got.Equal(d(tc.want)) {
				t.Errorf("CardBillDue(%s, dia %d, %d dias) = %s, esperado %s", tc.purchase, tc.dueDay, tc.close, got.Format("2006-01-02"), tc.want)
			}
		})
	}
}

func TestCardInstallmentBillDue(t *testing.T) {
	if got := CardInstallmentBillDue(d("2026-12-20"), 10); !got.Equal(d("2026-12-10")) {
		t.Errorf("parcela datada em dezembro deveria vencer 10/12, got %s", got.Format("2006-01-02"))
	}
	if got := CardInstallmentBillDue(d("2027-02-05"), 30); !got.Equal(d("2027-02-28")) {
		t.Errorf("vencimento dia 30 em fevereiro deveria ser 28/02, got %s", got.Format("2006-01-02"))
	}
}

func TestCardCloseEstimate(t *testing.T) {
	if got := CardCloseEstimate(d("2026-12-10"), 6); !got.Equal(d("2026-12-04")) {
		t.Errorf("fechamento estimado deveria ser 04/12, got %s", got.Format("2006-01-02"))
	}
}

func TestCardForecastHorizon(t *testing.T) {
	// Hoje 20/11/2026, vencimento dia 31: 3 meses à frente é fevereiro, que só tem 28 dias.
	if got := CardForecastHorizon(time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), 31); !got.Equal(time.Date(2027, 2, 28, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("horizonte inesperado: %s", got)
	}
}

func TestReplacementDates(t *testing.T) {
	day := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	fmtAll := func(ts []time.Time) []string {
		out := make([]string, len(ts))
		for i, v := range ts {
			out[i] = v.Format("2006-01-02")
		}
		return out
	}
	// Série 18x que começou na fatura de maio, trocada a partir de dezembro; a fatura aberta é a de dezembro: só dezembro.
	if got := fmtAll(ReplacementDates(day("2026-05-10"), day("2026-12-10"), day("2026-12-10"), 18, 6)); len(got) != 1 || got[0] != "2026-12-06" {
		t.Errorf("só a fatura aberta deveria entrar, got %v", got)
	}
	// Abriu a de fevereiro: dezembro, janeiro e fevereiro.
	if got := fmtAll(ReplacementDates(day("2026-05-10"), day("2026-12-10"), day("2027-02-10"), 18, 6)); len(got) != 3 || got[2] != "2027-02-06" {
		t.Errorf("esperava dez, jan e fev, got %v", got)
	}
	// Depois da última parcela (outubro/2027) não entra mais nada.
	if got := fmtAll(ReplacementDates(day("2026-05-10"), day("2026-12-10"), day("2028-01-10"), 18, 6)); len(got) != 11 || got[10] != "2027-10-06" {
		t.Errorf("esperava 11 meses até outubro/2027, got %v", got)
	}
	// Antes de from_bill abrir, nada; dia 31 vira o último dia do mês.
	if got := ReplacementDates(day("2026-05-10"), day("2026-12-10"), day("2026-11-10"), 18, 6); len(got) != 0 {
		t.Errorf("fatura de from_bill ainda não abriu: nada, got %v", fmtAll(got))
	}
	if got := fmtAll(ReplacementDates(day("2026-01-10"), day("2027-02-10"), day("2027-02-10"), 18, 31)); len(got) != 1 || got[0] != "2027-02-28" {
		t.Errorf("dia 31 em fevereiro deveria ser 28/02, got %v", got)
	}
}
