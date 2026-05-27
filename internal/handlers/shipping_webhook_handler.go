package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"rloco-backend/internal/repositories"
	"rloco-backend/internal/services"
)

// ShippingWebhookHandler handles inbound tracking events from Shippo and Shiprocket.
type ShippingWebhookHandler struct {
	orderRepo    repositories.OrderRepository
	orderService services.OrderService
	emailService services.EmailService
	shippoSecret     string // optional HMAC secret for Shippo signature verification
	shiprocketSecret string // optional shared token for Shiprocket webhook validation
}

func NewShippingWebhookHandler(
	orderRepo repositories.OrderRepository,
	orderService services.OrderService,
	emailService services.EmailService,
	shippoSecret, shiprocketSecret string,
) *ShippingWebhookHandler {
	return &ShippingWebhookHandler{
		orderRepo:        orderRepo,
		orderService:     orderService,
		emailService:     emailService,
		shippoSecret:     shippoSecret,
		shiprocketSecret: shiprocketSecret,
	}
}

// ── Shippo ────────────────────────────────────────────────────────────────────

type shippoWebhookEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type shippoTrackData struct {
	Carrier        string               `json:"carrier"`
	TrackingNumber string               `json:"tracking_number"`
	TrackingStatus shippoTrackingStatus `json:"tracking_status"`
}

type shippoTrackingStatus struct {
	Status        string    `json:"status"`
	StatusDetails string    `json:"status_details"`
	StatusDate    time.Time `json:"status_date"`
	Location      struct {
		City    string `json:"city"`
		State   string `json:"state"`
		Country string `json:"country"`
	} `json:"location"`
}

// HandleShippoWebhook receives track_updated events from Shippo.
// Shippo signs payloads with HMAC-SHA256; verification is skipped if
// SHIPPO_WEBHOOK_SECRET is not set.
func (h *ShippingWebhookHandler) HandleShippoWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}

	if h.shippoSecret != "" {
		sig := c.GetHeader("Shippo-Webhook-Signature")
		if !verifyShippoSignature(body, sig, h.shippoSecret) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
	}

	var event shippoWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	if event.Event != "track_updated" {
		c.Status(http.StatusNoContent)
		return
	}

	var data shippoTrackData
	if err := json.Unmarshal(event.Data, &data); err != nil || data.TrackingNumber == "" {
		c.Status(http.StatusNoContent)
		return
	}

	h.processTrackingEvent(
		c,
		data.TrackingNumber,
		data.TrackingStatus.Status,
		data.TrackingStatus.Location.City+", "+data.TrackingStatus.Location.State,
		data.TrackingStatus.StatusDetails,
	)
}

func verifyShippoSignature(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ── Shiprocket ────────────────────────────────────────────────────────────────

type shiprocketWebhookPayload struct {
	AWB           string                  `json:"awb"`
	CourierName   string                  `json:"courier_name"`
	CurrentStatus string                  `json:"current_status"`
	Scans         []shiprocketScan        `json:"scans"`
}

type shiprocketScan struct {
	Date     string `json:"date"`
	Activity string `json:"activity"`
	Location string `json:"location"`
}

// HandleShiprocketWebhook receives order status events from Shiprocket.
// If SHIPROCKET_WEBHOOK_SECRET is set, it must match the X-Shiprocket-Token header.
func (h *ShippingWebhookHandler) HandleShiprocketWebhook(c *gin.Context) {
	if h.shiprocketSecret != "" {
		token := c.GetHeader("X-Shiprocket-Token")
		if token != h.shiprocketSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}

	var payload shiprocketWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil || payload.AWB == "" {
		c.Status(http.StatusNoContent)
		return
	}

	location := ""
	description := payload.CurrentStatus
	if len(payload.Scans) > 0 {
		latest := payload.Scans[len(payload.Scans)-1]
		location = latest.Location
		if latest.Activity != "" {
			description = latest.Activity
		}
	}

	h.processTrackingEvent(c, payload.AWB, payload.CurrentStatus, location, description)
}

// ── shared ────────────────────────────────────────────────────────────────────

// processTrackingEvent looks up the order by tracking number, appends a tracking
// update, and sends the customer a notification for significant status changes.
func (h *ShippingWebhookHandler) processTrackingEvent(
	c *gin.Context,
	trackingNumber, status, location, description string,
) {
	ctx := c.Request.Context()

	order, err := h.orderRepo.GetByTrackingNumber(ctx, trackingNumber)
	if err != nil || order == nil {
		// Unknown tracking number — acknowledge so the carrier doesn't retry indefinitely
		c.Status(http.StatusNoContent)
		return
	}

	tn := trackingNumber
	_ = h.orderService.AddTrackingUpdate(ctx, order.ID, status, location, description, &tn)

	// Notify the customer for meaningful milestones
	normalised := strings.ToUpper(strings.TrimSpace(status))
	switch normalised {
	case "TRANSIT", "IN TRANSIT", "IN_TRANSIT", "DELIVERED", "DELIVERY EXCEPTION", "FAILED":
		_ = h.emailService.SendOrderStatusUpdate(order.ShippingInfo.Email, order.OrderNumber, status)
	}

	// If carrier confirms delivery, advance the order status
	if normalised == "DELIVERED" {
		orderID, err := primitive.ObjectIDFromHex(order.ID.Hex())
		if err == nil {
			_ = h.orderService.UpdateStatus(ctx, orderID, "delivered")
		}
	}

	c.Status(http.StatusNoContent)
}
