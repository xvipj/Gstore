package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	"gorm.io/gorm"

	"tienda-gorras/internal/models"
	"tienda-gorras/internal/payments"
	"tienda-gorras/internal/repository"
)

type PaymentService struct {
	db      *gorm.DB
	gateway PaymentGateway
}

func NewPaymentService(db *gorm.DB, gateway PaymentGateway) *PaymentService {
	return &PaymentService{db: db, gateway: gateway}
}

// ProcessPayment fetches the payment from Mercado Pago (never trusting the
// webhook body) and applies the state transition idempotently.
func (s *PaymentService) ProcessPayment(ctx context.Context, paymentID int) error {
	if s.gateway == nil || !s.gateway.IsConfigured() {
		return ErrPaymentsNotConfigured
	}

	payment, err := s.gateway.GetPayment(ctx, paymentID)
	if err != nil {
		return err
	}
	if payment.ExternalReference == "" {
		return errors.New("payment has no external reference")
	}

	orderID64, err := strconv.ParseUint(payment.ExternalReference, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid external reference: %w", err)
	}
	orderID := uint(orderID64)

	orders := repository.NewOrderRepository(s.db)
	order, err := orders.GetWithItems(ctx, orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOrderNotFound
		}
		return fmt.Errorf("load order: %w", err)
	}
	if payment.ExternalReference != strconv.FormatUint(uint64(order.ID), 10) {
		return ErrPaymentReferenceMismatch
	}

	switch payment.Status {
	case "approved":
		if err := validateApprovedPayment(payment, order); err != nil {
			return err
		}
		return s.markOrderPaid(ctx, order, payment.ID)
	case "rejected":
		_, err := orders.SetStatusIfPending(ctx, order.ID, models.OrderFailed, strconv.Itoa(payment.ID))
		return err
	case "cancelled":
		_, err := orders.SetStatusIfPending(ctx, order.ID, models.OrderCancelled, strconv.Itoa(payment.ID))
		return err
	default:
		// pending, in_process and other intermediate states do not change stock.
		return nil
	}
}

func validateApprovedPayment(payment payments.PaymentInfo, order *models.Order) error {
	if payment.CurrencyID != "COP" {
		return ErrPaymentCurrencyMismatch
	}
	if math.Abs(payment.TransactionAmount-float64(order.Total)) > 0.01 {
		return ErrPaymentAmountMismatch
	}
	return nil
}

func (s *PaymentService) markOrderPaid(ctx context.Context, order *models.Order, paymentID int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		orders := repository.NewOrderRepository(tx)
		caps := repository.NewCapRepository(tx)

		updated, err := orders.MarkPaid(ctx, order.ID, strconv.Itoa(paymentID))
		if err != nil {
			return fmt.Errorf("mark order as paid: %w", err)
		}
		if !updated {
			// Another webhook already processed this order.
			return nil
		}

		for _, item := range order.Items {
			ok, err := caps.DecrementStock(ctx, item.CapID, item.Quantity)
			if err != nil {
				return fmt.Errorf("decrement stock for cap %d: %w", item.CapID, err)
			}
			if !ok {
				return fmt.Errorf("%w: cap %d", ErrPaymentStockConflict, item.CapID)
			}
		}
		return nil
	})
}
