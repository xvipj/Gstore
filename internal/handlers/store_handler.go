package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"tienda-gorras/internal/services"
)

type StoreHandler struct {
	renderer *Renderer
	catalog  *services.CatalogService
	cart     *services.CartService
	checkout *services.CheckoutService
}

func NewStoreHandler(
	renderer *Renderer,
	catalog *services.CatalogService,
	cart *services.CartService,
	checkout *services.CheckoutService,
) *StoreHandler {
	return &StoreHandler{renderer: renderer, catalog: catalog, cart: cart, checkout: checkout}
}

func (h *StoreHandler) Home(c *gin.Context) {
	color := strings.TrimSpace(c.Query("color"))

	caps, err := h.catalog.ListCaps(c.Request.Context(), color)
	if err != nil {
		h.renderer.internalError(c, err)
		return
	}
	colors, err := h.catalog.Colors(c.Request.Context())
	if err != nil {
		h.renderer.internalError(c, err)
		return
	}

	data := StorePageData{
		BaseData:      baseData("Catálogo", "home"),
		Caps:          caps,
		Colors:        colors,
		SelectedColor: color,
	}
	if err := h.renderer.RenderPublic(c, http.StatusOK, "home.html", data); err != nil {
		h.renderer.internalError(c, err)
	}
}

func (h *StoreHandler) CartPage(c *gin.Context) {
	data := CartPageData{BaseData: baseData("Carrito", "cart")}
	if err := h.renderer.RenderPublic(c, http.StatusOK, "cart.html", data); err != nil {
		h.renderer.internalError(c, err)
	}
}

func (h *StoreHandler) CartPreview(c *gin.Context) {
	var payload struct {
		Items []services.CartItemInput `json:"items"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El carrito no tiene un formato válido."})
		return
	}

	preview, err := h.cart.Preview(c.Request.Context(), payload.Items)
	if err != nil {
		status, message := cartErrorMessage(err)
		if status >= http.StatusInternalServerError {
			c.Error(err) //nolint:errcheck
		}
		c.JSON(status, gin.H{"error": message})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (h *StoreHandler) Checkout(c *gin.Context) {
	var payload struct {
		Items []services.CartItemInput `json:"items"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El carrito no tiene un formato válido."})
		return
	}

	result, err := h.checkout.CreateCheckout(c.Request.Context(), payload.Items)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPaymentsNotConfigured):
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Mercado Pago no está configurado. Define MP_ACCESS_TOKEN en el archivo .env.",
			})
		case errors.Is(err, services.ErrEmptyCart):
			c.JSON(http.StatusBadRequest, gin.H{"error": "El carrito está vacío."})
		case errors.Is(err, services.ErrInvalidQuantity):
			c.JSON(http.StatusBadRequest, gin.H{"error": "La cantidad de uno de los productos no es válida."})
		case errors.Is(err, services.ErrCapUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "Una de las gorras ya no está disponible. Actualiza el carrito."})
		default:
			var stockErr *services.StockError
			if errors.As(err, &stockErr) {
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("No hay stock suficiente de %s. Disponibles: %d.", stockErr.CapName, stockErr.Available),
				})
				return
			}
			c.Error(err) //nolint:errcheck
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo iniciar el pago. Intenta de nuevo."})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"init_point": result.InitPoint})
}

func (h *StoreHandler) PaymentSuccess(c *gin.Context) {
	h.renderResult(c, "payment-success", "Pago exitoso", "Recibimos tu pago. La orden se actualizará cuando Mercado Pago confirme la transacción.", true)
}

func (h *StoreHandler) PaymentPending(c *gin.Context) {
	h.renderResult(c, "payment-pending", "Pago pendiente", "Tu pago está siendo procesado. Te recomendamos conservar los datos de la orden.", false)
}

func (h *StoreHandler) PaymentFailure(c *gin.Context) {
	h.renderResult(c, "payment-failure", "Pago fallido", "El pago no se pudo completar. No se descontó stock y puedes intentarlo de nuevo.", false)
}

func (h *StoreHandler) renderResult(c *gin.Context, page, heading, message string, success bool) {
	data := ResultPageData{
		BaseData: baseData(heading, page),
		Heading:  heading,
		Message:  message,
		Success:  success,
	}
	if err := h.renderer.RenderPublic(c, http.StatusOK, "payment_result.html", data); err != nil {
		h.renderer.internalError(c, err)
	}
}

func cartErrorMessage(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrEmptyCart):
		return http.StatusBadRequest, "El carrito está vacío."
	case errors.Is(err, services.ErrInvalidQuantity):
		return http.StatusBadRequest, "La cantidad de uno de los productos no es válida."
	case errors.Is(err, services.ErrCapUnavailable):
		return http.StatusConflict, "Una de las gorras ya no está disponible. Actualiza el carrito."
	}

	var stockErr *services.StockError
	if errors.As(err, &stockErr) {
		return http.StatusConflict, fmt.Sprintf("No hay stock suficiente de %s. Disponibles: %d.", stockErr.CapName, stockErr.Available)
	}
	return http.StatusInternalServerError, "No se pudo cargar el carrito."
}
