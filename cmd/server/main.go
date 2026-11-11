package main

import (
	"log"

	"tienda-gorras/internal/config"
	"tienda-gorras/internal/database"
	"tienda-gorras/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get database handle: %v", err)
	}
	defer sqlDB.Close()

	engine, err := router.New(cfg, db)
	if err != nil {
		log.Fatalf("configure router: %v", err)
	}

	log.Printf("Gorras store running at %s", cfg.AppBaseURL)
	if !cfg.MPAccessTokenConfigured() {
		log.Println("Mercado Pago is not configured; checkout will return a clear error until MP_ACCESS_TOKEN is set")
	}

	if err := engine.Run(":" + cfg.Port); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
