package services

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyCart                = errors.New("cart is empty")
	ErrInvalidQuantity          = errors.New("invalid quantity")
	ErrCapUnavailable           = errors.New("cap is not available")
	ErrPaymentsNotConfigured    = errors.New("Mercado Pago is not configured")
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrCapNotFound              = errors.New("cap not found")
	ErrOrderNotFound            = errors.New("order not found")
	ErrPaymentReferenceMismatch = errors.New("payment external reference does not match the order")
	ErrPaymentAmountMismatch    = errors.New("payment amount does not match the order")
	ErrPaymentCurrencyMismatch  = errors.New("payment currency does not match COP")
	ErrPaymentStockConflict     = errors.New("not enough stock to fulfill the paid order")
)

// StockError gives the UI enough detail to explain a shortage.
type StockError struct {
	CapName   string
	Available int
}

func (e *StockError) Error() string {
	return fmt.Sprintf("not enough stock for %s (available: %d)", e.CapName, e.Available)
}

// ValidationError is used by admin forms to highlight invalid fields.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return "validation failed"
}
