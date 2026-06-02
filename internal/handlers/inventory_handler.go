package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/services"
)

type InventoryHandler struct {
	inventoryService services.InventoryService
}

func NewInventoryHandler(inventoryService services.InventoryService) *InventoryHandler {
	return &InventoryHandler{inventoryService: inventoryService}
}

func (h *InventoryHandler) GetLowStock(c *gin.Context) {
	threshold, _ := strconv.Atoi(c.DefaultQuery("threshold", "10"))

	items, err := h.inventoryService.CheckLowStock(c.Request.Context(), threshold)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *InventoryHandler) GetAlerts(c *gin.Context) {
	threshold, _ := strconv.Atoi(c.DefaultQuery("threshold", "10"))

	alerts, err := h.inventoryService.GetStockAlerts(c.Request.Context(), threshold)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, alerts)
}
