package openfinance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
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

type billsProvider struct {
	mockProvider
	bills    []ports.ExternalBill
	billsErr error
}

func (p *billsProvider) FetchItem(ctx context.Context, itemID string, from time.Time) (ports.ItemData, error) {
	data, err := p.mockProvider.FetchItem(ctx, itemID, from)
	data.Bills, data.BillsErr = p.bills, p.billsErr
	return data, err
}

func TestSync_SavesBillsAndRebuildsForecasts(t *testing.T) {
	accID := uuid.New()
	var saved []ports.ExternalBill
	var savedFor uuid.UUID
	var rebuildDays int
	var rebuildDay time.Time
	repo := &mockStore{
		upsertAccountFn: func(context.Context, ports.ExternalAccount) (uuid.UUID, error) { return accID, nil },
		saveBillsFn: func(_ context.Context, id uuid.UUID, b []ports.ExternalBill) error {
			savedFor, saved = id, b
			return nil
		},
		rebuildFn: func(_ context.Context, days int, today time.Time) (int, int, error) {
			rebuildDays, rebuildDay = days, today
			return 5, 1, nil
		},
	}
	prov := &billsProvider{bills: []ports.ExternalBill{
		{ID: "b1", AccountID: "acc", DueDate: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), Total: 8583.09},
		{ID: "x", AccountID: "outra-conta", Total: 1},
	}}
	s := newSync(repo, &prov.mockProvider, "item")
	s.provider = prov
	s.SetClock(domain.FixedClock{T: time.Date(2026, 11, 20, 23, 30, 0, 0, time.UTC)})
	s.SetCardCloseDays(7)

	if _, err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if savedFor != accID || len(saved) != 1 || saved[0].ID != "b1" {
		t.Errorf("deveria gravar só a fatura da conta conhecida, com o ID interno: %v %+v", savedFor, saved)
	}
	if rebuildDays != 7 || !rebuildDay.Equal(time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("rebuild com %d dias em %s", rebuildDays, rebuildDay)
	}
}

func TestSync_BillsErrorDoesNotFailSync(t *testing.T) {
	prov := &billsProvider{billsErr: errors.New("conector sem faturas")}
	s := newSync(&mockStore{}, &prov.mockProvider, "item")
	s.provider = prov
	if _, err := s.Sync(context.Background()); err != nil {
		t.Errorf("faturas indisponíveis não deveriam falhar a sincronização, got %v", err)
	}
}

func TestSync_CardRefundIsNegativeExpenseAndNeverReconciled(t *testing.T) {
	var saved *domain.Purchase
	var pay *domain.Payment
	reconcileCalled := false
	repo := &mockStore{
		reconcileExternalFn: func(context.Context, ports.ExternalTransaction, uuid.UUID) (bool, error) {
			reconcileCalled = true
			return true, nil
		},
		saveExternalFn: func(_ context.Context, p *domain.Purchase, pm *domain.Payment) error {
			saved, pay = p, pm
			return nil
		},
	}
	refund := extTx("t1", domain.KindExpense)
	refund.Refund, refund.Amount, refund.BillID = true, 99.99, "bill-1"
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {refund}}}

	if _, err := newSync(repo, prov, "item").Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reconcileCalled {
		t.Error("estorno não deve casar com lançamento manual")
	}
	if saved == nil || saved.Kind != domain.KindExpense || saved.TotalAmount != -99.99 || pay.Amount != -99.99 {
		t.Fatalf("estorno salvo = %+v / %+v", saved, pay)
	}
	if pay.CardBillID == nil || *pay.CardBillID != "bill-1" {
		t.Errorf("fatura do banco não gravada: %v", pay.CardBillID)
	}
}

func TestSync_ReclassifiesOwnMovesEverySync(t *testing.T) {
	calls := 0
	repo := &mockStore{reclassifyFn: func(context.Context) (int64, int64, int64, error) { calls++; return 2, 1, 1, nil }}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}}}
	if _, err := newSync(repo, prov, "item").Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("deveria reclassificar uma vez por sincronização, got %d", calls)
	}

	repo.reclassifyFn = func(context.Context) (int64, int64, int64, error) { return 0, 0, 0, errors.New("falhou") }
	if _, err := newSync(repo, prov, "item").Sync(context.Background()); err == nil {
		t.Error("falha na reclassificação deveria voltar como erro")
	}
}

func TestSync_AppliesPendingNotes(t *testing.T) {
	calls := 0
	repo := &mockStore{applyNotesFn: func(context.Context) (int, error) { calls++; return 1, nil }}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}}}
	if _, err := newSync(repo, prov, "item").Sync(context.Background()); err != nil || calls != 1 {
		t.Errorf("notas pendentes deveriam ser aplicadas uma vez: calls=%d err=%v", calls, err)
	}
}
