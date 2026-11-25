package ports

import (
	"context"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// TransferReviewStore guarda a revisão das transferências vindas do banco. Nenhum desses dados vai à IA.
type TransferReviewStore interface {
	// PendingTransfers lista, da mais recente, as transferências externas sem decisão (até limit) e quantas há no total.
	PendingTransfers(ctx context.Context, limit int) ([]domain.TransferReviewItem, int, error)
	// DecidedTransfers lista as decisões, da mais recente.
	DecidedTransfers(ctx context.Context, limit int) ([]domain.TransferReviewItem, error)
	// DecideTransfer grava a decisão e o estado anterior, e aplica o efeito, numa transação. category só vale em
	// PAYMENT. Devolve domain.ErrTransferNotPending se o pagamento não está na lista de pendentes.
	DecideTransfer(ctx context.Context, paymentID uuid.UUID, decision domain.TransferDecision, category string) error
	// AutoDecideTransfer é DecideTransfer marcando a decisão como automática.
	AutoDecideTransfer(ctx context.Context, paymentID uuid.UUID, decision domain.TransferDecision, category string) error
	// AutoTransferCandidates são as pendentes que podem ser decididas sozinhas (todas, menos as que já foram desfeitas).
	AutoTransferCandidates(ctx context.Context) ([]domain.TransferReviewItem, error)
	// TransferMemory é o que já foi decidido por nome (domain.TransferMemoryKey), a decisão mais recente valendo.
	TransferMemory(ctx context.Context) (map[string]domain.RememberedTransfer, error)
	// UndoTransferDecision restaura o status e a categoria de antes e apaga a decisão, numa transação. A transferência
	// desfeita não volta a ser decidida sozinha.
	UndoTransferDecision(ctx context.Context, paymentID uuid.UUID) error
}
