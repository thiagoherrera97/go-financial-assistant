package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// CategoryStore classifica as despesas que ficaram em "Outros".
type CategoryStore interface {
	// UncategorizedExpenses lista as contas com despesa em Outros que ainda não têm regra, da maior para a menor.
	UncategorizedExpenses(ctx context.Context, limit int) ([]domain.UncategorizedGroup, error)
	// SetCategoryRule grava a regra e reclassifica as despesas da conta que estavam em Outros (ou na categoria
	// da regra anterior). Devolve quantos lançamentos mudaram. A regra também vale para as próximas sincronizações.
	SetCategoryRule(ctx context.Context, key, category string) (int64, error)
	// RefreshSubcategories preenche a subcategoria e o essencial/supérfluo das despesas (sem mexer no que foi corrigido à
	// mão). Idempotente; devolve quantas despesas mudaram.
	RefreshSubcategories(ctx context.Context) (int64, error)
	// SetClassification corrige a despesa do pagamento id (e, com ApplyToAll, as do mesmo estabelecimento). found falso se o
	// pagamento não existe ou não é despesa; others é quantas outras despesas mudaram.
	SetClassification(ctx context.Context, id uuid.UUID, c domain.Classification) (found bool, others int64, err error)
	// SpendBySubcategory soma as despesas do mês por subcategoria e essencial/supérfluo, com a média dos 3 meses anteriores.
	SpendBySubcategory(ctx context.Context, month time.Time) ([]domain.SubcategorySpend, error)
}
