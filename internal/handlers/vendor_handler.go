package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/middleware"
	"rloco-backend/internal/services"
)

type VendorHandler struct {
	vendorService services.VendorService
}

func NewVendorHandler(vendorService services.VendorService) *VendorHandler {
	return &VendorHandler{vendorService: vendorService}
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
