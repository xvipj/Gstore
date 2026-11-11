package models

import "time"

const (
	OrderPending   = "pending"
	OrderPaid      = "paid"
	OrderFailed    = "failed"
	OrderCancelled = "cancelled"
)

// Order is the internal record of a purchase. Its total is always calculated
// on the server from the current database prices.
type Order struct {
	ID             uint   `gorm:"primaryKey"`
	Total          int    `gorm:"not null"`
	Status         string `gorm:"not null;index"`
	MPPaymentID    string `gorm:"type:text"`
	MPPreferenceID string `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Items          []OrderItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
}

// OrderItem keeps a snapshot of the product name and price at purchase time.
type OrderItem struct {
	ID        uint `gorm:"primaryKey"`
	OrderID   uint `gorm:"not null;index"`
	CapID     uint `gorm:"not null;index"`
	Name      string
	UnitPrice int
	Quantity  int
}
