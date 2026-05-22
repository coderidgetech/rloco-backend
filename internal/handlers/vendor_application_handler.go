package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/services"
)

type VendorApplicationHandler struct {
	svc services.VendorApplicationService
}

func NewVendorApplicationHandler(svc services.VendorApplicationService) *VendorApplicationHandler {
	return &VendorApplicationHandler{svc: svc}
}

// Submit handles POST /api/vendor/apply  (public, no auth)
func (h *VendorApplicationHandler) Submit(c *gin.Context) {
	var app models.VendorApplication
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app.BusinessName = strings.TrimSpace(app.BusinessName)
	app.ContactName = strings.TrimSpace(app.ContactName)
	app.Email = strings.ToLower(strings.TrimSpace(app.Email))
	app.Phone = strings.TrimSpace(app.Phone)

	if app.BusinessName == "" || app.ContactName == "" || app.Email == "" ||
		app.Phone == "" || app.Category == "" || app.City == "" || app.State == "" || app.Country == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Required fields missing"})
		return
	}

	if err := h.svc.Submit(c.Request.Context(), &app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit application"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Application submitted successfully"})
}

// List handles GET /api/admin/vendor-applications  (admin only)
func (h *VendorApplicationHandler) List(c *gin.Context) {
	status := c.Query("status") // pending | approved | rejected | "" (all)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	apps, total, err := h.svc.List(c.Request.Context(), status, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch applications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"applications": apps,
		"total":        total,
		"limit":        limit,
		"skip":         skip,
	})
}

// GetOne handles GET /api/admin/vendor-applications/:id  (admin only)
func (h *VendorApplicationHandler) GetOne(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	app, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}
	c.JSON(http.StatusOK, app)
}

// Approve handles POST /api/admin/vendor-applications/:id/approve  (admin only)
func (h *VendorApplicationHandler) Approve(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var body struct {
		AdminNotes string `json:"admin_notes"`
	}
	c.ShouldBindJSON(&body)

	if err := h.svc.Approve(c.Request.Context(), id, body.AdminNotes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Application approved"})
}

// Reject handles POST /api/admin/vendor-applications/:id/reject  (admin only)
func (h *VendorApplicationHandler) Reject(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&body)

	if err := h.svc.Reject(c.Request.Context(), id, body.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Application rejected"})
}
