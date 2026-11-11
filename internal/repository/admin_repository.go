package repository

import (
	"context"

	"gorm.io/gorm"

	"tienda-gorras/internal/models"
)

type AdminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) FindByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	var admin models.AdminUser
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}
