package domain

import "time"

// Holidays são os dias sem expediente bancário no ano: feriados nacionais, Carnaval (segunda e terça), Sexta-feira
// Santa, Corpus Christi e os feriados de São Paulo (aniversário da cidade em 25/01 e Revolução Constitucionalista em 09/07).
// As datas vêm como meia-noite UTC.
func Holidays(year int) map[time.Time]bool {
	d := func(m time.Month, day int) time.Time { return time.Date(year, m, day, 0, 0, 0, 0, time.UTC) }
	easter := easterSunday(year)
	days := []time.Time{
		d(time.January, 1),        // Confraternização Universal
		d(time.January, 25),       // Aniversário de São Paulo
		d(time.April, 21),         // Tiradentes
		d(time.May, 1),            // Dia do Trabalho
		d(time.July, 9),           // Revolução Constitucionalista (SP)
		d(time.September, 7),      // Independência
		d(time.October, 12),       // Nossa Senhora Aparecida
		d(time.November, 2),       // Finados
		d(time.November, 15),      // Proclamação da República
		d(time.November, 20),      // Dia da Consciência Negra
		d(time.December, 25),      // Natal
		easter.AddDate(0, 0, -48), // Carnaval (segunda)
		easter.AddDate(0, 0, -47), // Carnaval (terça)
		easter.AddDate(0, 0, -2),  // Sexta-feira Santa
		easter.AddDate(0, 0, 60),  // Corpus Christi
	}
	out := make(map[time.Time]bool, len(days))
	for _, h := range days {
		out[h] = true
	}
	return out
}

// easterSunday é o domingo de Páscoa (algoritmo de Meeus/Jones/Butcher, calendário gregoriano).
func easterSunday(year int) time.Time {
	a := year % 19
	b, c := year/100, year%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// IsBusinessDay diz se o dia (pela data, ignorando a hora) tem expediente bancário.
func IsBusinessDay(t time.Time) bool {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	return !Holidays(day.Year())[day]
}

// PreviousBusinessDay é o próprio dia, se útil, ou o último dia útil antes dele (data em UTC).
func PreviousBusinessDay(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	for !IsBusinessDay(day) {
		day = day.AddDate(0, 0, -1)
	}
	return day
}
