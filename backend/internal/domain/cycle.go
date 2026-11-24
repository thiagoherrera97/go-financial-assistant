package domain

import (
	"sort"
	"time"
)

// CycleMode diz de onde vem o início do ciclo (configuração CYCLE_START).
type CycleMode string

const (
	CycleSalary   CycleMode = "salary"    // na 1ª parte do salário (SALARY_SCHEDULE); sem salário, o mês do calendário
	CycleFixedDay CycleMode = "fixed_day" // num dia fixo do mês (CYCLE_DAY), no dia útil anterior se cair em folga
	CycleCalendar CycleMode = "calendar"  // do dia 1 ao último dia do mês
)

// CycleRule é a regra configurada para os ciclos.
type CycleRule struct {
	Mode CycleMode
	Day  int // só em CycleFixedDay: 1 a 31 (limitado ao último dia do mês)
}

// CycleStart é o início de um ciclo do salário. Key é o primeiro dia do mês que dá nome ao ciclo (o mês do dia combinado
// da 1ª parte do salário); Start, o dia em que ele começa. Ambos à meia-noite em UTC.
type CycleStart struct {
	Key   time.Time
	Start time.Time
}

// BuildCycles calcula o início de cada ciclo, uma entrada por mês de from até to. O ciclo começa no dia útil da 1ª parte
// do salário (SalaryPayDate) ou, se o crédito real dessa parte chegou antes, no dia do crédito. Sem salário configurado
// não há ciclos (nil): as telas usam o mês do calendário.
func BuildCycles(parts []SalaryPart, credits []SalaryCredit, from, to time.Time) []CycleStart {
	if len(parts) == 0 {
		return nil
	}
	var out []CycleStart
	for key := monthKey(from); !key.After(monthKey(to)); key = key.AddDate(0, 1, 0) {
		status, _ := SalaryMonth(parts, key, credits)
		start := status[0].Expected
		if r := status[0].Received; r != nil {
			if d := dayOf(r.Date); d.Before(start) {
				start = d
			}
		}
		out = append(out, CycleStart{Key: key, Start: start})
	}
	return out
}

// BuildFixedDayCycles calcula ciclos que começam todo mês no mesmo dia (limitado ao último dia do mês; fim de semana ou
// feriado volta para o dia útil anterior, a mesma regra do salário). Dia 1 ou fora de 1..31 é o mês do calendário (nil).
func BuildFixedDayCycles(day int, from, to time.Time) []CycleStart {
	if day <= 1 || day > 31 {
		return nil
	}
	var out []CycleStart
	for key := monthKey(from); !key.After(monthKey(to)); key = key.AddDate(0, 1, 0) {
		out = append(out, CycleStart{Key: key, Start: SalaryPayDate(key.Year(), key.Month(), day)})
	}
	return out
}

// Cycles responde em que ciclo cai um dia e onde um ciclo começa e termina. Vazio, é o mês do calendário.
type Cycles struct {
	starts []CycleStart // ordenados por Start
}

// NewCycles monta os ciclos a partir dos inícios (em qualquer ordem).
func NewCycles(starts []CycleStart) Cycles {
	s := append([]CycleStart(nil), starts...)
	sort.Slice(s, func(i, j int) bool { return s[i].Start.Before(s[j].Start) })
	return Cycles{starts: s}
}

// KeyOf é a chave do ciclo em que day cai; antes do primeiro ciclo conhecido, o mês do calendário.
func (c Cycles) KeyOf(day time.Time) time.Time {
	d := dayOf(day)
	i := sort.Search(len(c.starts), func(i int) bool { return c.starts[i].Start.After(d) })
	if i == 0 {
		return monthKey(d)
	}
	return c.starts[i-1].Key
}

// Bounds é o primeiro e o último dia do ciclo key. Chave desconhecida é o mês do calendário; o último ciclo conhecido
// termina um mês depois do início (a tabela vai bem além de hoje, então isso só vale longe no futuro).
func (c Cycles) Bounds(key time.Time) (start, end time.Time) {
	k := monthKey(key)
	for i, s := range c.starts {
		if s.Key.Equal(k) {
			if i+1 < len(c.starts) {
				return s.Start, c.starts[i+1].Start.AddDate(0, 0, -1)
			}
			return s.Start, s.Start.AddDate(0, 1, -1)
		}
	}
	return k, k.AddDate(0, 1, -1)
}

// DayOf é a posição de day no seu ciclo (o primeiro dia é 1) e quantos dias o ciclo tem: o ritmo das metas e do Coach.
func (c Cycles) DayOf(day time.Time) (n, total int) {
	start, end := c.Bounds(c.KeyOf(day))
	return int(dayOf(day).Sub(start).Hours()/24) + 1, int(end.Sub(start).Hours()/24) + 1
}

func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func monthKey(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC) }
