package repository

import (
	"context"

	"gorm.io/gorm"

	"tienda-gorras/internal/models"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// CreateWithItems persists the order and its snapshotted items in one transaction.
func (r *OrderRepository) CreateWithItems(ctx context.Context, order *models.Order, items []models.OrderItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = order.ID
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		order.Items = items
		return nil
	})
}

func (r *OrderRepository) GetWithItems(ctx context.Context, id uint) (*models.Order, error) {
	var order models.Order
	if err := r.db.WithContext(ctx).Preload("Items").First(&order, id).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) ListWithItems(ctx context.Context) ([]models.Order, error) {
	var orders []models.Order
	err := r.db.WithContext(ctx).
		Preload("Items").
		Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

func (r *OrderRepository) SetPreferenceID(ctx context.Context, orderID uint, preferenceID string) error {
	return r.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("id = ?", orderID).
		Update("mp_preference_id", preferenceID).Error
}

// SetStatusIfPending prevents an old notification from overwriting a paid order.
func (r *OrderRepository) SetStatusIfPending(ctx context.Context, orderID uint, status, paymentID string) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("id = ? AND status = ?", orderID, models.OrderPending).
		Updates(map[string]any{
			"status":        status,
			"mp_payment_id": paymentID,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// MarkPaid transitions an order to paid exactly once. Callers must run it in a
// transaction together with the stock updates.
func (r *OrderRepository) MarkPaid(ctx context.Context, orderID uint, paymentID string) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("id = ? AND status <> ?", orderID, models.OrderPaid).
		Updates(map[string]any{
			"status":        models.OrderPaid,
			"mp_payment_id": paymentID,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

type RevenueStats struct {
	PaidOrders int64
	Revenue    int64
}

func (r *OrderRepository) RevenueStats(ctx context.Context) (RevenueStats, error) {
	var stats RevenueStats
	query := r.db.WithContext(ctx).Model(&models.Order{}).Where("status = ?", models.OrderPaid)

	if err := query.Count(&stats.PaidOrders).Error; err != nil {
		return stats, err
	}
	if err := query.Select("COALESCE(SUM(total), 0)").Scan(&stats.Revenue).Error; err != nil {
		return stats, err
	}
	return stats, nil
}
