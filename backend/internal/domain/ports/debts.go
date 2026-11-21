package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// DebtStore guarda as dívidas pessoais e lê o que o cálculo delas precisa. Datas são dias civis (fuso de São Paulo).
type DebtStore interface {
	Debts(ctx context.Context) ([]domain.Debt, error)
	CreateDebt(ctx context.Context, d domain.Debt) error
	// UpdateDebt regrava a dívida; false se ela não existe.
	UpdateDebt(ctx context.Context, d domain.Debt) (bool, error)
	// DeleteDebt apaga a dívida; false se ela não existe.
	DeleteDebt(ctx context.Context, id uuid.UUID) (bool, error)
	// PaymentsMatching devolve as saídas de conta corrente desde from cuja descrição contém match (sem diferenciar
	// maiúsculas e acentos).
	PaymentsMatching(ctx context.Context, match string, from time.Time) ([]domain.MoneyMovement, error)
	// BalanceDays devolve o menor e o último saldo de cada dia da conta desde from.
	BalanceDays(ctx context.Context, accountID uuid.UUID, from time.Time) ([]domain.BalanceDay, error)
	// CardBillTotal soma o que já está na fatura do cartão que vence em due (compras, parcelas previstas e estornos).
	CardBillTotal(ctx context.Context, cardID uuid.UUID, due time.Time) (float64, error)
	// AccountOutflows devolve as despesas da conta corrente com data de from a to (inclusive).
	AccountOutflows(ctx context.Context, accountID uuid.UUID, from, to time.Time) ([]domain.MoneyMovement, error)
}
