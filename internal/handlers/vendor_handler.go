package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/middleware"
	"rloco-backend/internal/repositories"
	"rloco-backend/internal/services"
)

type VendorHandler struct {
	vendorService      services.VendorService
	vendorAnalyticsRepo repositories.VendorAnalyticsRepository
}

func NewVendorHandler(vendorService services.VendorService, vendorAnalyticsRepo repositories.VendorAnalyticsRepository) *VendorHandler {
	return &VendorHandler{vendorService: vendorService, vendorAnalyticsRepo: vendorAnalyticsRepo}
}

// GetMe returns the vendor profile for the authenticated vendor user.
func (h *VendorHandler) GetMe(c *gin.Context) {
	vid := middleware.GetVendorIDFromContext(c)
	if vid == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No vendor profile for this account"})
		return
	}
	v, err := h.vendorService.GetByID(c.Request.Context(), *vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor not found"})
		return
	}
	c.JSON(http.StatusOK, v)
}

type vendorMeUpdate struct {
	Name         *string                `json:"name"`
	Email        *string                `json:"email"`
	Logo         *string                `json:"logo"`
	Preferences  map[string]interface{} `json:"preferences"`
}

// UpdateMe allows a vendor to update safe fields on their own vendor record.
func (h *VendorHandler) UpdateMe(c *gin.Context) {
	vid := middleware.GetVendorIDFromContext(c)
	if vid == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No vendor profile for this account"})
		return
	}
	current, err := h.vendorService.GetByID(c.Request.Context(), *vid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vendor not found"})
		return
	}

	var body vendorMeUpdate
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.Name != nil {
		current.Name = *body.Name
	}
	if body.Email != nil {
		current.Email = *body.Email
	}
	if body.Logo != nil {
		current.Logo = *body.Logo
	}
	if body.Preferences != nil {
		if current.Preferences == nil {
			current.Preferences = make(map[string]interface{})
		}
		for k, v := range body.Preferences {
			current.Preferences[k] = v
		}
	}

	if err := h.vendorService.Update(c.Request.Context(), *vid, current); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, current)
}

func vendorDateRange(c *gin.Context) (time.Time, time.Time) {
	days := 30
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	return start, end
}

// GetAnalyticsSummary returns lifetime totals for the authenticated vendor.
// GET /vendor/analytics/summary
func (h *VendorHandler) GetAnalyticsSummary(c *gin.Context) {
	vid := middleware.GetVendorIDFromContext(c)
	if vid == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No vendor profile for this account"})
		return
	}
	summary, err := h.vendorAnalyticsRepo.GetVendorSummary(c.Request.Context(), *vid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetAnalyticsRevenue returns daily revenue series for the vendor.
// GET /vendor/analytics/revenue
func (h *VendorHandler) GetAnalyticsRevenue(c *gin.Context) {
	vid := middleware.GetVendorIDFromContext(c)
	if vid == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No vendor profile for this account"})
		return
	}
	start, end := vendorDateRange(c)
	data, err := h.vendorAnalyticsRepo.GetVendorRevenueSeries(c.Request.Context(), *vid, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "start": start, "end": end})
}

// GetAnalyticsProducts returns top products for the vendor.
// GET /vendor/analytics/products
func (h *VendorHandler) GetAnalyticsProducts(c *gin.Context) {
	vid := middleware.GetVendorIDFromContext(c)
	if vid == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No vendor profile for this account"})
		return
	}
	data, err := h.vendorAnalyticsRepo.GetVendorTopProducts(c.Request.Context(), *vid, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// GetAnalyticsOrders returns order status breakdown for the vendor.
// GET /vendor/analytics/orders
func (h *VendorHandler) GetAnalyticsOrders(c *gin.Context) {
	vid := middleware.GetVendorIDFromContext(c)
	if vid == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No vendor profile for this account"})
		return
	}
	start, end := vendorDateRange(c)
	data, err := h.vendorAnalyticsRepo.GetVendorOrderStats(c.Request.Context(), *vid, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
