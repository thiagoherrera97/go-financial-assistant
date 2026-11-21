package ports

import (
	"context"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// VendorStore guarda o plano de fornecedores (ex.: casamento), os fornecedores e os pagamentos a eles.
type VendorStore interface {
	VendorPlan(ctx context.Context) (domain.VendorPlan, error)
	SaveVendorPlan(ctx context.Context, p domain.VendorPlan) error
	// Vendors devolve os fornecedores com os pagamentos de cada um.
	Vendors(ctx context.Context) ([]domain.Vendor, error)
	// CreateVendor grava o fornecedor e, se vier, um pagamento inicial (o que já foi pago antes de usar o app).
	CreateVendor(ctx context.Context, v domain.Vendor, initial *domain.VendorPayment) error
	// UpdateVendor troca nome, valor contratado e vencimento; false se o fornecedor não existe.
	UpdateVendor(ctx context.Context, v domain.Vendor) (bool, error)
	DeleteVendor(ctx context.Context, id uuid.UUID) (bool, error)
	// AddVendorPayment grava um pagamento. Com TransactionID, valor e data vêm da transação (o que vier em p é
	// ignorado) e os erros são domain.ErrVendorTxNotFound e domain.ErrVendorTxLinked. Sem o fornecedor: ErrVendorNotFound.
	AddVendorPayment(ctx context.Context, p domain.VendorPayment) error
	DeleteVendorPayment(ctx context.Context, vendorID, paymentID uuid.UUID) (bool, error)
}
