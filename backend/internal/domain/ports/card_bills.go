package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// CardBillReader lê as faturas dos cartões como o app as monta (lançamentos com fatura calculada) ao lado do total que o
// banco informou, para conferir uma com a outra.
type CardBillReader interface {
	// CardBills resume as faturas do cartão, da mais antiga à mais nova.
	CardBills(ctx context.Context, cardID uuid.UUID) ([]domain.CardBillSummary, error)
	// CardBillLines devolve as linhas da fatura do cartão que vence no mês de due.
	CardBillLines(ctx context.Context, cardID uuid.UUID, due time.Time) ([]domain.CardBillLine, error)
}
