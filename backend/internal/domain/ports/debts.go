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
	// MovementsWith devolve as entradas e saídas de conta corrente desde from cuja descrição traz algum dos nomes de match
	// (domain.MatchesPayee), em ordem de data.
	MovementsWith(ctx context.Context, match string, from time.Time) ([]domain.MoneyMovement, error)
	// BalanceDays devolve o menor e o último saldo de cada dia da conta desde from.
	BalanceDays(ctx context.Context, accountID uuid.UUID, from time.Time) ([]domain.BalanceDay, error)
	// CardBillTotal soma o que já está na fatura do cartão que vence em due (compras, parcelas previstas e estornos).
	CardBillTotal(ctx context.Context, cardID uuid.UUID, due time.Time) (float64, error)
	// BankMovements devolve as despesas (In falso) e as entradas fora o salário (In verdadeiro) de todas as contas correntes
	// com data de from a to (inclusive), sem as transferências entre contas próprias.
	BankMovements(ctx context.Context, from, to time.Time) ([]domain.MoneyMovement, error)
	// CardSpend soma as compras à vista no cartão (estornos abatem) com data de from a to (inclusive).
	CardSpend(ctx context.Context, cardID uuid.UUID, from, to time.Time) (float64, error)
}
