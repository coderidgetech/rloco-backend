package handlers

import (
	"net/http"
	"strconv"

	"rloco-backend/internal/models"
	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ReturnHandler struct {
	returnService services.ReturnService
}

func NewReturnHandler(returnService services.ReturnService) *ReturnHandler {
	return &ReturnHandler{returnService: returnService}
}

func (h *ReturnHandler) Create(c *gin.Context) {
	orderIDStr := c.Param("id")
	orderID, err := primitive.ObjectIDFromHex(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Items       []models.ReturnItem `json:"items" binding:"required"`
		Reason      string              `json:"reason" binding:"required"`
		Description string              `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	returnReq, err := h.returnService.Create(c.Request.Context(), orderID, userIDObj, req.Items, req.Reason, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, returnReq)
}

func (h *ReturnHandler) Get(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	returnReq, err := h.returnService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Return request not found"})
		return
	}

	// Check ownership
	role, _ := c.Get("role")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, _ := primitive.ObjectIDFromHex(userIDStr)

	if role != "admin" && returnReq.UserID.Hex() != userIDObj.Hex() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, returnReq)
}

func (h *ReturnHandler) List(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

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

	returns, total, err := h.returnService.GetByUserID(c.Request.Context(), userIDObj, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"returns": returns,
		"total":   total,
		"limit":   limit,
		"skip":    skip,
	})
}

func (h *ReturnHandler) Approve(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	if err := h.returnService.Approve(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return request approved"})
}

func (h *ReturnHandler) Reject(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.returnService.Reject(c.Request.Context(), id, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Return request rejected"})
}

func (h *ReturnHandler) ProcessRefund(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	var req struct {
		RefundMethod string `json:"refund_method" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.returnService.ProcessRefund(c.Request.Context(), id, req.RefundMethod); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Refund processed successfully"})
}

// UpdateStatus dispatches to Approve or Reject based on the status field.
// This bridges the frontend contract (PUT /admin/returns/:id/status)
// to the backend's specific approve/reject methods.
func (h *ReturnHandler) UpdateStatus(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid return ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch req.Status {
	case "approved":
		if err := h.returnService.Approve(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Return request approved"})
	case "rejected":
		if err := h.returnService.Reject(c.Request.Context(), id, "Admin decision"); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Return request rejected"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status value. Use 'approved' or 'rejected'"})
	}
}

func (h *ReturnHandler) ListAll(c *gin.Context) {
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

	filter := make(map[string]interface{})
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}
	if refundStatus := c.Query("refund_status"); refundStatus != "" {
		filter["refund_status"] = refundStatus
	}

	returns, total, err := h.returnService.List(c.Request.Context(), filter, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"returns": returns,
		"total":   total,
		"limit":   limit,
		"skip":    skip,
	})
}
