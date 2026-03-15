package handlers

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/services"
)

type PaymentHandler struct {
	paymentService services.PaymentService
}

func NewPaymentHandler(paymentService services.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

func (h *PaymentHandler) CreatePaymentIntent(c *gin.Context) {
	var req struct {
		OrderID        string  `json:"order_id" binding:"required"`
		Amount         float64 `json:"amount" binding:"required"`
		Currency       string  `json:"currency" binding:"required"`
		Gateway        string  `json:"gateway" binding:"required"` // "stripe" or "paypal"
		PaymentMethod  string  `json:"payment_method"`               // "card", "upi", "wallet" - so Stripe shows correct method (e.g. UPI first for INR)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Gateway != "stripe" && req.Gateway != "paypal" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gateway must be 'stripe' or 'paypal'"})
		return
	}

	orderID, err := primitive.ObjectIDFromHex(req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	if req.Currency == "" {
		req.Currency = "usd"
	}
	// Normalize so "INR" from client becomes "inr" for Stripe
	req.Currency = strings.ToLower(strings.TrimSpace(req.Currency))

	intent, err := h.paymentService.CreatePaymentIntent(c.Request.Context(), orderID, req.Amount, req.Currency, req.Gateway, req.PaymentMethod)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, intent)
}

func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	var req struct {
		PaymentIntentID string `json:"payment_intent_id" binding:"required"`
		PaymentMethodID string `json:"payment_method_id" binding:"required"`
		Gateway         string `json:"gateway" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.paymentService.ProcessPayment(c.Request.Context(), req.PaymentIntentID, req.PaymentMethodID, req.Gateway); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payment processed successfully"})
}

func (h *PaymentHandler) HandleWebhook(c *gin.Context) {
	gateway := c.Param("gateway")
	if gateway != "stripe" && gateway != "paypal" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gateway"})
		return
	}

	// Read request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read payload"})
		return
	}

	// Get signature from header
	signature := c.GetHeader("X-Stripe-Signature")
	if gateway == "paypal" {
		signature = c.GetHeader("PAYPAL-AUTH-ALGO")
	}

	if err := h.paymentService.HandleWebhook(c.Request.Context(), gateway, payload, signature); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Webhook processed"})
}

func (h *PaymentHandler) Refund(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	var req struct {
		Amount *float64 `json:"amount,omitempty"` // Optional: partial refund
	}

	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.paymentService.RefundPayment(c.Request.Context(), id, req.Amount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Refund processed successfully"})
}

func (h *PaymentHandler) GetTransaction(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction ID"})
		return
	}

	transaction, err := h.paymentService.GetTransaction(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	c.JSON(http.StatusOK, transaction)
}
