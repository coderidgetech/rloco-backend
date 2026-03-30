package handlers

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/services"
)

func requesterFromContext(c *gin.Context) (primitive.ObjectID, string, bool) {
	roleVal, ok := c.Get("role")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Role missing"})
		return primitive.NilObjectID, "", false
	}
	role, ok := roleVal.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid role"})
		return primitive.NilObjectID, "", false
	}
	userIDVal, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User missing"})
		return primitive.NilObjectID, "", false
	}
	userIDStr, ok := userIDVal.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user"})
		return primitive.NilObjectID, "", false
	}
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user"})
		return primitive.NilObjectID, "", false
	}
	return userID, role, true
}

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
		Gateway        string  `json:"gateway" binding:"required"` // "stripe"
		PaymentMethod  string  `json:"payment_method"`               // "card", "upi", "wallet" - so Stripe shows correct method (e.g. UPI first for INR)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Gateway != "stripe" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gateway must be 'stripe'"})
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

	userID, role, ok := requesterFromContext(c)
	if !ok {
		return
	}
	intent, err := h.paymentService.CreatePaymentIntent(c.Request.Context(), userID, role, orderID, req.Amount, req.Currency, req.Gateway, req.PaymentMethod)
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

	if req.Gateway != "stripe" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gateway must be 'stripe'"})
		return
	}

	userID, role, ok := requesterFromContext(c)
	if !ok {
		return
	}
	if err := h.paymentService.ProcessPayment(c.Request.Context(), userID, role, req.PaymentIntentID, req.PaymentMethodID, req.Gateway); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payment processed successfully"})
}

func (h *PaymentHandler) HandleWebhook(c *gin.Context) {
	gateway := c.Param("gateway")
	if gateway != "stripe" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gateway"})
		return
	}

	// Read request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read payload"})
		return
	}

	signature := c.GetHeader("X-Stripe-Signature")

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

	userID, role, ok := requesterFromContext(c)
	if !ok {
		return
	}
	transaction, err := h.paymentService.GetTransaction(c.Request.Context(), userID, role, id)
	if err != nil {
		status := http.StatusNotFound
		if strings.Contains(strings.ToLower(err.Error()), "access denied") {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transaction)
}
