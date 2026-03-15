package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/services"
)

type TaxHandler struct {
	taxService services.TaxService
}

func NewTaxHandler(taxService services.TaxService) *TaxHandler {
	return &TaxHandler{taxService: taxService}
}

func (h *TaxHandler) Calculate(c *gin.Context) {
	var req struct {
		Country   string  `json:"country" binding:"required"`
		State     *string `json:"state,omitempty"`
		City      *string `json:"city,omitempty"`
		PostalCode *string `json:"postal_code,omitempty"`
		Subtotal  float64 `json:"subtotal" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	state := ""
	if req.State != nil {
		state = *req.State
	}
	city := ""
	if req.City != nil {
		city = *req.City
	}
	postalCode := ""
	if req.PostalCode != nil {
		postalCode = *req.PostalCode
	}

	taxAmount, rate, err := h.taxService.CalculateTax(c.Request.Context(), req.Country, state, city, postalCode, req.Subtotal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tax_amount": taxAmount,
		"rate":       rate,
	})
}

func (h *TaxHandler) List(c *gin.Context) {
	activeOnly := c.DefaultQuery("active_only", "true") == "true"

	rates, err := h.taxService.GetRates(c.Request.Context(), activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rates)
}

func (h *TaxHandler) Create(c *gin.Context) {
	var rate models.TaxRate
	if err := c.ShouldBindJSON(&rate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.taxService.Create(c.Request.Context(), &rate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rate)
}

func (h *TaxHandler) Update(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tax rate ID"})
		return
	}

	var rate models.TaxRate
	if err := c.ShouldBindJSON(&rate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.taxService.Update(c.Request.Context(), id, &rate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tax rate updated successfully"})
}

func (h *TaxHandler) Delete(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tax rate ID"})
		return
	}

	if err := h.taxService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tax rate deleted successfully"})
}
