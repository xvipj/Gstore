package models

import (
	"time"

	"gorm.io/gorm"
)

// Cap is a product in the catalog. Prices are stored as whole Colombian pesos.
type Cap struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"not null"`
	Description string `gorm:"type:text;not null"`
	Color       string `gorm:"not null;index"`
	Price       int    `gorm:"not null"`
	Stock       int    `gorm:"not null;default:0"`
	Sold        int    `gorm:"not null;default:0"`
	ImagePath   string `gorm:"not null"`
	Published   bool   `gorm:"not null;default:true;index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}
