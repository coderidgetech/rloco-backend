package handlers

import (
	"net/http"

	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
)

type PromotionHandler struct {
	promotionService services.PromotionService
}

func NewPromotionHandler(promotionService services.PromotionService) *PromotionHandler {
	return &PromotionHandler{promotionService: promotionService}
}

func (h *PromotionHandler) List(c *gin.Context) {
	activeOnly := c.Query("active_only") == "true"
	promotions, err := h.promotionService.List(c.Request.Context(), activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, promotions)
}

func (h *PromotionHandler) Validate(c *gin.Context) {
	var req struct {
		Code     string  `json:"code" binding:"required"`
		Subtotal float64 `json:"subtotal"` // no "required": Go's validator treats 0 as missing, and 0 is a valid subtotal
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	promotion, discount, err := h.promotionService.Validate(c.Request.Context(), req.Code, req.Subtotal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":     true,
		"promotion": promotion,
		"discount":  discount,
	})
}
