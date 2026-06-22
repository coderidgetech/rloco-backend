package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
	"rloco-backend/internal/services"
)

type ShippingHandler struct {
	shippingService services.ShippingService
	productRepo     repositories.ProductRepository
}

func NewShippingHandler(shippingService services.ShippingService, productRepo repositories.ProductRepository) *ShippingHandler {
	return &ShippingHandler{shippingService: shippingService, productRepo: productRepo}
}

func (h *ShippingHandler) Calculate(c *gin.Context) {
	var req struct {
		Country    string   `json:"country" binding:"required"`
		State      string   `json:"state,omitempty"`
		City       string   `json:"city,omitempty"`
		Address    string   `json:"address,omitempty"`
		PostalCode string   `json:"postal_code,omitempty"`
		FirstName  string   `json:"first_name,omitempty"`
		LastName   string   `json:"last_name,omitempty"`
		Email      string   `json:"email,omitempty"`
		Phone      string   `json:"phone,omitempty"`
		Subtotal   float64  `json:"subtotal"` // no "required": Go's validator treats 0 as missing, and 0 is a valid subtotal
		Weight     *float64 `json:"weight,omitempty"`
		Items      []struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		} `json:"items,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// When the client sends cart items, compute authoritative weight + parcel
	// dimensions from the catalog (same rule as order fulfillment: max L, max W,
	// summed H, real per-unit weight) so the checkout estimate matches the rate
	// the order will actually be charged.
	var lenCm, widCm, heiCm *float64
	if len(req.Items) > 0 {
		var weightLb, maxL, maxW, sumH float64
		for _, it := range req.Items {
			if it.Quantity <= 0 {
				continue
			}
			oid, oidErr := primitive.ObjectIDFromHex(it.ProductID)
			if oidErr != nil {
				continue
			}
			p, perr := h.productRepo.GetByID(c.Request.Context(), oid)
			if perr != nil || p == nil {
				continue
			}
			unitLb := services.DefaultItemWeightLb
			if p.Weight != nil && *p.Weight > 0 {
				unitLb = *p.Weight * 2.20462 // model weight is kg
			}
			weightLb += unitLb * float64(it.Quantity)
			if p.LengthCm != nil && *p.LengthCm > maxL {
				maxL = *p.LengthCm
			}
			if p.WidthCm != nil && *p.WidthCm > maxW {
				maxW = *p.WidthCm
			}
			if p.HeightCm != nil && *p.HeightCm > 0 {
				sumH += *p.HeightCm * float64(it.Quantity)
			}
		}
		if weightLb > 0 {
			req.Weight = &weightLb
		}
		if maxL > 0 {
			lenCm = &maxL
		}
		if maxW > 0 {
			widCm = &maxW
		}
		if sumH > 0 {
			heiCm = &sumH
		}
	}

	methods, err := h.shippingService.CalculateShipping(c.Request.Context(), services.ShippingQuoteRequest{
		Country:    req.Country,
		State:      req.State,
		City:       req.City,
		Address:    req.Address,
		PostalCode: req.PostalCode,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Email:      req.Email,
		Phone:      req.Phone,
		Subtotal:   req.Subtotal,
		Weight:     req.Weight,
		LengthCm:   lenCm,
		WidthCm:    widCm,
		HeightCm:   heiCm,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"methods": methods})
}

func (h *ShippingHandler) List(c *gin.Context) {
	activeOnly := c.DefaultQuery("active_only", "true") == "true"

	methods, err := h.shippingService.GetMethods(c.Request.Context(), activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, methods)
}

func (h *ShippingHandler) Create(c *gin.Context) {
	var method models.ShippingMethod
	if err := c.ShouldBindJSON(&method); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.shippingService.Create(c.Request.Context(), &method); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, method)
}

func (h *ShippingHandler) Update(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid shipping method ID"})
		return
	}

	method, err := h.shippingService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shipping method not found"})
		return
	}
	if err := c.ShouldBindJSON(method); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	method.ID = id

	if err := h.shippingService.Update(c.Request.Context(), id, method); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Shipping method updated successfully"})
}

func (h *ShippingHandler) Delete(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid shipping method ID"})
		return
	}

	if err := h.shippingService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Shipping method deleted successfully"})
}
