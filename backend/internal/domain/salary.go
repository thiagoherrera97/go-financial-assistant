package domain

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SalaryPart é uma parte do salário: o dia combinado do mês e o valor líquido esperado.
type SalaryPart struct {
	Day    int
	Amount float64
}

// SalaryMatchDays é a distância máxima, em dias, entre a data prevista e o crédito real para contar como a mesma parte.
const SalaryMatchDays = 5

// ParseSalarySchedule lê "15=5200; 30=4366,62": partes separadas por ponto e vírgula, cada uma "dia=valor". O valor aceita
// vírgula ou ponto como decimal. Texto vazio é "sem salário configurado". As partes voltam em ordem de dia.
func ParseSalarySchedule(raw string) ([]SalaryPart, error) {
	var parts []SalaryPart
	seen := map[int]bool{}
	for _, chunk := range strings.Split(raw, ";") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		dayText, amountText, ok := strings.Cut(chunk, "=")
		if !ok {
			return nil, fmt.Errorf("%q: use dia=valor, como 15=5200", chunk)
		}
		day, err := strconv.Atoi(strings.TrimSpace(dayText))
		if err != nil || day < 1 || day > 31 {
			return nil, fmt.Errorf("%q: o dia vai de 1 a 31", chunk)
		}
		if seen[day] {
			return nil, fmt.Errorf("dia %d repetido", day)
		}
		seen[day] = true
		amountText = strings.TrimSpace(amountText)
		if strings.Contains(amountText, ",") {
			amountText = strings.ReplaceAll(strings.ReplaceAll(amountText, ".", ""), ",", ".")
		}
		amount, err := strconv.ParseFloat(amountText, 64)
		if err != nil || amount <= 0 || math.IsInf(amount, 0) || amount > 10_000_000 {
			return nil, fmt.Errorf("%q: valor inválido", chunk)
		}
		parts = append(parts, SalaryPart{Day: day, Amount: math.Round(amount*100) / 100})
	}
	if len(parts) > 6 {
		return nil, errors.New("no máximo 6 partes")
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Day < parts[j].Day })
	return parts, nil
}

// SalaryPayDate é quando a parte cai no mês: o dia combinado (limitado ao último dia do mês, então 30 em fevereiro vira o
// dia 28 ou 29) ou, se não for dia útil, o dia útil anterior.
func SalaryPayDate(year int, month time.Month, day int) time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return PreviousBusinessDay(time.Date(year, month, min(day, last), 0, 0, 0, 0, time.UTC))
}

// SalaryCredit é um crédito de salário que chegou do banco.
type SalaryCredit struct {
	Date   time.Time
	Amount float64
}

// SalaryStatus é uma parte do salário em um mês: quando deve cair, quanto, e se já caiu.
type SalaryStatus struct {
	Day      int
	Expected time.Time
	Amount   float64
	Received *SalaryCredit // nil = ainda a receber
}

// SalaryMonth confere as partes do mês contra os créditos de salário que chegaram. Cada parte fica com o crédito mais
// perto da data prevista, até SalaryMatchDays de distância; um crédito serve a uma parte só. Créditos extras (férias,
// 13º) não entram.
func SalaryMonth(parts []SalaryPart, month time.Time, credits []SalaryCredit) []SalaryStatus {
	out := make([]SalaryStatus, len(parts))
	for i, p := range parts {
		out[i] = SalaryStatus{Day: p.Day, Expected: SalaryPayDate(month.Year(), month.Month(), p.Day), Amount: p.Amount}
	}
	used := make([]bool, len(credits))
	type pair struct {
		part, credit int
		dist         float64
	}
	var pairs []pair
	for i, s := range out {
		for j, c := range credits {
			c := time.Date(c.Date.Year(), c.Date.Month(), c.Date.Day(), 0, 0, 0, 0, time.UTC)
			dist := math.Abs(c.Sub(s.Expected).Hours() / 24)
			if dist <= SalaryMatchDays {
				pairs = append(pairs, pair{i, j, dist})
			}
		}
	}
	sort.SliceStable(pairs, func(a, b int) bool { return pairs[a].dist < pairs[b].dist })
	for _, p := range pairs {
		if out[p.part].Received != nil || used[p.credit] {
			continue
		}
		c := credits[p.credit]
		out[p.part].Received = &c
		used[p.credit] = true
	}
	return out
}

// SalaryTotal soma o valor esperado das partes.
func SalaryTotal(parts []SalaryPart) float64 {
	var total float64
	for _, p := range parts {
		total += p.Amount
	}
	return total
}
