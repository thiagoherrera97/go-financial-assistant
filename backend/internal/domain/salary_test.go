package domain

import (
	"testing"
	"time"
)

func TestEasterSunday(t *testing.T) {
	for year, want := range map[int]time.Time{2024: ymd(2024, 3, 31), 2026: ymd(2026, 4, 5), 2027: ymd(2027, 3, 28), 2028: ymd(2028, 4, 16)} {
		if got := easterSunday(year); !got.Equal(want) {
			t.Errorf("Páscoa %d = %s, quer %s", year, got.Format("2006-01-02"), want.Format("2006-01-02"))
		}
	}
}

func TestIsBusinessDay(t *testing.T) {
	cases := []struct {
		day  time.Time
		want bool
		why  string
	}{
		{ymd(2026, 11, 13), true, "sexta comum"},
		{ymd(2026, 11, 14), false, "sábado"},
		{ymd(2026, 11, 15), false, "domingo e Proclamação"},
		{ymd(2026, 11, 20), false, "Consciência Negra"},
		{ymd(2026, 2, 16), false, "Carnaval segunda"},
		{ymd(2026, 2, 17), false, "Carnaval terça"},
		{ymd(2026, 2, 18), true, "quarta de cinzas"},
		{ymd(2026, 4, 3), false, "Sexta-feira Santa"},
		{ymd(2026, 6, 4), false, "Corpus Christi"},
		{ymd(2027, 1, 25), false, "aniversário de SP"},
		{ymd(2027, 7, 9), false, "9 de julho (SP)"},
		{ymd(2026, 12, 30), true, "quarta antes do ano novo"},
	}
	for _, c := range cases {
		if got := IsBusinessDay(c.day); got != c.want {
			t.Errorf("%s (%s) = %v, quer %v", c.day.Format("2006-01-02"), c.why, got, c.want)
		}
	}
}

func TestSalaryPayDate(t *testing.T) {
	cases := []struct {
		year  int
		month time.Month
		day   int
		want  time.Time
	}{
		{2026, time.November, 15, ymd(2026, 11, 13)}, // domingo e feriado → sexta
		{2026, time.November, 30, ymd(2026, 11, 30)},
		{2026, time.August, 15, ymd(2026, 8, 14)},    // sábado → sexta
		{2026, time.November, 20, ymd(2026, 11, 19)}, // feriado numa sexta → quinta
		{2027, time.February, 30, ymd(2027, 2, 26)},  // 28/02 é domingo → sexta 26
		{2028, time.February, 30, ymd(2028, 2, 25)},  // 29/02 é terça de Carnaval, 28 é segunda → sexta 25
		{2026, time.May, 30, ymd(2026, 5, 29)},       // sábado → sexta
		{2027, time.January, 25, ymd(2027, 1, 22)},   // feriado de SP na segunda → sexta
	}
	for _, c := range cases {
		if got := SalaryPayDate(c.year, c.month, c.day); !got.Equal(c.want) {
			t.Errorf("dia %d de %s/%d = %s, quer %s", c.day, c.month, c.year, got.Format("2006-01-02"), c.want.Format("2006-01-02"))
		}
	}
}

func TestParseSalarySchedule(t *testing.T) {
	parts, err := ParseSalarySchedule(" 30=4.366,62 ; 15=5200 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 || parts[0] != (SalaryPart{15, 5200}) || parts[1] != (SalaryPart{30, 4366.62}) {
		t.Fatalf("partes = %+v", parts)
	}
	if p, _ := ParseSalarySchedule("15=4366.62"); p[0].Amount != 4366.62 {
		t.Errorf("ponto decimal: %+v", p)
	}
	if p, err := ParseSalarySchedule(""); err != nil || p != nil {
		t.Errorf("vazio = %+v, %v", p, err)
	}
	for _, bad := range []string{"15", "0=10", "32=10", "15=abc", "15=-3", "15=10; 15=20", "x=10"} {
		if _, err := ParseSalarySchedule(bad); err == nil {
			t.Errorf("%q deveria falhar", bad)
		}
	}
}

func TestSalaryMonth(t *testing.T) {
	parts := []SalaryPart{{15, 5200}, {30, 4366.62}}
	month := ymd(2026, 11, 1)

	// Só a primeira caiu (dia 13); o crédito de outubro e o de férias ficam de fora.
	credits := []SalaryCredit{{Date: ymd(2026, 10, 30), Amount: 4366.62}, {Date: ymd(2026, 11, 13), Amount: 5200}, {Date: ymd(2026, 11, 3), Amount: 10797.79}}
	got, extras := SalaryMonth(parts, month, credits)
	if len(extras) != 1 || extras[0].Amount != 10797.79 {
		t.Errorf("extras = %+v: só o crédito de férias de novembro (o de outubro é de outro mês)", extras)
	}
	if len(got) != 2 {
		t.Fatalf("partes = %d", len(got))
	}
	if !got[0].Expected.Equal(ymd(2026, 11, 13)) || got[0].Received == nil || got[0].Received.Amount != 5200 {
		t.Errorf("dia 15 = %+v", got[0])
	}
	if !got[1].Expected.Equal(ymd(2026, 11, 30)) || got[1].Received != nil {
		t.Errorf("dia 30 = %+v (deve estar a receber)", got[1])
	}

	// Valor diferente do combinado ainda é a mesma parte; um crédito não serve a duas partes.
	got, _ = SalaryMonth(parts, ymd(2026, 9, 1), []SalaryCredit{{Date: ymd(2026, 9, 15), Amount: 5200}, {Date: ymd(2026, 9, 30), Amount: 9213.81}})
	if got[0].Received == nil || got[0].Received.Amount != 5200 || got[1].Received == nil || got[1].Received.Amount != 9213.81 {
		t.Errorf("setembro = %+v / %+v", got[0].Received, got[1].Received)
	}
	got, extras = SalaryMonth([]SalaryPart{{14, 100}, {16, 100}}, month, []SalaryCredit{{Date: ymd(2026, 11, 16), Amount: 100}})
	if len(extras) != 0 {
		t.Errorf("crédito usado não é extra: %+v", extras)
	}
	if got[0].Received != nil || got[1].Received == nil {
		t.Errorf("crédito único foi para a parte errada: %+v / %+v", got[0].Received, got[1].Received)
	}
}

func ymd(y, m, day int) time.Time { return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC) }

func TestSalaryMonthByValue(t *testing.T) {
	parts := []SalaryPart{{15, 5200}, {30, 4366.62}}
	dec := ymd(2026, 12, 1)

	// Um extra no mesmo dia do salário não toma o lugar dele: o valor decide.
	got, extras := SalaryMonth(parts, dec, []SalaryCredit{{Date: ymd(2026, 12, 15), Amount: 1800}, {Date: ymd(2026, 12, 15), Amount: 5150}})
	if got[0].Received == nil || got[0].Received.Amount != 5150 || got[0].Guessed {
		t.Errorf("dia 15 = %+v: deve ficar com o crédito de valor parecido, sem palpite", got[0].Received)
	}
	if len(extras) != 1 || extras[0].Amount != 1800 {
		t.Errorf("extras = %+v", extras)
	}

	// O salário cheio chega dia 18 (soma das duas partes): palpite de mês inteiro, nada fica a receber.
	got, extras = SalaryMonth(parts, dec, []SalaryCredit{{Date: ymd(2026, 12, 18), Amount: 9566.62}})
	for _, s := range got {
		if s.Received == nil || s.Received.Amount != 9566.62 || !s.Guessed {
			t.Errorf("dia %d = %+v (guessed=%v): o salário cheio cobre as duas partes, como palpite", s.Day, s.Received, s.Guessed)
		}
	}
	if len(extras) != 0 {
		t.Errorf("extras = %+v", extras)
	}

	// Valor fora do padrão perto do dia: palpite daquela parte (o comportamento de antes), marcado para conferir.
	got, _ = SalaryMonth(parts, dec, []SalaryCredit{{Date: ymd(2026, 12, 14), Amount: 7000}})
	if got[0].Received == nil || !got[0].Guessed || got[1].Received != nil {
		t.Errorf("palpite = %+v / %+v", got[0], got[1])
	}
}

func TestSalaryMonthDecisions(t *testing.T) {
	parts := []SalaryPart{{15, 5200}, {30, 4366.62}}
	dec, jan := ymd(2026, 12, 1), ymd(2027, 1, 1)

	// O salário de janeiro pago em dezembro: cobre janeiro inteiro e não é extra de dezembro.
	early := SalaryCredit{ID: "a", Date: ymd(2026, 12, 18), Amount: 9566.62, Kind: SalaryKindMonth, Month: jan}
	got, extras := SalaryMonth(parts, dec, []SalaryCredit{early})
	if got[0].Received != nil || got[1].Received != nil || len(extras) != 0 {
		t.Errorf("dezembro = %+v / %+v, extras %+v: o crédito é de janeiro", got[0].Received, got[1].Received, extras)
	}
	got, _ = SalaryMonth(parts, jan, []SalaryCredit{early})
	for _, s := range got {
		if s.Received == nil || s.Received.ID != "a" || s.Guessed {
			t.Errorf("janeiro dia %d = %+v", s.Day, s.Received)
		}
	}

	// Uma parte escolhida à mão, mesmo com valor e data fora do padrão.
	part := SalaryCredit{ID: "b", Date: ymd(2026, 12, 22), Amount: 7000, Kind: SalaryKindPart, Month: dec, Day: 30}
	got, _ = SalaryMonth(parts, dec, []SalaryCredit{part})
	if got[0].Received != nil || got[1].Received == nil || got[1].Received.ID != "b" {
		t.Errorf("parte escolhida = %+v / %+v", got[0].Received, got[1].Received)
	}

	// PLR confirmado nunca vira salário, nem perto do dia e com o valor da parte.
	plr := SalaryCredit{ID: "c", Date: ymd(2026, 12, 15), Amount: 5200, Kind: SalaryKindPLR}
	got, extras = SalaryMonth(parts, dec, []SalaryCredit{plr})
	if got[0].Received != nil || len(extras) != 1 || extras[0].Kind != SalaryKindPLR {
		t.Errorf("PLR = %+v, extras %+v", got[0].Received, extras)
	}
}

func TestSalaryReviews(t *testing.T) {
	parts := []SalaryPart{{15, 5200}, {30, 4366.62}}
	dec := ymd(2026, 12, 1)
	credits := []SalaryCredit{
		{ID: "ok", Date: ymd(2026, 12, 15), Amount: 5180},                         // bate: não pergunta
		{ID: "x", Date: ymd(2026, 12, 20), Amount: 2500},                          // extra sem decisão
		{ID: "d", Date: ymd(2026, 12, 21), Amount: 900, Kind: SalaryKindOvertime}, // já decidido
	}
	got := SalaryReviews(parts, dec, credits)
	if len(got) != 1 || got[0].Credit.ID != "x" || got[0].Suggest != "" {
		t.Fatalf("revisões = %+v: só o extra sem decisão, sem palpite", got)
	}

	full := SalaryReviews(parts, dec, []SalaryCredit{{ID: "f", Date: ymd(2026, 12, 18), Amount: 9566.62}})
	if len(full) != 1 || full[0].Suggest != SalaryKindMonth || !full[0].Month.Equal(dec) {
		t.Errorf("salário cheio = %+v: uma pergunta só, com palpite de mês inteiro", full)
	}
	one := SalaryReviews(parts, dec, []SalaryCredit{{ID: "p", Date: ymd(2026, 12, 29), Amount: 6100}})
	if len(one) != 1 || one[0].Suggest != SalaryKindPart || one[0].Day != 30 {
		t.Errorf("parte com valor diferente = %+v", one)
	}
}

func TestSalaryKind(t *testing.T) {
	if !SalaryKindPLR.IsExtra() || SalaryKindMonth.IsExtra() || SalaryKind("").IsExtra() {
		t.Error("IsExtra")
	}
	if SalaryKind("X").Valid() || !SalaryKindThirteenth.Valid() {
		t.Error("Valid")
	}
}
