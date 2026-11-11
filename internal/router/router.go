package router

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tienda-gorras/internal/config"
	"tienda-gorras/internal/handlers"
	"tienda-gorras/internal/middleware"
	"tienda-gorras/internal/payments"
	"tienda-gorras/internal/repository"
	"tienda-gorras/internal/services"
)

const adminBodyLimit = 5 << 20

// New wires repositories, services, handlers and routes.
func New(cfg *config.Config, db *gorm.DB) (*gin.Engine, error) {
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return nil, err
	}

	renderer := handlers.NewRenderer(cfg.TemplatesDir)
	sessions := middleware.NewSessionManager(cfg.SessionSecret, cfg.CookieSecure)

	capRepo := repository.NewCapRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	adminRepo := repository.NewAdminRepository(db)

	catalogService := services.NewCatalogService(capRepo)
	cartService := services.NewCartService(capRepo)
	adminService := services.NewAdminService(adminRepo, capRepo, orderRepo, cfg.UploadDir)
	paymentClient, err := payments.New(cfg.MPAccessToken)
	if err != nil {
		return nil, err
	}
	checkoutService := services.NewCheckoutService(cartService, orderRepo, paymentClient, services.CheckoutURLs{
		Success: cfg.MPSuccessURL,
		Pending: cfg.MPPendingURL,
		Failure: cfg.MPFailureURL,
		Webhook: cfg.MPNotificationURL,
	})
	paymentService := services.NewPaymentService(db, paymentClient)

	storeHandler := handlers.NewStoreHandler(renderer, catalogService, cartService, checkoutService)
	adminHandler := handlers.NewAdminHandler(renderer, adminService, sessions)
	paymentHandler := handlers.NewPaymentHandler(paymentService)

	engine := gin.New()
	engine.MaxMultipartMemory = 2 << 20
	_ = engine.SetTrustedProxies(nil)
	engine.Use(
		gin.Logger(),
		middleware.NoStore(),
		middleware.Recovery(renderer.RenderErrorPage),
	)
	engine.Static("/static", cfg.StaticDir)

	engine.GET("/", storeHandler.Home)
	engine.GET("/carrito", storeHandler.CartPage)
	engine.POST("/api/cart", middleware.LimitBody(1<<20), storeHandler.CartPreview)
	engine.POST("/checkout", middleware.LimitBody(1<<20), storeHandler.Checkout)
	engine.GET("/pago/exitoso", storeHandler.PaymentSuccess)
	engine.GET("/pago/pendiente", storeHandler.PaymentPending)
	engine.GET("/pago/fallido", storeHandler.PaymentFailure)
	engine.POST("/webhooks/mercadopago", middleware.LimitBody(1<<20), paymentHandler.MercadoPagoWebhook)

	admin := engine.Group("/admin")
	admin.GET("/login", adminHandler.LoginPage)
	admin.POST("/login", middleware.LimitBody(1<<20), adminHandler.Login)

	protected := admin.Group("")
	protected.Use(middleware.RequireAdmin(sessions))
	protected.GET("", adminHandler.Dashboard)
	protected.POST("/logout", middleware.LimitBody(1<<20), middleware.RequireCSRF(sessions), adminHandler.Logout)
	protected.GET("/caps/new", adminHandler.NewCapForm)
	protected.POST("/caps", middleware.LimitBody(adminBodyLimit), middleware.RequireCSRF(sessions), adminHandler.CreateCap)
	protected.GET("/caps/:id/edit", adminHandler.EditCapForm)
	protected.POST("/caps/:id", middleware.LimitBody(adminBodyLimit), middleware.RequireCSRF(sessions), adminHandler.UpdateCap)
	protected.POST("/caps/:id/delete", middleware.LimitBody(1<<20), middleware.RequireCSRF(sessions), adminHandler.DeleteCap)

	engine.NoRoute(func(c *gin.Context) {
		renderer.RenderErrorPage(c, http.StatusNotFound)
	})

	return engine, nil
}
