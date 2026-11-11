package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"tienda-gorras/internal/services"
)

type PaymentHandler struct {
	payments *services.PaymentService
}

func NewPaymentHandler(payments *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{payments: payments}
}

// MercadoPagoWebhook receives Checkout Pro notifications. The body is only used
// to discover the payment id; the actual status is always fetched from the API.
func (h *PaymentHandler) MercadoPagoWebhook(c *gin.Context) {
	notificationType := strings.TrimSpace(c.Query("type"))
	if notificationType == "" {
		notificationType = strings.TrimSpace(c.Query("topic"))
	}
	paymentIDText := firstNonEmpty(
		c.Query("data.id"),
		c.Query("data_id"),
		c.Query("id"),
	)

	var body struct {
		Type   string `json:"type"`
		Action string `json:"action"`
		Data   struct {
			ID json.RawMessage `json:"id"`
		} `json:"data"`
	}
	bodyBytes, _ := readLimited(c.Request.Body, 1<<20)
	if len(bytes.TrimSpace(bodyBytes)) > 0 {
		_ = json.Unmarshal(bodyBytes, &body)
	}
	if notificationType == "" {
		notificationType = body.Type
	}
	if notificationType == "" && strings.HasPrefix(body.Action, "payment.") {
		notificationType = "payment"
	}
	if paymentIDText == "" && len(body.Data.ID) > 0 {
		paymentIDText = rawJSONID(body.Data.ID)
	}

	if notificationType != "" && notificationType != "payment" {
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}
	if paymentIDText == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing payment id"})
		return
	}

	paymentID, err := strconv.Atoi(paymentIDText)
	if err != nil || paymentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}

	if err := h.payments.ProcessPayment(c.Request.Context(), paymentID); err != nil {
		switch {
		case errors.Is(err, services.ErrPaymentsNotConfigured):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payments not configured"})
		case errors.Is(err, services.ErrOrderNotFound),
			errors.Is(err, services.ErrPaymentReferenceMismatch),
			errors.Is(err, services.ErrPaymentAmountMismatch),
			errors.Is(err, services.ErrPaymentCurrencyMismatch):
			// Acknowledge permanently invalid notifications so they are not retried.
			log.Printf("[webhook] payment %d ignored: %v", paymentID, err)
			c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		default:
			log.Printf("[webhook] payment %d failed: %v", paymentID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "payment processing failed"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func rawJSONID(raw json.RawMessage) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return ""
	}
	switch id := value.(type) {
	case string:
		return strings.TrimSpace(id)
	case json.Number:
		return id.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
