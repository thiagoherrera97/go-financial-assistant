package openfinance

import (
	"context"
	"errors"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestSync_RecomputesCardBillsAfterTransactions(t *testing.T) {
	var order []string
	var gotDays []int
	repo := &mockStore{
		saveExternalFn: func(context.Context, *domain.Purchase, *domain.Payment) error {
			order = append(order, "save")
			return nil
		},
		recomputeBillsFn: func(_ context.Context, days int) (int64, error) {
			order = append(order, "bills")
			gotDays = append(gotDays, days)
			return 3, nil
		},
	}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}}}

	s := newSync(repo, prov, "item")
	if _, err := s.Sync(context.Background()); err != nil {
		t.Fatalf("esperava sem erro, got %v", err)
	}
	if len(order) != 2 || order[1] != "bills" {
		t.Fatalf("a fatura deveria ser calculada uma vez, depois das transações: %v", order)
	}
	if gotDays[0] != domain.DefaultCardCloseDaysBeforeDue {
		t.Errorf("sem configuração deveria usar %d dias, got %d", domain.DefaultCardCloseDaysBeforeDue, gotDays[0])
	}

	s.SetCardCloseDays(8)
	if _, err := s.Sync(context.Background()); err != nil {
		t.Fatalf("esperava sem erro, got %v", err)
	}
	if gotDays[1] != 8 {
		t.Errorf("deveria usar os 8 dias configurados, got %d", gotDays[1])
	}

	s.SetCardCloseDays(0)
	if err := s.RecomputeCardBills(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotDays[2] != domain.DefaultCardCloseDaysBeforeDue {
		t.Errorf("valor inválido deveria voltar ao padrão, got %d", gotDays[2])
	}
}

func TestSync_CardBillErrorIsReported(t *testing.T) {
	boom := errors.New("falhou")
	repo := &mockStore{recomputeBillsFn: func(context.Context, int) (int64, error) { return 0, boom }}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}}}

	res, err := newSync(repo, prov, "item").Sync(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("o erro da fatura deveria voltar no Sync, got %v", err)
	}
	if res.Inserted != 1 {
		t.Errorf("as transações deveriam ser gravadas mesmo assim, got %+v", res)
	}
}
