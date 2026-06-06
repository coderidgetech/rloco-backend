package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrderHandler struct {
	orderService   services.OrderService
	productService services.ProductService
	idempotency    repositories.OrderIdempotencyRepository
	configService  services.ConfigService
}

func NewOrderHandler(orderService services.OrderService, productService services.ProductService, idempotency repositories.OrderIdempotencyRepository, configService services.ConfigService) *OrderHandler {
	return &OrderHandler{
		orderService:   orderService,
		productService: productService,
		idempotency:    idempotency,
		configService:  configService,
	}
}

// marketFromCountry maps a shipping country to a region/market code (US or IN).
func marketFromCountry(country string) string {
	switch strings.ToLower(strings.TrimSpace(country)) {
	case "united states", "us", "usa":
		return "US"
	case "india", "in":
		return "IN"
	default:
		return ""
	}
}

// regionAvailability resolves whether a market is open for orders from the live
// SiteConfig (falling back to defaults), returning the coming-soon message when not.
// It fails open only when the regions config is entirely absent, so an unconfigured
// deployment behaves as before rather than rejecting every order.
func (h *OrderHandler) regionAvailability(ctx context.Context, country string) (enabled bool, message string) {
	market := marketFromCountry(country)
	if market == "" {
		return true, ""
	}

	cfg := getDefaultConfig()
	if h.configService != nil {
		if stored, err := h.configService.Get(ctx); err == nil && stored != nil && len(stored.Config) > 0 {
			// ensureRegionDefaults backfills regions for configs predating gating.
			cfg = ensureRegionDefaults(stored.Config)
		}
	}

	general, ok := cfg["general"].(map[string]interface{})
	if !ok {
		return true, ""
	}
	regions, ok := general["regions"].(map[string]interface{})
	if !ok {
		return true, ""
	}
	region, ok := regions[market].(map[string]interface{})
	if !ok {
		return true, ""
	}
	en, ok := region["enabled"].(bool)
	if !ok {
		return true, ""
	}
	msg, _ := region["comingSoonMessage"].(string)
	return en, msg
}

// getVendorProductIDs gets all product IDs for a vendor
func (h *OrderHandler) getVendorProductIDs(ctx context.Context, vendorID primitive.ObjectID) ([]primitive.ObjectID, error) {
	filter := map[string]interface{}{
		"vendor_id": vendorID,
	}
	products, _, err := h.productService.List(ctx, filter, 1000, 0, "newest")
	if err != nil {
		return nil, err
	}

	productIDs := make([]primitive.ObjectID, 0, len(products))
	for _, p := range products {
		productIDs = append(productIDs, p.ID)
	}
	return productIDs, nil
}

func (h *OrderHandler) List(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	id, _ := primitive.ObjectIDFromHex(userIDStr)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))

	// Enforce pagination limits
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if skip < 0 {
		skip = 0
	}

	role, _ := c.Get("role")
	if role == "admin" {
		// Admin can see all orders
		filter := make(map[string]interface{})
		if status := c.Query("status"); status != "" {
			filter["status"] = status
		}
		orders, total, err := h.orderService.List(c.Request.Context(), filter, limit, skip)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"orders": orders, "total": total})
		return
	}

	if role == "vendor" {
		// Vendors can only see orders for their products
		vendorID, vendorIDExists := c.Get("vendor_id")
		if !vendorIDExists || vendorID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Vendor ID not found"})
			return
		}

		var vendorIDObj *primitive.ObjectID
		if id, ok := vendorID.(*primitive.ObjectID); ok {
			vendorIDObj = id
		} else if idStr, ok := vendorID.(string); ok {
			id, err := primitive.ObjectIDFromHex(idStr)
			if err == nil {
				vendorIDObj = &id
			}
		}

		if vendorIDObj == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid vendor ID"})
			return
		}

		// Get vendor's product IDs
		productIDs, err := h.getVendorProductIDs(c.Request.Context(), *vendorIDObj)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get vendor products"})
			return
		}

		// Filter orders to only include those with vendor's products
		// Note: This requires order service to support product ID filtering
		// For now, we'll get all orders and filter in memory (not ideal for large datasets)
		filter := make(map[string]interface{})
		if status := c.Query("status"); status != "" {
			filter["status"] = status
		}
		allOrders, _, err := h.orderService.List(c.Request.Context(), filter, limit*10, skip) // Get more to filter
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Filter orders that contain vendor's products
		vendorOrders := make([]*models.Order, 0)
		productIDMap := make(map[primitive.ObjectID]bool)
		for _, pid := range productIDs {
			productIDMap[pid] = true
		}

		for _, order := range allOrders {
			for _, item := range order.Items {
				if productIDMap[item.ProductID] {
					vendorOrders = append(vendorOrders, order)
					break
				}
			}
		}

		// Apply pagination to filtered results
		start := skip
		if start > len(vendorOrders) {
			start = len(vendorOrders)
		}
		end := start + limit
		if end > len(vendorOrders) {
			end = len(vendorOrders)
		}

		if start < len(vendorOrders) {
			c.JSON(http.StatusOK, gin.H{
				"orders": vendorOrders[start:end],
				"total":  int64(len(vendorOrders)),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"orders": []*models.Order{},
				"total":  int64(len(vendorOrders)),
			})
		}
		return
	}

	// Regular users see only their orders
	orders, total, err := h.orderService.GetByUserID(c.Request.Context(), id, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders, "total": total})
}

func (h *OrderHandler) Get(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	order, err := h.orderService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Check ownership
	role, roleExists := c.Get("role")
	userID, userIDExists := c.Get("user_id")
	if !roleExists || !userIDExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User information not found"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	if role == "admin" {
		// Admin can see all orders
		c.JSON(http.StatusOK, order)
		return
	}

	if role == "customer" {
		// Customers can only see their own orders
		if order.UserID.Hex() != userIDObj.Hex() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: You can only view your own orders"})
			return
		}
		c.JSON(http.StatusOK, order)
		return
	}

	if role == "vendor" {
		// Vendors can only see orders for their products
		vendorID, vendorIDExists := c.Get("vendor_id")
		if !vendorIDExists || vendorID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Vendor ID not found"})
			return
		}

		var vendorIDObj *primitive.ObjectID
		if id, ok := vendorID.(*primitive.ObjectID); ok {
			vendorIDObj = id
		} else if idStr, ok := vendorID.(string); ok {
			id, err := primitive.ObjectIDFromHex(idStr)
			if err == nil {
				vendorIDObj = &id
			}
		}

		if vendorIDObj == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid vendor ID"})
			return
		}

		// Check if order contains vendor's products
		productIDs, err := h.getVendorProductIDs(c.Request.Context(), *vendorIDObj)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get vendor products"})
			return
		}

		productIDMap := make(map[primitive.ObjectID]bool)
		for _, pid := range productIDs {
			productIDMap[pid] = true
		}

		hasVendorProduct := false
		for _, item := range order.Items {
			if productIDMap[item.ProductID] {
				hasVendorProduct = true
				break
			}
		}

		if !hasVendorProduct {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: This order does not contain your products"})
			return
		}

		// Vendor-scoped view: only their line items; redact customer PII beyond fulfillment basics.
		filtered := *order
		var vitems []models.OrderItem
		for _, item := range order.Items {
			if productIDMap[item.ProductID] {
				vitems = append(vitems, item)
			}
		}
		filtered.Items = vitems
		filtered.ShippingInfo = models.ShippingInfo{
			FirstName: order.ShippingInfo.FirstName,
			LastName:  order.ShippingInfo.LastName,
			City:      order.ShippingInfo.City,
			State:     order.ShippingInfo.State,
			Country:   order.ShippingInfo.Country,
			ZipCode:   order.ShippingInfo.ZipCode,
		}
		c.JSON(http.StatusOK, &filtered)
		return
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
}

func (h *OrderHandler) Create(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	id, _ := primitive.ObjectIDFromHex(userIDStr)

	idemKey := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(idemKey) > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key header must be at most 256 characters"})
		return
	}

	if h.idempotency != nil && idemKey != "" {
		if oid, _ := h.idempotency.LookupOrderID(c.Request.Context(), id, idemKey); oid != nil {
			order, err := h.orderService.GetByID(c.Request.Context(), *oid)
			if err == nil {
				c.JSON(http.StatusOK, order)
				return
			}
		}
		leased, err := h.idempotency.TryReserve(c.Request.Context(), id, idemKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not start checkout"})
			return
		}
		if !leased {
			for i := 0; i < 50; i++ {
				if oid, _ := h.idempotency.LookupOrderID(c.Request.Context(), id, idemKey); oid != nil {
					order, err := h.orderService.GetByID(c.Request.Context(), *oid)
					if err == nil {
						c.JSON(http.StatusOK, order)
						return
					}
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			c.JSON(http.StatusConflict, gin.H{"error": "Duplicate checkout in progress; try again shortly"})
			return
		}
	}

	var req struct {
		Items              []models.OrderItem  `json:"items" binding:"required"`
		ShippingInfo       models.ShippingInfo `json:"shipping_info" binding:"required"`
		PaymentInfo        models.PaymentInfo  `json:"payment_info"`
		PaymentMethod      string              `json:"payment_method" binding:"required"`
		PromotionCode      *string             `json:"promotion_code,omitempty"`
		GiftPackingCharge  float64             `json:"gift_packing_charge"` // Optional; e.g. 50 per gift item
		ShippingCarrier    string              `json:"shipping_carrier,omitempty"` // Customer-selected rate (carrier)
		ShippingService    string              `json:"shipping_service,omitempty"` // Customer-selected rate (service level)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		if h.idempotency != nil && idemKey != "" {
			_ = h.idempotency.Release(c.Request.Context(), id, idemKey)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.GiftPackingCharge < 0 {
		req.GiftPackingCharge = 0
	}

	// Region gating: refuse orders shipping to a region that isn't open yet.
	if enabled, msg := h.regionAvailability(c.Request.Context(), req.ShippingInfo.Country); !enabled {
		if h.idempotency != nil && idemKey != "" {
			_ = h.idempotency.Release(c.Request.Context(), id, idemKey)
		}
		if msg == "" {
			msg = "Orders are not available in this region yet."
		}
		c.JSON(http.StatusForbidden, gin.H{"error": msg, "code": "region_unavailable"})
		return
	}

	order, err := h.orderService.Create(c.Request.Context(), id, req.Items, req.ShippingInfo, req.PaymentInfo, req.PaymentMethod, req.PromotionCode, req.GiftPackingCharge, req.ShippingCarrier, req.ShippingService)
	if err != nil {
		if h.idempotency != nil && idemKey != "" {
			_ = h.idempotency.Release(c.Request.Context(), id, idemKey)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.idempotency != nil && idemKey != "" {
		_ = h.idempotency.Commit(c.Request.Context(), id, idemKey, order.ID)
	}
	c.JSON(http.StatusCreated, order)
}

// CreateGuest places an order without authentication. Only Cash on Delivery is
// supported for guest orders. The cart items are submitted inline (no server-side cart).
func (h *OrderHandler) CreateGuest(c *gin.Context) {
	var req struct {
		GuestEmail    string             `json:"guest_email" binding:"required,email"`
		GuestName     string             `json:"guest_name" binding:"required"`
		Items         []models.OrderItem `json:"items" binding:"required"`
		ShippingInfo  models.ShippingInfo `json:"shipping_info" binding:"required"`
		PromotionCode *string             `json:"promotion_code,omitempty"`
		ShippingCarrier string            `json:"shipping_carrier,omitempty"` // Customer-selected rate (carrier)
		ShippingService string            `json:"shipping_service,omitempty"` // Customer-selected rate (service level)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Region gating: refuse orders shipping to a region that isn't open yet.
	if enabled, msg := h.regionAvailability(c.Request.Context(), req.ShippingInfo.Country); !enabled {
		if msg == "" {
			msg = "Orders are not available in this region yet."
		}
		c.JSON(http.StatusForbidden, gin.H{"error": msg, "code": "region_unavailable"})
		return
	}
	order, err := h.orderService.CreateGuestOrder(c.Request.Context(), req.GuestEmail, req.GuestName, req.Items, req.ShippingInfo, "cod", req.PromotionCode, req.ShippingCarrier, req.ShippingService)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, order)
}

func (h *OrderHandler) Track(c *gin.Context) {
	orderNumber := c.Param("orderNumber")
	order, err := h.orderService.GetByOrderNumber(c.Request.Context(), orderNumber)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	role, _ := c.Get("role")
	userIDVal, _ := c.Get("user_id")
	userIDStr, _ := userIDVal.(string)
	userIDObj, _ := primitive.ObjectIDFromHex(userIDStr)
	if role != "admin" && order.UserID != userIDObj {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) GetTracking(c *gin.Context) {
	orderID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Check ownership
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, _ := primitive.ObjectIDFromHex(userIDStr)

	order, err := h.orderService.GetByID(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	role, _ := c.Get("role")
	if role != "admin" && order.UserID != userIDObj {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	updates, err := h.orderService.GetTrackingUpdates(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"updates": updates})
}

func (h *OrderHandler) Cancel(c *gin.Context) {
	orderID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, _ := primitive.ObjectIDFromHex(userIDStr)

	var req struct {
		Reason string `json:"reason,omitempty"`
	}
	c.ShouldBindJSON(&req)

	if err := h.orderService.Cancel(c.Request.Context(), orderID, userIDObj, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order cancelled successfully"})
}

func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	orderID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.orderService.UpdateStatus(c.Request.Context(), orderID, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, _ := h.orderService.GetByID(c.Request.Context(), orderID)
	c.JSON(http.StatusOK, order)
}

// Fulfill purchases a shipping label via the appropriate carrier (Shiprocket for India,
// Shippo for all other destinations), saves the tracking number, and marks the order
// as shipped — firing email/SMS/push notifications automatically.
func (h *OrderHandler) Fulfill(c *gin.Context) {
	orderID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	order, err := h.orderService.FulfillOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}
