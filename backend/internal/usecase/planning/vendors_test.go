package planning

import (
	"math"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func vendor(name string, contracted, paid float64, due *time.Time) domain.Vendor {
	v := domain.Vendor{ID: uuid.New(), Name: name, Contracted: contracted, DueDate: due}
	if paid > 0 {
		v.Payments = []domain.VendorPayment{{Amount: paid}}
	}
	return v
}

// spreadsheetVendors são os fornecedores da aba FORNECEDORES da planilha.
func spreadsheetVendors() []domain.Vendor {
	return []domain.Vendor{
		vendor("Churrasgrill", 18489.98, 4000, day(2027, time.August, 23)),
		vendor("Suzi Decoradora", 18800, 3840, day(2027, time.August, 14)),
		vendor("Assessoria Vivá", 3895, 2960.2, day(2027, time.January, 5)),
		vendor("Amanda fotógrafa", 4680, 1404, day(2027, time.August, 9)),
		vendor("Rock bar", 3100, 310, day(2027, time.August, 19)),
		vendor("Espaço awa", 6000, 6000, nil),
	}
}

var plan10 = domain.VendorPlan{Name: "Casamento", ContributionDay: 10}

func TestBuildVendorSummary_MatchesSpreadsheet(t *testing.T) {
	today := time.Date(2026, time.November, 20, 15, 0, 0, 0, time.UTC)
	s := BuildVendorSummary(plan10, spreadsheetVendors(), nil, Projection{}, today)

	if s.Contracted != 54964.98 || s.Paid != 18514.2 || s.Saved != 0 || s.Remaining != 36450.78 {
		t.Fatalf("totais: contratado=%v pago=%v guardado=%v falta=%v", s.Contracted, s.Paid, s.Saved, s.Remaining)
	}
	if !s.NextContribution.Equal(*day(2026, time.December, 10)) {
		t.Errorf("próximo aporte: %v", s.NextContribution)
	}
	// O último vencimento sai dos fornecedores (23/08), não de uma data digitada.
	if s.LastDue == nil || !s.LastDue.Equal(*day(2027, time.August, 23)) || s.MonthsLeft != 9 {
		t.Errorf("último vencimento e meses: %v %d", s.LastDue, s.MonthsLeft)
	}
	if s.FlatPerMonth != 4050.09 || s.PerMonth != 4050.09 || s.Tight != nil {
		t.Errorf("guardar por mês: flat=%v per=%v tight=%v", s.FlatPerMonth, s.PerMonth, s.Tight)
	}
	if s.Surplus != nil || s.Fits != nil {
		t.Errorf("sem histórico não há sobra: %v %v", s.Surplus, s.Fits)
	}
	// Ordem: com prazo pelo vencimento, quitado por último.
	if s.Vendors[0].Vendor.Name != "Assessoria Vivá" || s.Vendors[0].Contributions != 1 || s.Vendors[5].Vendor.Name != "Espaço awa" || !s.Vendors[5].Done {
		t.Errorf("ordem: %s (%d aportes) … %s", s.Vendors[0].Vendor.Name, s.Vendors[0].Contributions, s.Vendors[5].Vendor.Name)
	}
	if math.Abs(s.Vendors[0].Remaining-934.8) > 0.001 {
		t.Errorf("falta da assessoria: %v", s.Vendors[0].Remaining)
	}
}

func TestBuildVendorSummary_EarlyDeadlineRaisesPerMonth(t *testing.T) {
	today := time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC)
	vendors := []domain.Vendor{
		vendor("Buffet", 12000, 0, day(2027, time.August, 30)),
		vendor("Vestido", 6000, 0, day(2027, time.January, 15)),
	}
	s := BuildVendorSummary(plan10, vendors, nil, Projection{}, today)
	// Média: 18000 em 9 aportes = 2000. Mas o vestido vence em jan: 6000 em 2 aportes (dez e jan) = 3000.
	if s.FlatPerMonth != 2000 || s.PerMonth != 3000 || s.Tight == nil || s.Tight.Name != "Vestido" {
		t.Fatalf("prazo apertado: flat=%v per=%v tight=%v", s.FlatPerMonth, s.PerMonth, s.Tight)
	}

	// Com 4000 guardados na caixinha, o vestido precisa de 1000/aporte e a média manda (14000/9).
	s = BuildVendorSummary(domain.VendorPlan{ContributionDay: 10, SavingsBox: "Casamento"}, vendors, map[string]float64{"Casamento": 4000}, Projection{}, today)
	if !s.SavedFromBox || s.Saved != 4000 || s.Remaining != 14000 || s.PerMonth != 1555.56 || s.Tight != nil {
		t.Errorf("com caixinha: %+v", s)
	}
}

func TestBuildVendorSummary_BoxMissingAndLate(t *testing.T) {
	today := time.Date(2027, time.February, 12, 0, 0, 0, 0, time.UTC)
	vendors := []domain.Vendor{vendor("Convites", 800, 200, day(2027, time.March, 5))}
	// Caixinha que sumiu das contas não cai no valor à mão: guardado fica 0 e o app avisa.
	s := BuildVendorSummary(domain.VendorPlan{ContributionDay: 10, SavingsBox: "Sumiu", SavedManual: 999}, vendors, map[string]float64{}, Projection{}, today)
	if s.SavedFromBox || s.Saved != 0 {
		t.Errorf("caixinha ausente: %v %v", s.SavedFromBox, s.Saved)
	}
	// Próximo aporte 10/03 vem depois do vencimento 05/03: atrasado, precisa de tudo já.
	if !s.NextContribution.Equal(*day(2027, time.March, 10)) || !s.Vendors[0].Late || s.MonthsLeft != 1 || s.PerMonth != 600 {
		t.Errorf("atrasado: next=%v late=%v meses=%d per=%v", s.NextContribution, s.Vendors[0].Late, s.MonthsLeft, s.PerMonth)
	}
}

func TestBuildVendorSummary_FitsSurplus(t *testing.T) {
	p := Projection{Assumptions: Assumptions{Income: 8000, Fixed: 3000, Variable: 2000, BasedOn: 3}}
	today := time.Date(2026, time.November, 5, 0, 0, 0, 0, time.UTC)
	s := BuildVendorSummary(plan10, []domain.Vendor{vendor("Buffet", 10000, 0, day(2027, time.August, 10))}, nil, p, today)
	// Hoje (5) vem antes do dia 10: o aporte de novembro conta. 10 aportes de nov a ago = 1000, cabe em 3000.
	if s.MonthsLeft != 10 || s.PerMonth != 1000 || s.Surplus == nil || *s.Surplus != 3000 || s.Fits == nil || !*s.Fits {
		t.Errorf("cabe na sobra: %+v", s)
	}
}

func TestBuildVendorSummary_AllPaid(t *testing.T) {
	s := BuildVendorSummary(plan10, []domain.Vendor{vendor("Espaço", 6000, 6000, day(2027, time.August, 1))}, nil, Projection{}, time.Now())
	if s.Remaining != 0 || s.PerMonth != 0 || s.LastDue != nil || s.MonthsLeft != 0 {
		t.Errorf("tudo pago: %+v", s)
	}
}
