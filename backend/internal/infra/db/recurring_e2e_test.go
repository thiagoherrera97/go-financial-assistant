package db_test

// Recorrentes de ponta a ponta com o repositório real (sem mock): a geração diária, a leitura das ativas e o "cancelar X"
// pelo chat. Os mocks do usecase não pegam erro de leitura do banco (ex.: coluna faltando no SELECT).

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"
	"github.com/google/uuid"
)

func e2eRepo(t *testing.T) (ports.PurchaseRepository, *db.DB) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	var loc *time.Location
	if name := os.Getenv("TEST_DB_TIMEZONE"); name != "" {
		var err error
		if loc, err = time.LoadLocation(name); err != nil {
			t.Fatal(err)
		}
	}
	pg, err := db.NewPostgres(context.Background(), url, loc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)
	return db.NewPurchaseRepository(pg), pg
}

func saoPauloLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// e2eRecurring grava uma recorrente ativa (despesa, renda ou transferência) com descrição única e a apaga no fim.
func e2eRecurring(t *testing.T, repo ports.PurchaseRepository, pg *db.DB, kind domain.PurchaseKind, name string, day int) *domain.Purchase {
	t.Helper()
	desc := "teste-h4-e2e-" + name + "-" + uuid.NewString()[:8]
	var (
		p   *domain.Purchase
		err error
	)
	switch kind {
	case domain.KindIncome:
		p, err = domain.NewIncome(3000, &desc, domain.CategorySalary, domain.PaymentMethodPix, domain.PurchaseTypeRecurring, "teste")
	case domain.KindTransfer:
		p, err = domain.NewTransfer(200, &desc, domain.PaymentMethodPix, domain.PurchaseTypeRecurring, "teste", domain.TransferDirectionOut)
	default:
		p, err = domain.NewPurchase(55, &desc, domain.CategoryEntertainment, domain.PaymentMethodCreditCard, domain.PurchaseTypeRecurring, "teste")
	}
	if err != nil {
		t.Fatal(err)
	}
	p.DayOfMonth = &day
	if err := repo.Save(context.Background(), p, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM purchases WHERE id = $1`, p.ID) }) //nolint:errcheck
	return p
}

type e2ePayment struct {
	amount    float64
	status    string
	refMonth  time.Time
	createdAt time.Time
	hasDue    bool
}

func e2ePayments(t *testing.T, pg *db.DB, purchaseID uuid.UUID) []e2ePayment {
	t.Helper()
	rows, err := pg.Pool.Query(context.Background(),
		`SELECT amount, status, reference_month, created_at, due_date IS NOT NULL FROM payments WHERE purchase_id = $1`, purchaseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []e2ePayment
	for rows.Next() {
		var p e2ePayment
		if err := rows.Scan(&p.amount, &p.status, &p.refMonth, &p.createdAt, &p.hasDue); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func e2eUsecase(t *testing.T, repo ports.LedgerStore, analyzer ports.AIAnalyzer, now time.Time) *ledger.AnalyzeExpense {
	t.Helper()
	uc := ledger.NewAnalyzeExpense(repo, analyzer, slog.New(slog.DiscardHandler))
	uc.SetClock(domain.FixedClock{T: now.UTC()})
	uc.SetLocation(saoPauloLoc(t))
	return uc
}

// (a) Geração com o repositório real: despesa e renda do dia 5 com o app fora no dia (hoje 10/11) são recuperadas; a do
// dia 10 é lançada no dia; uma segunda execução não duplica.
func TestRecurringE2E_GenerateWithRealRepository(t *testing.T) {
	repo, pg := e2eRepo(t)
	loc := saoPauloLoc(t)
	expense := e2eRecurring(t, repo, pg, domain.KindExpense, "netflix", 5)
	income := e2eRecurring(t, repo, pg, domain.KindIncome, "salario", 5)
	today := e2eRecurring(t, repo, pg, domain.KindExpense, "academia", 10)
	later := e2eRecurring(t, repo, pg, domain.KindExpense, "aluguel", 20)

	now := time.Date(2026, 11, 10, 9, 0, 0, 0, loc)
	uc := e2eUsecase(t, repo, nil, now)
	for range 2 {
		if err := uc.GenerateRecurringExpenses(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	day5 := time.Date(2026, 11, 5, 0, 0, 0, 0, loc)
	for _, c := range []struct {
		name   string
		p      *domain.Purchase
		amount float64
	}{{"despesa", expense, 55}, {"renda", income, 3000}} {
		got := e2ePayments(t, pg, c.p.ID)
		if len(got) != 1 {
			t.Fatalf("%s do dia 5: esperado 1 lançamento, got %d", c.name, len(got))
		}
		g := got[0]
		if g.amount != c.amount || g.status != "PAID" || g.hasDue || !g.refMonth.Equal(nov) || !g.createdAt.Equal(day5) {
			t.Errorf("%s recuperada: %+v (esperado %.2f PAID, sem vencimento, mês %v, created_at %v)", c.name, g, c.amount, nov, day5)
		}
	}
	if got := e2ePayments(t, pg, today.ID); len(got) != 1 || got[0].createdAt.Equal(time.Date(2026, 11, 10, 0, 0, 0, 0, loc)) {
		t.Errorf("a do dia 10 é lançada no dia, sem carimbo retroativo: %+v", got)
	}
	if got := e2ePayments(t, pg, later.ID); len(got) != 0 {
		t.Errorf("a do dia 20 ainda não chegou: %+v", got)
	}
}

// (b) As leituras das ativas devolvem a compra inteira, com kind e transfer_direction.
func TestRecurringE2E_FindActiveAndByDescriptionReturnKind(t *testing.T) {
	repo, pg := e2eRepo(t)
	ctx := context.Background()
	expense := e2eRecurring(t, repo, pg, domain.KindExpense, "spotify", 7)
	income := e2eRecurring(t, repo, pg, domain.KindIncome, "aluguel-recebido", 7)
	transfer := e2eRecurring(t, repo, pg, domain.KindTransfer, "aporte", 7)

	actives, err := repo.FindActiveRecurring(ctx)
	if err != nil {
		t.Fatalf("FindActiveRecurring com recorrentes ativas: %v", err)
	}
	byID := map[uuid.UUID]domain.Purchase{}
	for _, p := range actives {
		byID[p.ID] = p
	}
	if p, ok := byID[expense.ID]; !ok || p.Kind != domain.KindExpense || p.TransferDirection != nil || p.DayOfMonth == nil || *p.DayOfMonth != 7 {
		t.Errorf("despesa: %+v (achada=%v)", p, ok)
	}
	if p, ok := byID[income.ID]; !ok || p.Kind != domain.KindIncome {
		t.Errorf("renda: %+v (achada=%v)", p, ok)
	}
	if p, ok := byID[transfer.ID]; !ok || p.Kind != domain.KindTransfer || p.TransferDirection == nil || *p.TransferDirection != domain.TransferDirectionOut {
		t.Errorf("transferência: %+v (achada=%v)", p, ok)
	}

	found, err := repo.FindByDescription(ctx, strings.TrimPrefix(*income.Description, "teste-h4-e2e-"))
	if err != nil {
		t.Fatalf("FindByDescription: %v", err)
	}
	if len(found) != 1 || found[0].ID != income.ID || found[0].Kind != domain.KindIncome || found[0].TotalAmount != 3000 {
		t.Errorf("FindByDescription: %+v", found)
	}
}

// cancelAnalyzer responde ao texto como a IA responde a "cancelar X".
type cancelAnalyzer struct{ what string }

func (a cancelAnalyzer) AnalyzeText(context.Context, string) (*ports.ExpenseAnalysis, error) {
	return &ports.ExpenseAnalysis{Type: ports.ExpenseTypeCancelRecurring, CancelInfo: &ports.CancelInfo{Description: a.what}, Confidence: 0.9}, nil
}
func (cancelAnalyzer) AnalyzeImage(context.Context, []byte, string) (*ports.ExpenseAnalysis, error) {
	return nil, nil
}
func (cancelAnalyzer) AnalyzeDocument(context.Context, []byte, string) (*ports.StatementAnalysis, error) {
	return nil, nil
}

// (c) "cancelar X" pelo chat, com o repositório real: a recorrente fica inativa e não é mais lançada, nem recuperada.
func TestRecurringE2E_CancelByNameStopsLaunching(t *testing.T) {
	repo, pg := e2eRepo(t)
	loc := saoPauloLoc(t)
	ctx := context.Background()
	netflix := e2eRecurring(t, repo, pg, domain.KindExpense, "netflix", 5)
	name := strings.TrimPrefix(*netflix.Description, "teste-h4-e2e-")

	uc := e2eUsecase(t, repo, cancelAnalyzer{what: name}, time.Date(2026, 11, 3, 12, 0, 0, 0, loc))
	out, err := uc.ExecuteText(ctx, ledger.TextInput{Text: "cancelar " + name})
	if err != nil {
		t.Fatalf("cancelar pelo chat: %v", err)
	}
	if !out.Cancelled || out.ID != netflix.ID.String() {
		t.Fatalf("saída do cancelamento: %+v", out)
	}
	var active bool
	if err := pg.Pool.QueryRow(ctx, `SELECT is_active FROM purchases WHERE id = $1`, netflix.ID).Scan(&active); err != nil || active {
		t.Fatalf("depois de cancelar: is_active=%v err=%v", active, err)
	}

	// No dia 10 (o dia 5 já passou), a cancelada não é recuperada.
	gen := e2eUsecase(t, repo, nil, time.Date(2026, 11, 10, 9, 0, 0, 0, loc))
	if err := gen.GenerateRecurringExpenses(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e2ePayments(t, pg, netflix.ID); len(got) != 0 {
		t.Errorf("recorrente cancelada não deveria ser lançada: %+v", got)
	}
}
