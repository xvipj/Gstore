package repository

import (
	"context"

	"gorm.io/gorm"

	"tienda-gorras/internal/models"
)

type CapRepository struct {
	db *gorm.DB
}

func NewCapRepository(db *gorm.DB) *CapRepository {
	return &CapRepository{db: db}
}

func (r *CapRepository) ListPublished(ctx context.Context, color string) ([]models.Cap, error) {
	query := r.db.WithContext(ctx).
		Where("published = ?", true).
		Order("id ASC")

	if color != "" {
		query = query.Where("color = ?", color)
	}

	var caps []models.Cap
	err := query.Find(&caps).Error
	return caps, err
}

func (r *CapRepository) PublishedColors(ctx context.Context) ([]string, error) {
	var colors []string
	err := r.db.WithContext(ctx).
		Model(&models.Cap{}).
		Where("published = ?", true).
		Distinct().
		Order("color ASC").
		Pluck("color", &colors).Error
	return colors, err
}

func (r *CapRepository) GetByID(ctx context.Context, id uint) (*models.Cap, error) {
	var cap models.Cap
	if err := r.db.WithContext(ctx).First(&cap, id).Error; err != nil {
		return nil, err
	}
	return &cap, nil
}

func (r *CapRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.Cap, error) {
	if len(ids) == 0 {
		return []models.Cap{}, nil
	}
	var caps []models.Cap
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&caps).Error
	return caps, err
}

func (r *CapRepository) ListAll(ctx context.Context) ([]models.Cap, error) {
	var caps []models.Cap
	err := r.db.WithContext(ctx).Order("id DESC").Find(&caps).Error
	return caps, err
}

func (r *CapRepository) Create(ctx context.Context, cap *models.Cap) error {
	return r.db.WithContext(ctx).Create(cap).Error
}

func (r *CapRepository) Update(ctx context.Context, cap *models.Cap) error {
	return r.db.WithContext(ctx).Model(&models.Cap{}).
		Where("id = ?", cap.ID).
		Updates(map[string]any{
			"name":        cap.Name,
			"description": cap.Description,
			"color":       cap.Color,
			"price":       cap.Price,
			"stock":       cap.Stock,
			"image_path":  cap.ImagePath,
			"published":   cap.Published,
		}).Error
}

func (r *CapRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.Cap{}, id).Error
}

// DecrementStock updates stock and sold atomically. It returns false when the
// requested quantity is no longer available.
func (r *CapRepository) DecrementStock(ctx context.Context, capID uint, quantity int) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.Cap{}).
		Where("id = ? AND stock >= ?", capID, quantity).
		Updates(map[string]any{
			"stock": gorm.Expr("stock - ?", quantity),
			"sold":  gorm.Expr("sold + ?", quantity),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

type CatalogStats struct {
	PublishedCaps int64
	StockUnits    int64
	SoldUnits     int64
}

func (r *CapRepository) CatalogStats(ctx context.Context) (CatalogStats, error) {
	var stats CatalogStats
	query := r.db.WithContext(ctx).Model(&models.Cap{}).Where("published = ?", true)

	if err := query.Count(&stats.PublishedCaps).Error; err != nil {
		return stats, err
	}
	if err := query.Select("COALESCE(SUM(stock), 0)").Scan(&stats.StockUnits).Error; err != nil {
		return stats, err
	}
	if err := query.Select("COALESCE(SUM(sold), 0)").Scan(&stats.SoldUnits).Error; err != nil {
		return stats, err
	}
	return stats, nil
}

type ColorStats struct {
	Color string
	Stock int
	Sold  int
}

func (r *CapRepository) StatsByColor(ctx context.Context) ([]ColorStats, error) {
	var stats []ColorStats
	err := r.db.WithContext(ctx).
		Model(&models.Cap{}).
		Select("color, COALESCE(SUM(stock), 0) AS stock, COALESCE(SUM(sold), 0) AS sold").
		Where("published = ?", true).
		Group("color").
		Order("color ASC").
		Scan(&stats).Error
	return stats, err
}
