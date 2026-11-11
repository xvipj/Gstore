package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config contains every runtime setting used by the application.
type Config struct {
	Port              string
	AppBaseURL        string
	DBPath            string
	SessionSecret     string
	AdminUser         string
	AdminPassword     string
	MPAccessToken     string
	MPNotificationURL string
	MPSuccessURL      string
	MPPendingURL      string
	MPFailureURL      string
	TemplatesDir      string
	StaticDir         string
	UploadDir         string
	CookieSecure      bool
}

// Load reads the .env file when present and falls back to safe demo defaults.
func Load() (*Config, error) {
	_ = godotenv.Load()

	baseURL := strings.TrimRight(env("APP_BASE_URL", "http://localhost:8080"), "/")
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("APP_BASE_URL is invalid: %w", err)
	}

	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		random := make([]byte, 32)
		if _, err := rand.Read(random); err != nil {
			return nil, fmt.Errorf("generate session secret: %w", err)
		}
		secret = hex.EncodeToString(random)
		log.Println("[config] SESSION_SECRET is not set; using an ephemeral secret for this process")
	}

	return &Config{
		Port:              env("PORT", "8080"),
		AppBaseURL:        baseURL,
		DBPath:            env("DB_PATH", "tienda.db"),
		SessionSecret:     secret,
		AdminUser:         env("ADMIN_USER", "admin"),
		AdminPassword:     env("ADMIN_PASSWORD", "admin123"),
		MPAccessToken:     strings.TrimSpace(os.Getenv("MP_ACCESS_TOKEN")),
		MPNotificationURL: env("MP_NOTIFICATION_URL", baseURL+"/webhooks/mercadopago"),
		MPSuccessURL:      env("MP_SUCCESS_URL", baseURL+"/pago/exitoso"),
		MPPendingURL:      env("MP_PENDING_URL", baseURL+"/pago/pendiente"),
		MPFailureURL:      env("MP_FAILURE_URL", baseURL+"/pago/fallido"),
		TemplatesDir:      "web/templates",
		StaticDir:         "web/static",
		UploadDir:         "web/static/uploads",
		CookieSecure:      strings.HasPrefix(strings.ToLower(baseURL), "https://"),
	}, nil
}

func (c *Config) MPAccessTokenConfigured() bool {
	return strings.TrimSpace(c.MPAccessToken) != ""
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
