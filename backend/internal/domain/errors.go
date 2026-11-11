package domain

import "errors"

var (
	ErrInvalidAmount        = errors.New("valor deve ser maior que zero")
	ErrInvalidPaymentMethod = errors.New("forma de pagamento inválida")
	ErrPurchaseNotFound     = errors.New("compra não encontrada")
)

// Revisão de transferências.
var (
	ErrTransferNotPending      = errors.New("transferência não encontrada ou já decidida")
	ErrTransferNoDecision      = errors.New("transferência sem decisão para desfazer")
	ErrTransferPaymentOnIncome = errors.New("\"Pagamento\" vale só para saídas; use \"Para uma pessoa\" ou \"Entre contas minhas\"")
)
