package services

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tienda-gorras/internal/models"
	"tienda-gorras/internal/payments"
	"tienda-gorras/internal/repository"
)

type fakeGateway struct {
	configured bool
	payment    payments.PaymentInfo
	paymentErr error
	preference payments.PreferenceResult
	prefErr    error
	lastPref   payments.PreferenceInput
}

func (f *fakeGateway) IsConfigured() bool {
	return f.configured
}

func (f *fakeGateway) CreatePreference(_ context.Context, input payments.PreferenceInput) (payments.PreferenceResult, error) {
	f.lastPref = input
	return f.preference, f.prefErr
}

func (f *fakeGateway) GetPayment(_ context.Context, _ int) (payments.PaymentInfo, error) {
	return f.payment, f.paymentErr
}

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Migrator().DropTable(&models.OrderItem{}, &models.Order{}, &models.Cap{}, &models.AdminUser{}); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if err := db.AutoMigrate(&models.Cap{}, &models.Order{}, &models.OrderItem{}, &models.AdminUser{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createOrder(t *testing.T, db *gorm.DB, stock, quantity int) (*models.Cap, *models.Order) {
	t.Helper()
	cap := &models.Cap{
		Name:        "Test Cap",
		Description: "Test",
		Color:       "Negro",
		Price:       85000,
		Stock:       stock,
		ImagePath:   "/static/uploads/test.jpg",
		Published:   true,
	}
	if err := db.Create(cap).Error; err != nil {
		t.Fatalf("create cap: %v", err)
	}

	order := &models.Order{Total: cap.Price * quantity, Status: models.OrderPending}
	items := []models.OrderItem{{
		CapID:     cap.ID,
		Name:      cap.Name,
		UnitPrice: cap.Price,
		Quantity:  quantity,
	}}
	if err := repository.NewOrderRepository(db).CreateWithItems(context.Background(), order, items); err != nil {
		t.Fatalf("create order: %v", err)
	}
	return cap, order
}

func TestApprovedPaymentUpdatesStockOnce(t *testing.T) {
	db := testDB(t)
	cap, order := createOrder(t, db, 5, 2)

	gateway := &fakeGateway{
		configured: true,
		payment: payments.PaymentInfo{
			ID:                12345,
			Status:            "approved",
			ExternalReference: strconv.FormatUint(uint64(order.ID), 10),
			CurrencyID:        "COP",
			TransactionAmount: float64(order.Total),
		},
	}
	service := NewPaymentService(db, gateway)

	if err := service.ProcessPayment(context.Background(), 12345); err != nil {
		t.Fatalf("process payment: %v", err)
	}
	if err := service.ProcessPayment(context.Background(), 12345); err != nil {
		t.Fatalf("idempotent process payment: %v", err)
	}

	var updatedCap models.Cap
	if err := db.First(&updatedCap, cap.ID).Error; err != nil {
		t.Fatalf("load cap: %v", err)
	}
	if updatedCap.Stock != 3 || updatedCap.Sold != 2 {
		t.Fatalf("expected stock=3 sold=2, got stock=%d sold=%d", updatedCap.Stock, updatedCap.Sold)
	}

	var updatedOrder models.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != models.OrderPaid {
		t.Fatalf("expected paid order, got %s", updatedOrder.Status)
	}
	if updatedOrder.MPPaymentID != "12345" {
		t.Fatalf("expected payment id 12345, got %s", updatedOrder.MPPaymentID)
	}

	adminService := NewAdminService(
		repository.NewAdminRepository(db),
		repository.NewCapRepository(db),
		repository.NewOrderRepository(db),
		t.TempDir(),
	)
	summary, err := adminService.Summary(context.Background())
	if err != nil {
		t.Fatalf("load admin summary: %v", err)
	}
	if summary.StockUnits != 3 || summary.SoldUnits != 2 || summary.Revenue != 170000 || summary.PaidOrders != 1 {
		t.Fatalf("unexpected admin summary: %+v", summary)
	}
	if len(summary.Colors) != 1 || summary.Colors[0].Color != "Negro" || summary.Colors[0].Stock != 3 || summary.Colors[0].Sold != 2 {
		t.Fatalf("unexpected color summary: %+v", summary.Colors)
	}
}

func TestRejectedAndCancelledPaymentsDoNotTouchStock(t *testing.T) {
	for _, test := range []struct {
		paymentStatus string
		orderStatus   string
	}{
		{paymentStatus: "rejected", orderStatus: models.OrderFailed},
		{paymentStatus: "cancelled", orderStatus: models.OrderCancelled},
	} {
		t.Run(test.paymentStatus, func(t *testing.T) {
			db := testDB(t)
			cap, order := createOrder(t, db, 5, 2)
			gateway := &fakeGateway{
				configured: true,
				payment: payments.PaymentInfo{
					ID:                99,
					Status:            test.paymentStatus,
					ExternalReference: strconv.FormatUint(uint64(order.ID), 10),
					CurrencyID:        "COP",
					TransactionAmount: float64(order.Total),
				},
			}

			if err := NewPaymentService(db, gateway).ProcessPayment(context.Background(), 99); err != nil {
				t.Fatalf("process payment: %v", err)
			}

			var updatedCap models.Cap
			if err := db.First(&updatedCap, cap.ID).Error; err != nil {
				t.Fatalf("load cap: %v", err)
			}
			if updatedCap.Stock != 5 || updatedCap.Sold != 0 {
				t.Fatalf("stock must not change, got stock=%d sold=%d", updatedCap.Stock, updatedCap.Sold)
			}

			var updatedOrder models.Order
			if err := db.First(&updatedOrder, order.ID).Error; err != nil {
				t.Fatalf("load order: %v", err)
			}
			if updatedOrder.Status != test.orderStatus {
				t.Fatalf("expected status %s, got %s", test.orderStatus, updatedOrder.Status)
			}
		})
	}
}

func TestPaymentValidationRejectsMismatches(t *testing.T) {
	t.Run("external reference", func(t *testing.T) {
		db := testDB(t)
		_, _ = createOrder(t, db, 5, 1)
		gateway := &fakeGateway{
			configured: true,
			payment: payments.PaymentInfo{
				ID:                1,
				Status:            "approved",
				ExternalReference: "9999",
				CurrencyID:        "COP",
				TransactionAmount: 85000,
			},
		}
		err := NewPaymentService(db, gateway).ProcessPayment(context.Background(), 1)
		if !errors.Is(err, ErrOrderNotFound) {
			t.Fatalf("expected missing order, got %v", err)
		}
	})

	t.Run("currency", func(t *testing.T) {
		db := testDB(t)
		_, order := createOrder(t, db, 5, 1)
		gateway := &fakeGateway{
			configured: true,
			payment: payments.PaymentInfo{
				ID:                1,
				Status:            "approved",
				ExternalReference: strconv.FormatUint(uint64(order.ID), 10),
				CurrencyID:        "USD",
				TransactionAmount: float64(order.Total),
			},
		}
		err := NewPaymentService(db, gateway).ProcessPayment(context.Background(), 1)
		if !errors.Is(err, ErrPaymentCurrencyMismatch) {
			t.Fatalf("expected currency mismatch, got %v", err)
		}
	})

	t.Run("amount", func(t *testing.T) {
		db := testDB(t)
		_, order := createOrder(t, db, 5, 1)
		gateway := &fakeGateway{
			configured: true,
			payment: payments.PaymentInfo{
				ID:                1,
				Status:            "approved",
				ExternalReference: strconv.FormatUint(uint64(order.ID), 10),
				CurrencyID:        "COP",
				TransactionAmount: 1,
			},
		}
		err := NewPaymentService(db, gateway).ProcessPayment(context.Background(), 1)
		if !errors.Is(err, ErrPaymentAmountMismatch) {
			t.Fatalf("expected amount mismatch, got %v", err)
		}
	})
}

func TestApprovedPaymentRollsBackWhenStockIsGone(t *testing.T) {
	db := testDB(t)
	cap, order := createOrder(t, db, 1, 2)
	gateway := &fakeGateway{
		configured: true,
		payment: payments.PaymentInfo{
			ID:                77,
			Status:            "approved",
			ExternalReference: strconv.FormatUint(uint64(order.ID), 10),
			CurrencyID:        "COP",
			TransactionAmount: float64(order.Total),
		},
	}

	err := NewPaymentService(db, gateway).ProcessPayment(context.Background(), 77)
	if !errors.Is(err, ErrPaymentStockConflict) {
		t.Fatalf("expected stock conflict, got %v", err)
	}

	var updatedCap models.Cap
	_ = db.First(&updatedCap, cap.ID).Error
	if updatedCap.Stock != 1 || updatedCap.Sold != 0 {
		t.Fatalf("stock must be rolled back, got stock=%d sold=%d", updatedCap.Stock, updatedCap.Sold)
	}
	var updatedOrder models.Order
	_ = db.First(&updatedOrder, order.ID).Error
	if updatedOrder.Status != models.OrderPending {
		t.Fatalf("order status must remain pending, got %s", updatedOrder.Status)
	}
}

func TestCheckoutRecalculatesPricesAndCreatesPreference(t *testing.T) {
	db := testDB(t)
	cap := &models.Cap{
		Name:        "Checkout Cap",
		Description: "Test",
		Color:       "Azul",
		Price:       105000,
		Stock:       4,
		ImagePath:   "/static/uploads/test.jpg",
		Published:   true,
	}
	if err := db.Create(cap).Error; err != nil {
		t.Fatalf("create cap: %v", err)
	}

	gateway := &fakeGateway{
		configured: true,
		preference: payments.PreferenceResult{ID: "pref-123", InitPoint: "https://example.com/checkout"},
	}
	service := NewCheckoutService(
		NewCartService(repository.NewCapRepository(db)),
		repository.NewOrderRepository(db),
		gateway,
		CheckoutURLs{Success: "s", Pending: "p", Failure: "f", Webhook: "w"},
	)

	result, err := service.CreateCheckout(context.Background(), []CartItemInput{
		{CapID: cap.ID, Quantity: 2},
		{CapID: cap.ID, Quantity: 1},
	})
	if err != nil {
		t.Fatalf("create checkout: %v", err)
	}
	if result.InitPoint != "https://example.com/checkout" {
		t.Fatalf("unexpected init point: %s", result.InitPoint)
	}

	var order models.Order
	if err := db.Preload("Items").First(&order, result.OrderID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.Total != 315000 {
		t.Fatalf("expected recalculated total 315000, got %d", order.Total)
	}
	if len(order.Items) != 1 || order.Items[0].Quantity != 3 {
		t.Fatalf("duplicate cart lines must merge: %+v", order.Items)
	}
	if gateway.lastPref.ExternalReference != strconv.FormatUint(uint64(order.ID), 10) {
		t.Fatalf("external reference must be the order id")
	}
	if len(gateway.lastPref.Items) != 1 || gateway.lastPref.Items[0].UnitPrice != 105000 {
		t.Fatalf("preference must use database prices")
	}
}

func TestCheckoutFailsClearlyWithoutConfiguration(t *testing.T) {
	db := testDB(t)
	gateway := &fakeGateway{configured: false}
	service := NewCheckoutService(
		NewCartService(repository.NewCapRepository(db)),
		repository.NewOrderRepository(db),
		gateway,
		CheckoutURLs{},
	)
	_, err := service.CreateCheckout(context.Background(), []CartItemInput{{CapID: 1, Quantity: 1}})
	if !errors.Is(err, ErrPaymentsNotConfigured) {
		t.Fatalf("expected configuration error, got %v", err)
	}
}
