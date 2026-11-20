package openfinance

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

// mockStore implementa ports.ExternalStore; cada função, quando definida, substitui o comportamento padrão (sucesso).
type mockStore struct {
	saveInvestmentsFn   func(ctx context.Context, itemID string, positions []ports.ExternalInvestment, day time.Time) error
	refreshExternalFn   func(ctx context.Context, tx ports.ExternalTransaction, accountID uuid.UUID) (bool, error)
	upsertAccountFn     func(ctx context.Context, a ports.ExternalAccount) (uuid.UUID, error)
	upsertInstitutionFn func(ctx context.Context, itemID string, inst ports.ExternalInstitution) (uuid.UUID, bool, error)
	saveLogoFn          func(ctx context.Context, id uuid.UUID, data []byte, mime string) error
	reconcileExternalFn func(ctx context.Context, tx ports.ExternalTransaction, accountID uuid.UUID) (bool, error)
	recategorizeFn      func(ctx context.Context, mapping map[string]domain.Category) (int64, error)
	saveExternalFn      func(ctx context.Context, purchase *domain.Purchase, payment *domain.Payment) error
	recomputeBillsFn    func(ctx context.Context, closeDaysBefore int) (int64, error)
	saveBillsFn         func(ctx context.Context, accountID uuid.UUID, bills []ports.ExternalBill) error
	rebuildFn           func(ctx context.Context, closeDaysBefore int, today time.Time) (int, int, error)
	reclassifyFn        func(ctx context.Context) (int64, int64, int64, error)
	applyNotesFn        func(ctx context.Context) (int, error)
}

func (m *mockStore) ApplyPendingNotes(ctx context.Context) (int, error) {
	if m.applyNotesFn != nil {
		return m.applyNotesFn(ctx)
	}
	return 0, nil
}

func (m *mockStore) ReclassifyOwnMoves(ctx context.Context) (int64, int64, int64, error) {
	if m.reclassifyFn != nil {
		return m.reclassifyFn(ctx)
	}
	return 0, 0, 0, nil
}

func (m *mockStore) SaveInvestments(ctx context.Context, itemID string, positions []ports.ExternalInvestment, day time.Time) error {
	if m.saveInvestmentsFn != nil {
		return m.saveInvestmentsFn(ctx, itemID, positions, day)
	}
	return nil
}

func (m *mockStore) UpsertAccount(ctx context.Context, a ports.ExternalAccount) (uuid.UUID, error) {
	if m.upsertAccountFn != nil {
		return m.upsertAccountFn(ctx, a)
	}
	return uuid.New(), nil
}

func (m *mockStore) UpsertInstitution(ctx context.Context, itemID string, inst ports.ExternalInstitution) (uuid.UUID, bool, error) {
	if m.upsertInstitutionFn != nil {
		return m.upsertInstitutionFn(ctx, itemID, inst)
	}
	return uuid.New(), false, nil
}

func (m *mockStore) SaveInstitutionLogo(ctx context.Context, id uuid.UUID, data []byte, mime string) error {
	if m.saveLogoFn != nil {
		return m.saveLogoFn(ctx, id, data, mime)
	}
	return nil
}

func (m *mockStore) RefreshExternal(ctx context.Context, tx ports.ExternalTransaction, accountID uuid.UUID) (bool, error) {
	if m.refreshExternalFn != nil {
		return m.refreshExternalFn(ctx, tx, accountID)
	}
	return false, nil
}

func (m *mockStore) ReconcileExternal(ctx context.Context, tx ports.ExternalTransaction, accountID uuid.UUID) (bool, error) {
	if m.reconcileExternalFn != nil {
		return m.reconcileExternalFn(ctx, tx, accountID)
	}
	return false, nil
}

func (m *mockStore) SaveExternal(ctx context.Context, purchase *domain.Purchase, payment *domain.Payment) error {
	if m.saveExternalFn != nil {
		return m.saveExternalFn(ctx, purchase, payment)
	}
	return nil
}

func (m *mockStore) RecomputeCardBills(ctx context.Context, closeDaysBefore int) (int64, error) {
	if m.recomputeBillsFn != nil {
		return m.recomputeBillsFn(ctx, closeDaysBefore)
	}
	return 0, nil
}

func (m *mockStore) SaveCardBills(ctx context.Context, accountID uuid.UUID, bills []ports.ExternalBill) error {
	if m.saveBillsFn != nil {
		return m.saveBillsFn(ctx, accountID, bills)
	}
	return nil
}

func (m *mockStore) RebuildCardForecasts(ctx context.Context, closeDaysBefore int, today time.Time) (int, int, error) {
	if m.rebuildFn != nil {
		return m.rebuildFn(ctx, closeDaysBefore, today)
	}
	return 0, 0, nil
}

func (m *mockStore) RecategorizeExternalOther(ctx context.Context, mapping map[string]domain.Category) (int64, error) {
	if m.recategorizeFn != nil {
		return m.recategorizeFn(ctx, mapping)
	}
	return 0, nil
}
