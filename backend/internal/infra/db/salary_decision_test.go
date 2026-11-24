package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func salarySeed(t *testing.T, repo *testRepo, pg *DB, category domain.Category, income bool, amount float64, date time.Time) uuid.UUID {
	t.Helper()
	desc := "teste-salario"
	var (
		p   *domain.Purchase
		err error
	)
	if income {
		p, err = domain.NewIncome(amount, &desc, category, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "teste")
	} else {
		p, err = domain.NewPurchase(amount, &desc, category, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "teste")
	}
	if err != nil {
		t.Fatal(err)
	}
	pay := domain.NewPayment(p.ID, amount, domain.PaymentStatusPaid)
	pay.DueDate = &date
	if err := repo.Save(context.Background(), p, []domain.Payment{*pay}); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)
	return pay.ID
}

func salaryCreditByID(t *testing.T, repo *testRepo, id uuid.UUID, date time.Time) domain.SalaryCredit {
	t.Helper()
	credits, err := repo.SalaryCredits(context.Background(), date, date)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range credits {
		if c.ID == id.String() {
			return c
		}
	}
	t.Fatalf("crédito %s não voltou em SalaryCredits", id)
	return domain.SalaryCredit{}
}

func TestSalaryDecisions(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	date := time.Date(1999, 3, 18, 0, 0, 0, 0, time.UTC)
	salary := salarySeed(t, repo, pg, domain.CategorySalary, true, 9566.62, date)
	food := salarySeed(t, repo, pg, domain.CategoryFood, false, 50, date)

	if c := salaryCreditByID(t, repo, salary, date); c.Kind != "" || c.Amount != 9566.62 || c.Label != "teste-salario" {
		t.Fatalf("sem decisão = %+v", c)
	}
	if err := repo.DecideSalary(ctx, food, domain.SalaryKindPLR, nil, 0); !errors.Is(err, domain.ErrSalaryCreditNotFound) {
		t.Fatalf("despesa não é crédito de salário: %v", err)
	}

	month := time.Date(1999, 4, 1, 0, 0, 0, 0, time.UTC)
	if err := repo.DecideSalary(ctx, salary, domain.SalaryKindMonth, &month, 15); err != nil {
		t.Fatal(err)
	}
	if c := salaryCreditByID(t, repo, salary, date); c.Kind != domain.SalaryKindMonth || !c.Month.Equal(month) || c.Day != 0 {
		t.Fatalf("mês inteiro = %+v (o dia só vale para PART)", c)
	}

	// Trocar de ideia regrava; um extra não guarda mês.
	if err := repo.DecideSalary(ctx, salary, domain.SalaryKindPLR, &month, 0); err != nil {
		t.Fatal(err)
	}
	if c := salaryCreditByID(t, repo, salary, date); c.Kind != domain.SalaryKindPLR || !c.Month.IsZero() {
		t.Fatalf("PLR = %+v", c)
	}

	if err := repo.DecideSalary(ctx, salary, domain.SalaryKindPart, &month, 30); err != nil {
		t.Fatal(err)
	}
	if c := salaryCreditByID(t, repo, salary, date); c.Kind != domain.SalaryKindPart || c.Day != 30 {
		t.Fatalf("parte = %+v", c)
	}

	if err := repo.UndoSalaryDecision(ctx, salary); err != nil {
		t.Fatal(err)
	}
	if c := salaryCreditByID(t, repo, salary, date); c.Kind != "" {
		t.Fatalf("desfeito = %+v", c)
	}
}
