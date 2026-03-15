package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/services"
)

type ShippingHandler struct {
	shippingService services.ShippingService
}

func NewShippingHandler(shippingService services.ShippingService) *ShippingHandler {
	return &ShippingHandler{shippingService: shippingService}
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
		Subtotal   float64  `json:"subtotal" binding:"required"`
		Weight     *float64 `json:"weight,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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

	var method models.ShippingMethod
	if err := c.ShouldBindJSON(&method); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.shippingService.Update(c.Request.Context(), id, &method); err != nil {
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
