package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func TestVendorsRepo_Integration(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()

	plan := domain.VendorPlan{Name: "Casamento", ContributionDay: 10, SavingsBox: "Casamento"}
	if err := repo.SaveVendorPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.VendorPlan(ctx); err != nil || got != plan {
		t.Fatalf("plano: %+v %v", got, err)
	}

	due := time.Date(2027, time.August, 23, 0, 0, 0, 0, time.UTC)
	v := domain.Vendor{ID: uuid.New(), Name: "Churrasgrill", Contracted: 18489.98, DueDate: &due}
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM vendors WHERE id = $1`, v.ID) })
	initial := &domain.VendorPayment{ID: uuid.New(), Amount: 4000, PaidOn: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)}
	if err := repo.CreateVendor(ctx, v, initial); err != nil {
		t.Fatal(err)
	}

	// Liga um gasto do extrato: valor e data vêm dele.
	p := manualExpense(t, repo, pg, 1500, domain.PurchaseTypeSingle)
	var txID uuid.UUID
	if err := pg.Pool.QueryRow(ctx, `SELECT id FROM payments WHERE purchase_id = $1`, p.ID).Scan(&txID); err != nil {
		t.Fatal(err)
	}
	if err := repo.AddVendorPayment(ctx, domain.VendorPayment{ID: uuid.New(), VendorID: v.ID, TransactionID: &txID}); err != nil {
		t.Fatalf("ligar: %v", err)
	}
	if err := repo.AddVendorPayment(ctx, domain.VendorPayment{ID: uuid.New(), VendorID: v.ID, TransactionID: &txID}); !errors.Is(err, domain.ErrVendorTxLinked) {
		t.Errorf("ligar de novo = %v", err)
	}
	other := uuid.New()
	if err := repo.AddVendorPayment(ctx, domain.VendorPayment{ID: uuid.New(), VendorID: v.ID, TransactionID: &other}); !errors.Is(err, domain.ErrVendorTxNotFound) {
		t.Errorf("transação inexistente = %v", err)
	}
	if err := repo.AddVendorPayment(ctx, domain.VendorPayment{ID: uuid.New(), VendorID: uuid.New(), Amount: 1, PaidOn: due}); !errors.Is(err, domain.ErrVendorNotFound) {
		t.Errorf("fornecedor inexistente = %v", err)
	}

	vendors, err := repo.Vendors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got *domain.Vendor
	for i := range vendors {
		if vendors[i].ID == v.ID {
			got = &vendors[i]
		}
	}
	if got == nil || len(got.Payments) != 2 || got.Paid() != 5500 || got.DueDate == nil || !got.DueDate.Equal(due) {
		t.Fatalf("lido: %+v", got)
	}
	linked := got.Payments[1]
	if got.Payments[0].PaidOn.After(linked.PaidOn) {
		linked = got.Payments[0]
	}
	if linked.TransactionID == nil || *linked.TransactionID != txID || linked.Amount != 1500 || linked.Description != "teste-open-finance" {
		t.Errorf("pagamento ligado: %+v", linked)
	}

	// Apagar a transação some com o vínculo.
	if _, err := pg.Pool.Exec(ctx, `DELETE FROM purchases WHERE id = $1`, p.ID); err != nil {
		t.Fatal(err)
	}
	vendors, _ = repo.Vendors(ctx)
	for _, x := range vendors {
		if x.ID == v.ID && len(x.Payments) != 1 {
			t.Errorf("vínculo deveria sumir com a transação: %+v", x.Payments)
		}
	}

	v.Name, v.DueDate = "Churras", nil
	if ok, err := repo.UpdateVendor(ctx, v); !ok || err != nil {
		t.Errorf("alterar: %v %v", ok, err)
	}
	if ok, err := repo.DeleteVendorPayment(ctx, uuid.New(), initial.ID); ok || err != nil {
		t.Errorf("apagar pagamento de outro fornecedor: %v %v", ok, err)
	}
	if ok, err := repo.DeleteVendorPayment(ctx, v.ID, initial.ID); !ok || err != nil {
		t.Errorf("apagar pagamento: %v %v", ok, err)
	}
	if ok, err := repo.DeleteVendor(ctx, v.ID); !ok || err != nil {
		t.Errorf("apagar: %v %v", ok, err)
	}
}
