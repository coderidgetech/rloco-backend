package handlers

import (
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

type AdminHandler struct {
	productService        services.ProductService
	orderService          services.OrderService
	userRepo              repositories.UserRepository
	vendorService         services.VendorService
	promotionService      services.PromotionService
	analyticsService      services.AnalyticsService
	configService         services.ConfigService
	advancedAnalyticsRepo repositories.AdvancedAnalyticsRepository
}

func NewAdminHandler(
	productService services.ProductService,
	orderService services.OrderService,
	userRepo repositories.UserRepository,
	vendorService services.VendorService,
	promotionService services.PromotionService,
	analyticsService services.AnalyticsService,
	configService services.ConfigService,
	advancedAnalyticsRepo repositories.AdvancedAnalyticsRepository,
) *AdminHandler {
	return &AdminHandler{
		productService:        productService,
		orderService:          orderService,
		userRepo:              userRepo,
		vendorService:         vendorService,
		promotionService:      promotionService,
		analyticsService:      analyticsService,
		configService:         configService,
		advancedAnalyticsRepo: advancedAnalyticsRepo,
	}
}

func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	now := time.Now()
	// Current period: last 30 days
	currentEnd := now
	currentStart := now.AddDate(0, 0, -30)
	// Previous period: 30-60 days ago for comparison
	prevEnd := currentStart
	prevStart := now.AddDate(0, 0, -60)

	currentRevenue, _ := h.analyticsService.GetRevenueAnalytics(c.Request.Context(), currentStart, currentEnd)
	prevRevenue, _ := h.analyticsService.GetRevenueAnalytics(c.Request.Context(), prevStart, prevEnd)
	customerStats, _ := h.analyticsService.GetCustomerAnalytics(c.Request.Context(), currentStart, currentEnd)
	prevCustomerStats, _ := h.analyticsService.GetCustomerAnalytics(c.Request.Context(), prevStart, prevEnd)

	// Count total products
	_, totalProducts, _ := h.productService.List(c.Request.Context(), map[string]interface{}{}, 1, 0, "")

	toFloat := func(m map[string]interface{}, key string) float64 {
		if m == nil {
			return 0
		}
		switch v := m[key].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case int32:
			return float64(v)
		}
		return 0
	}

	changePct := func(current, prev float64) float64 {
		if prev == 0 {
			if current > 0 {
				return 100
			}
			return 0
		}
		return ((current - prev) / prev) * 100
	}

	totalRevenue := toFloat(currentRevenue, "total_revenue")
	prevTotalRevenue := toFloat(prevRevenue, "total_revenue")
	totalOrders := toFloat(currentRevenue, "total_orders")
	prevTotalOrders := toFloat(prevRevenue, "total_orders")
	totalCustomers := toFloat(customerStats, "total_customers")
	prevTotalCustomers := toFloat(prevCustomerStats, "total_customers")

	c.JSON(http.StatusOK, gin.H{
		"total_revenue":    totalRevenue,
		"revenue_change":   changePct(totalRevenue, prevTotalRevenue),
		"total_orders":     int(totalOrders),
		"orders_change":    changePct(totalOrders, prevTotalOrders),
		"total_customers":  int(totalCustomers),
		"customers_change": changePct(totalCustomers, prevTotalCustomers),
		"total_products":   totalProducts,
		"products_change":  0,
	})
}

func (h *AdminHandler) GetDashboardSales(c *gin.Context) {
	endDate, startDate := parseDateRangeOrDays(c, 7)

	stats, err := h.analyticsService.GetRevenueAnalytics(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

func (h *AdminHandler) GetDashboardOrders(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 10
	}
	filter := make(map[string]interface{})
	orders, total, err := h.orderService.List(c.Request.Context(), filter, limit, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": orders, "total": total})
}

func (h *AdminHandler) GetDashboardProducts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 50 {
		limit = 50
	}
	if limit < 1 {
		limit = 10
	}

	products, total, err := h.productService.List(c.Request.Context(), map[string]interface{}{}, limit, 0, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": products, "total": total})
}

// parseDateRangeOrDays parses start_date/end_date ISO strings from query params.
// Falls back to a rolling window of `defaultDays` days if not provided.
func parseDateRangeOrDays(c *gin.Context, defaultDays int) (endDate, startDate time.Time) {
	endDate = time.Now()
	startDate = endDate.AddDate(0, 0, -defaultDays)

	if s := c.Query("start_date"); s != "" {
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			startDate = parsed
		}
	}
	if e := c.Query("end_date"); e != "" {
		if parsed, err := time.Parse(time.RFC3339, e); err == nil {
			endDate = parsed
		}
	}
	// Fallback to days param
	if s := c.Query("start_date"); s == "" {
		if days, err := strconv.Atoi(c.DefaultQuery("days", strconv.Itoa(defaultDays))); err == nil && days > 0 {
			startDate = endDate.AddDate(0, 0, -days)
		}
	}
	return
}

func (h *AdminHandler) ListCustomers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if skip < 0 {
		skip = 0
	}

	customers, err := h.userRepo.List(c.Request.Context(), limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, customers)
}

func (h *AdminHandler) GetCustomer(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	customer, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	c.JSON(http.StatusOK, customer)
}

func (h *AdminHandler) UpdateCustomer(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userRepo.Update(c.Request.Context(), id, &user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Customer updated successfully"})
}

func (h *AdminHandler) ListVendors(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if skip < 0 {
		skip = 0
	}

	vendors, total, err := h.vendorService.List(c.Request.Context(), limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"vendors": vendors, "total": total})
}

func (h *AdminHandler) GetVendor(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid vendor ID"})
		return
	}

	vendor, err := h.vendorService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor not found"})
		return
	}

	c.JSON(http.StatusOK, vendor)
}

func (h *AdminHandler) CreateVendor(c *gin.Context) {
	var req struct {
		models.Vendor
		InitialPassword string                 `json:"initial_password,omitempty"`
		Metadata        map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	v := req.Vendor
	if req.Metadata != nil {
		if p, ok := req.Metadata["permissions"].(map[string]interface{}); ok && len(p) > 0 {
			v.Permissions = p
		}
		if code, ok := req.Metadata["subscriptionPlanCode"].(string); ok && strings.TrimSpace(code) != "" {
			v.SubscriptionPlan = strings.TrimSpace(code)
		} else if planObj, ok := req.Metadata["subscriptionPlan"].(map[string]interface{}); ok {
			if cn, ok := planObj["configName"].(string); ok && strings.TrimSpace(cn) != "" {
				v.SubscriptionPlan = strings.TrimSpace(cn)
			}
		}
		skipMeta := map[string]bool{
			"permissions": true, "subscriptionPlan": true, "subscriptionPlanId": true, "subscriptionPlanCode": true,
		}
		pref := make(map[string]interface{})
		for k, val := range req.Metadata {
			if skipMeta[k] {
				continue
			}
			pref[k] = val
		}
		if len(pref) > 0 {
			if v.Preferences == nil {
				v.Preferences = pref
			} else {
				for k, val := range pref {
					v.Preferences[k] = val
				}
			}
		}
	}

	result, err := h.vendorService.Create(c.Request.Context(), &v, req.InitialPassword)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

func (h *AdminHandler) UpdateVendor(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid vendor ID"})
		return
	}

	var vendor models.Vendor
	if err := c.ShouldBindJSON(&vendor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.vendorService.Update(c.Request.Context(), id, &vendor); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vendor updated successfully"})
}

func (h *AdminHandler) DeleteVendor(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid vendor ID"})
		return
	}

	if err := h.vendorService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vendor deleted successfully"})
}

func (h *AdminHandler) UpdateVendorPermissions(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid vendor ID"})
		return
	}

	var req struct {
		Permissions map[string]interface{} `json:"permissions" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.vendorService.UpdatePermissions(c.Request.Context(), id, req.Permissions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissions updated successfully"})
}

func (h *AdminHandler) ListPromotions(c *gin.Context) {
	promotions, err := h.promotionService.List(c.Request.Context(), false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, promotions)
}

func (h *AdminHandler) CreatePromotion(c *gin.Context) {
	var promotion models.Promotion
	if err := c.ShouldBindJSON(&promotion); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.promotionService.Create(c.Request.Context(), &promotion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, promotion)
}

func (h *AdminHandler) UpdatePromotion(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid promotion ID"})
		return
	}

	var promotion models.Promotion
	if err := c.ShouldBindJSON(&promotion); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.promotionService.Update(c.Request.Context(), id, &promotion); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Promotion updated successfully"})
}

func (h *AdminHandler) DeletePromotion(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid promotion ID"})
		return
	}

	if err := h.promotionService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Promotion deleted successfully"})
}

func (h *AdminHandler) GetRevenueAnalytics(c *gin.Context) {
	endDate, startDate := parseDateRangeOrDays(c, 30)

	stats, err := h.analyticsService.GetRevenueAnalytics(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) GetOrderAnalytics(c *gin.Context) {
	endDate, startDate := parseDateRangeOrDays(c, 30)

	stats, err := h.analyticsService.GetOrderAnalytics(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) GetProductAnalytics(c *gin.Context) {
	endDate, startDate := parseDateRangeOrDays(c, 30)

	stats, err := h.analyticsService.GetProductAnalytics(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) GetCustomerAnalytics(c *gin.Context) {
	endDate, startDate := parseDateRangeOrDays(c, 30)

	stats, err := h.analyticsService.GetCustomerAnalytics(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) GetTrafficAnalytics(c *gin.Context) {
	endDate, startDate := parseDateRangeOrDays(c, 30)

	stats, err := h.analyticsService.GetTrafficAnalytics(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) GetContent(c *gin.Context) {
	siteConfig, err := h.configService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if siteConfig.Config == nil || len(siteConfig.Config) == 0 {
		c.JSON(http.StatusOK, getDefaultConfig())
		return
	}
	c.JSON(http.StatusOK, siteConfig.Config)
}

func (h *AdminHandler) UpdateContent(c *gin.Context) {
	existingConfig, err := h.configService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get existing configuration"})
		return
	}

	var incomingData map[string]interface{}
	if err := c.ShouldBindJSON(&incomingData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Merge incoming content keys into the existing config map
	configMap := existingConfig.Config
	if configMap == nil {
		configMap = getDefaultConfig()
	}
	for k, v := range incomingData {
		configMap[k] = v
	}

	updatedConfig := &models.SiteConfig{
		ID:        existingConfig.ID,
		Config:    configMap,
		UpdatedAt: time.Now(),
	}

	if err := h.configService.Update(c.Request.Context(), updatedConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Content updated successfully"})
}

func (h *AdminHandler) GetSettings(c *gin.Context) {
	siteConfig, err := h.configService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if siteConfig.Config == nil || len(siteConfig.Config) == 0 {
		c.JSON(http.StatusOK, getDefaultConfig())
		return
	}
	c.JSON(http.StatusOK, siteConfig.Config)
}

func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	existingConfig, err := h.configService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get existing configuration"})
		return
	}

	var incomingData map[string]interface{}
	if err := c.ShouldBindJSON(&incomingData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Merge incoming settings keys into the existing config map
	configMap := existingConfig.Config
	if configMap == nil {
		configMap = getDefaultConfig()
	}
	for k, v := range incomingData {
		configMap[k] = v
	}

	updatedConfig := &models.SiteConfig{
		ID:        existingConfig.ID,
		Config:    configMap,
		UpdatedAt: time.Now(),
	}

	if err := h.configService.Update(c.Request.Context(), updatedConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

// getDefaultConfig returns the default configuration matching the frontend defaultConfig
func getDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"general": map[string]interface{}{
			"siteName":     "Rloco",
			"tagline":      "Modern Luxury Fashion",
			"description":  "Rloco is a premium fashion retailer offering curated collections of luxury apparel, accessories, and beauty products.",
			"email":        "hello@rloco.com",
			"phone":        "+1 (555) 123-4567",
			"supportEmail": "support@rloco.com",
			"address":      "123 Fashion Avenue, New York, NY 10001, United States",
			"socialMedia": map[string]interface{}{
				"instagram": "@rloco",
				"facebook":  "facebook.com/rloco",
				"twitter":   "@rloco",
				"pinterest": "pinterest.com/rloco",
			},
			"currency":           "usd",
			"timezone":           "america/new_york",
			"dateFormat":         "mm/dd/yyyy",
			"multiCurrency":      true,
			"autoDetectLocation": true,
			"maintenanceMode":    false,
			"maintenanceMessage": "We're currently performing scheduled maintenance. We'll be back soon!",
			// Region availability. Clients gate the storefront/checkout on this; the
			// backend also rejects orders shipping to a disabled region (see order flow).
			"regions": map[string]interface{}{
				"US": map[string]interface{}{
					"enabled": true,
					"status":  "live",
				},
				"IN": map[string]interface{}{
					"enabled":           false,
					"status":            "coming_soon",
					"comingSoonMessage": "We're launching in India soon. Stay tuned!",
				},
			},
		},
		"design": map[string]interface{}{
			"colors": map[string]interface{}{
				"primary":            "#B4770E",
				"primaryLight":       "#D4970E",
				"primaryDark":        "#8B5A0B",
				"secondary":          "#000000",
				"secondaryGray":      "#666666",
				"secondaryLightGray": "#999999",
				"dominant":           "#FFFFFF",
				"dominantOffWhite":   "#F8F8F8",
				"dominantLight":      "#F5F5F5",
			},
			"typography": map[string]interface{}{
				"headingFont":   "Inter",
				"bodyFont":      "Inter",
				"baseFontSize":  "16",
				"lineHeight":    "1.5",
				"letterSpacing": "normal",
			},
			"layout": map[string]interface{}{
				"borderRadius":   "0",
				"containerWidth": "1920",
				"sectionSpacing": "large",
			},
			"animations": map[string]interface{}{
				"enabled":      true,
				"speed":        "normal",
				"hoverEffects": "subtle",
			},
		},
		"homepage": map[string]interface{}{
			"hero": map[string]interface{}{
				"enabled":           true,
				"heading":           "",
				"subheading":        "",
				"primaryButtonText": "Shop Collection",
				"primaryButtonLink": "/shop",
				"backgroundImage":   "",
				"style":             "fullscreen",
			},
			"sections": map[string]interface{}{
				"featuredProducts":  true,
				"newArrivals":       true,
				"shopByCategory":    true,
				"bestSellers":       true,
				"editorialFeatures": true,
				"promotionalBanner": true,
				"testimonials":      true,
				"brandStory":        false,
				"instagramFeed":     false,
				"newsletterSignup":  true,
			},
			"featuredCollections": []interface{}{"new", "women", "accessories"},
		},
		"navigation": map[string]interface{}{
			"header": map[string]interface{}{
				"style":        "transparent",
				"height":       "80",
				"sticky":       true,
				"showSearch":   true,
				"showCurrency": true,
			},
			"megaMenu": map[string]interface{}{
				"style": "compact",
			},
			"footer": map[string]interface{}{
				"style":            "multi-column",
				"showNewsletter":   true,
				"showSocial":       true,
				"showPaymentIcons": true,
				"copyrightText":    "© 2026 Rloco. All rights reserved.",
			},
		},
		"categories": map[string]interface{}{
			"women": map[string]interface{}{
				"clothing":    []interface{}{"Dresses", "Tops", "Bottoms", "Outerwear", "Knitwear"},
				"accessories": []interface{}{"Shoes", "Jewelry", "Bags"},
			},
			"men": map[string]interface{}{
				"clothing":    []interface{}{"Shirts", "Tops", "Bottoms", "Outerwear", "Knitwear"},
				"accessories": []interface{}{"Shoes", "Accessories"},
			},
		},
		"email": map[string]interface{}{
			"smtp": map[string]interface{}{
				"host":      "smtp.sendgrid.net",
				"port":      "587",
				"username":  "apikey",
				"password":  "",
				"fromEmail": "noreply@rloco.com",
				"fromName":  "Rloco",
			},
			"sms": map[string]interface{}{
				"enabled":           false,
				"twilioAccountSid":  "",
				"twilioAuthToken":   "",
				"twilioPhoneNumber": "",
				"notifications": map[string]interface{}{
					"orderPlaced":    false,
					"orderShipped":   true,
					"outForDelivery": true,
					"delivered":      true,
				},
			},
		},
		"seo": map[string]interface{}{
			"meta": map[string]interface{}{
				"title":        "Rloco - Modern Luxury Fashion",
				"description":  "Shop curated luxury fashion at Rloco. Discover timeless pieces from the world's finest designers. Free shipping on orders over $100.",
				"keywords":     "luxury fashion, designer clothing, high-end fashion, premium accessories",
				"canonicalUrl": "https://rloco.com",
			},
			"openGraph": map[string]interface{}{
				"title":           "Rloco - Modern Luxury Fashion",
				"description":     "Discover curated luxury fashion collections at Rloco.",
				"image":           "",
				"twitterCardType": "summary_large",
			},
			"sitemap": map[string]interface{}{
				"autoGenerate": true,
			},
			"schema": map[string]interface{}{
				"organization": true,
				"product":      true,
				"breadcrumb":   true,
			},
		},
		"analytics": map[string]interface{}{
			"googleAnalytics": map[string]interface{}{
				"enabled":           false,
				"measurementId":     "",
				"enhancedEcommerce": true,
				"trackUserId":       true,
			},
			"facebookPixel": map[string]interface{}{
				"enabled": false,
				"pixelId": "",
				"events": map[string]interface{}{
					"pageView":         true,
					"viewContent":      true,
					"addToCart":        true,
					"initiateCheckout": true,
					"purchase":         true,
				},
			},
			"other": map[string]interface{}{
				"googleTagManager":   "",
				"tiktokPixel":        "",
				"pinterestTag":       "",
				"hotjarSiteId":       "",
				"customHeaderScript": "",
			},
		},
		"badges": map[string]interface{}{
			"display": map[string]interface{}{
				"position":      "top-left",
				"size":          "medium",
				"shape":         "rounded",
				"allowMultiple": true,
				"maxBadges":     "2",
			},
		},
		// Admin-only in DB; stripped from GET /api/config (not in filterPublicSiteConfig allowlist).
		"vendorSubscriptions": map[string]interface{}{
			"plans": []interface{}{},
		},
	}
}

func (h *AdminHandler) GetConfiguration(c *gin.Context) {
	siteConfig, err := h.configService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Return the config map directly, not the full SiteConfig object
	// Frontend expects: {"general": {...}, "categories": {...}, ...}
	if siteConfig.Config == nil || len(siteConfig.Config) == 0 {
		// Return default config if database is empty
		c.JSON(http.StatusOK, getDefaultConfig())
		return
	}
	c.JSON(http.StatusOK, siteConfig.Config)
}

func (h *AdminHandler) UpdateConfiguration(c *gin.Context) {
	// Get existing config first to preserve ID
	existingConfig, err := h.configService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get existing configuration"})
		return
	}

	// Parse incoming JSON - frontend sends the config map directly
	var incomingData map[string]interface{}
	if err := c.ShouldBindJSON(&incomingData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use incoming data as config map (frontend sends the map directly)
	configMap := incomingData

	// Create SiteConfig with merged config (preserve existing ID)
	updatedConfig := &models.SiteConfig{
		ID:        existingConfig.ID,
		Config:    configMap,
		UpdatedAt: time.Now(),
	}

	if err := h.configService.Update(c.Request.Context(), updatedConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration updated successfully"})
}

// filterPublicSiteConfig strips admin-only keys (e.g. SMTP) from stored site config.
func filterPublicSiteConfig(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	allowed := map[string]bool{
		"general": true, "design": true, "homepage": true, "navigation": true,
		"categories": true, "footer": true, "localization": true, "pages": true,
		"seo": true, "social": true, "marketing": true, "analytics": true,
	}
	out := make(map[string]interface{})
	for k, v := range src {
		if allowed[k] {
			out[k] = v
		}
	}
	return out
}

// ensureRegionDefaults returns a config with the default general.regions block
// injected when missing, so deployments whose stored config predates region gating
// still gate correctly without an admin re-saving. Non-mutating: it shallow-copies
// only the maps it needs to touch and leaves the input untouched.
func ensureRegionDefaults(cfg map[string]interface{}) map[string]interface{} {
	if cfg == nil {
		return cfg
	}
	general, ok := cfg["general"].(map[string]interface{})
	if !ok {
		return cfg
	}
	if _, has := general["regions"]; has {
		return cfg
	}
	dg, ok := getDefaultConfig()["general"].(map[string]interface{})
	if !ok {
		return cfg
	}
	newGeneral := make(map[string]interface{}, len(general)+1)
	for k, v := range general {
		newGeneral[k] = v
	}
	newGeneral["regions"] = dg["regions"]

	out := make(map[string]interface{}, len(cfg))
	for k, v := range cfg {
		out[k] = v
	}
	out["general"] = newGeneral
	return out
}

// GetPublicConfig returns public site configuration (customer-facing data only)
// Uses the same logic as GetConfiguration but is public (no auth required)
func (h *AdminHandler) GetPublicConfig(c *gin.Context) {
	siteConfig, err := h.configService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if siteConfig.Config == nil || len(siteConfig.Config) == 0 {
		c.JSON(http.StatusOK, filterPublicSiteConfig(getDefaultConfig()))
		return
	}
	c.JSON(http.StatusOK, filterPublicSiteConfig(ensureRegionDefaults(siteConfig.Config)))
}

func (h *AdminHandler) GetPublicContent(c *gin.Context) {
	siteConfig, err := h.configService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get config map from database or use defaults
	var configMap map[string]interface{}
	if siteConfig.Config != nil && len(siteConfig.Config) > 0 {
		configMap = siteConfig.Config
	} else {
		configMap = getDefaultConfig()
	}

	// Return content from config map
	content := make(map[string]interface{})
	if homepage, ok := configMap["homepage"].(map[string]interface{}); ok {
		content["homepage"] = homepage
	}
	if pages, ok := configMap["pages"].(map[string]interface{}); ok {
		content["pages"] = pages
	}

	c.JSON(http.StatusOK, content)
}

// GetCohortAnalytics returns user retention cohort data grouped by signup month.
// GET /admin/analytics/cohort
func (h *AdminHandler) GetCohortAnalytics(c *gin.Context) {
	months := 6
	data, err := h.advancedAnalyticsRepo.GetCohortData(c.Request.Context(), months)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cohorts": data, "months": months})
}

// GetFunnelAnalytics returns add-to-cart → checkout → purchase funnel counts.
// GET /admin/analytics/funnel
func (h *AdminHandler) GetFunnelAnalytics(c *gin.Context) {
	endDate, startDate := parseDateRangeOrDays(c, 30)
	data, err := h.advancedAnalyticsRepo.GetFunnelData(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"funnel": data, "start": startDate, "end": endDate})
}

// GetAdminProductRecommendations returns top co-purchased product pairs for admin insight.
// GET /admin/analytics/recommendations
func (h *AdminHandler) GetAdminProductRecommendations(c *gin.Context) {
	pairs, err := h.advancedAnalyticsRepo.GetCoPurchasePairs(c.Request.Context(), 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pairs": pairs})
}
