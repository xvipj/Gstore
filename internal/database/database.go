package database

import (
	"fmt"
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tienda-gorras/internal/config"
	"tienda-gorras/internal/models"
)

// Open creates the SQLite connection, applies migrations and seeds demo data.
func Open(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// SQLite handles a small portfolio workload well with a single writer.
	// WAL keeps reads available while the webhook transaction is running.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if err := db.Exec(pragma).Error; err != nil {
			return nil, fmt.Errorf("apply %s: %w", pragma, err)
		}
	}

	if err := db.AutoMigrate(
		&models.Cap{},
		&models.Order{},
		&models.OrderItem{},
		&models.AdminUser{},
	); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	if err := seedCaps(db); err != nil {
		return nil, err
	}
	if err := seedAdmin(db, cfg); err != nil {
		return nil, err
	}

	return db, nil
}

func seedCaps(db *gorm.DB) error {
	var count int64
	// Include soft-deleted rows so clearing the catalog never resurrects the seed.
	if err := db.Unscoped().Model(&models.Cap{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count caps: %w", err)
	}
	if count > 0 {
		return nil
	}

	caps := []models.Cap{
		{
			Name:        "Gorra Clásica Negra",
			Description: "Gorra de seis paneles en algodón negro, con visera curva y ajuste trasero. Un básico sobrio que combina con cualquier outfit.",
			Color:       "Negro",
			Price:       85000,
			Stock:       12,
			ImagePath:   "/static/uploads/seed-negro.jpg",
			Published:   true,
		},
		{
			Name:        "Gorra Blanca Minimal",
			Description: "Silueta limpia en algodón blanco de tejido firme. Costuras tonales y visera plana para un look directo y sin ruido.",
			Color:       "Blanco",
			Price:       79000,
			Stock:       8,
			ImagePath:   "/static/uploads/seed-blanco.jpg",
			Published:   true,
		},
		{
			Name:        "Gorra Gris Urbana",
			Description: "Gorra gris de perfil medio con paneles estructurados y cierre ajustable. Pensada para el uso diario en la ciudad.",
			Color:       "Gris",
			Price:       92000,
			Stock:       10,
			ImagePath:   "/static/uploads/seed-gris.jpg",
			Published:   true,
		},
		{
			Name:        "Gorra Azul Denim",
			Description: "Acabado tipo denim en azul profundo, visera curva y costuras reforzadas. Una pieza versátil con textura marcada.",
			Color:       "Azul",
			Price:       105000,
			Stock:       6,
			ImagePath:   "/static/uploads/seed-azul.jpg",
			Published:   true,
		},
		{
			Name:        "Gorra Roja Retro",
			Description: "Gorra de algodón rojo con visera curva y detalles clásicos. Edición de temporada, disponible por tiempo limitado.",
			Color:       "Rojo",
			Price:       88000,
			Stock:       0,
			ImagePath:   "/static/uploads/seed-rojo.jpg",
			Published:   true,
		},
		{
			Name:        "Gorra Beige Safari",
			Description: "Tono beige cálido y tejido de algodón resistente. Una gorra de inspiración utilitaria para días largos al aire libre.",
			Color:       "Beige",
			Price:       115000,
			Stock:       7,
			ImagePath:   "/static/uploads/seed-beige.jpg",
			Published:   true,
		},
	}

	if err := db.Create(&caps).Error; err != nil {
		return fmt.Errorf("seed caps: %w", err)
	}
	log.Printf("[database] seeded %d caps", len(caps))
	return nil
}

func seedAdmin(db *gorm.DB, cfg *config.Config) error {
	var count int64
	if err := db.Model(&models.AdminUser{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	admin := models.AdminUser{
		Username:     cfg.AdminUser,
		PasswordHash: string(hash),
	}
	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	log.Printf("[database] seeded admin user %q (created at %s)", admin.Username, time.Now().Format(time.RFC3339))
	return nil
}
