package services

import (
	"context"
	"fmt"

	"tienda-gorras/internal/models"
	"tienda-gorras/internal/payments"
	"tienda-gorras/internal/repository"
)

type PaymentGateway interface {
	IsConfigured() bool
	CreatePreference(ctx context.Context, input payments.PreferenceInput) (payments.PreferenceResult, error)
	GetPayment(ctx context.Context, paymentID int) (payments.PaymentInfo, error)
}

type CheckoutURLs struct {
	Success string
	Pending string
	Failure string
	Webhook string
}

type CheckoutService struct {
	cart    *CartService
	orders  *repository.OrderRepository
	gateway PaymentGateway
	urls    CheckoutURLs
}

func NewCheckoutService(
	cart *CartService,
	orders *repository.OrderRepository,
	gateway PaymentGateway,
	urls CheckoutURLs,
) *CheckoutService {
	return &CheckoutService{cart: cart, orders: orders, gateway: gateway, urls: urls}
}

type CheckoutResult struct {
	OrderID   uint   `json:"order_id"`
	InitPoint string `json:"init_point"`
}

// CreateCheckout recalculates the cart from SQLite, stores an order and creates
// the Mercado Pago Checkout Pro preference.
func (s *CheckoutService) CreateCheckout(ctx context.Context, items []CartItemInput) (CheckoutResult, error) {
	if s.gateway == nil || !s.gateway.IsConfigured() {
		return CheckoutResult{}, ErrPaymentsNotConfigured
	}

	lines, err := s.cart.GetLines(ctx, items)
	if err != nil {
		return CheckoutResult{}, err
	}

	order := &models.Order{Status: models.OrderPending}
	orderItems := make([]models.OrderItem, 0, len(lines))
	for _, line := range lines {
		order.Total += line.LineTotal
		orderItems = append(orderItems, models.OrderItem{
			CapID:     line.Cap.ID,
			Name:      line.Cap.Name,
			UnitPrice: line.Cap.Price,
			Quantity:  line.Quantity,
		})
	}

	if err := s.orders.CreateWithItems(ctx, order, orderItems); err != nil {
		return CheckoutResult{}, fmt.Errorf("create order: %w", err)
	}

	preferenceItems := make([]payments.PreferenceItem, 0, len(lines))
	for _, line := range lines {
		preferenceItems = append(preferenceItems, payments.PreferenceItem{
			ID:        line.Cap.ID,
			Title:     fmt.Sprintf("%s (%s)", line.Cap.Name, line.Cap.Color),
			Quantity:  line.Quantity,
			UnitPrice: line.Cap.Price,
		})
	}

	result, err := s.gateway.CreatePreference(ctx, payments.PreferenceInput{
		Items:             preferenceItems,
		ExternalReference: fmt.Sprintf("%d", order.ID),
		NotificationURL:   s.urls.Webhook,
		BackURLs: payments.BackURLs{
			Success: s.urls.Success,
			Pending: s.urls.Pending,
			Failure: s.urls.Failure,
		},
	})
	if err != nil {
		_, _ = s.orders.SetStatusIfPending(ctx, order.ID, models.OrderFailed, "")
		return CheckoutResult{}, fmt.Errorf("create Mercado Pago preference: %w", err)
	}

	if err := s.orders.SetPreferenceID(ctx, order.ID, result.ID); err != nil {
		return CheckoutResult{}, fmt.Errorf("save Mercado Pago preference id: %w", err)
	}

	return CheckoutResult{OrderID: order.ID, InitPoint: result.InitPoint}, nil
}
