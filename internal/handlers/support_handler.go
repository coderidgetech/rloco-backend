package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/services"
)

type SupportHandler struct {
	supportService services.SupportService
}

func NewSupportHandler(supportService services.SupportService) *SupportHandler {
	return &SupportHandler{supportService: supportService}
}

func (h *SupportHandler) Create(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		OrderID   *string `json:"order_id,omitempty"`
		Subject   string  `json:"subject" binding:"required"`
		Category  string  `json:"category" binding:"required"`
		Priority  string  `json:"priority" binding:"required"`
		Message   string  `json:"message" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var orderID *primitive.ObjectID
	if req.OrderID != nil {
		id, err := primitive.ObjectIDFromHex(*req.OrderID)
		if err == nil {
			orderID = &id
		}
	}

	ticket, err := h.supportService.CreateTicket(c.Request.Context(), userIDObj, orderID, req.Subject, req.Category, req.Priority, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ticket)
}

func (h *SupportHandler) Get(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	ticket, err := h.supportService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Check ownership
	role, _ := c.Get("role")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, _ := primitive.ObjectIDFromHex(userIDStr)

	if role != "admin" && ticket.UserID.Hex() != userIDObj.Hex() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, ticket)
}

func (h *SupportHandler) List(c *gin.Context) {
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

	role, _ := c.Get("role")
	if role == "admin" {
		// Admin can see all tickets
		filter := make(map[string]interface{})
		if status := c.Query("status"); status != "" {
			filter["status"] = status
		}
		if category := c.Query("category"); category != "" {
			filter["category"] = category
		}
		if priority := c.Query("priority"); priority != "" {
			filter["priority"] = priority
		}

		tickets, total, err := h.supportService.List(c.Request.Context(), filter, limit, skip)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"tickets": tickets,
			"total":   total,
			"limit":   limit,
			"skip":    skip,
		})
		return
	}

	// Regular users see only their tickets
	tickets, total, err := h.supportService.GetByUserID(c.Request.Context(), userIDObj, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tickets": tickets,
		"total":   total,
		"limit":   limit,
		"skip":    skip,
	})
}

func (h *SupportHandler) AddMessage(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	role, _ := c.Get("role")
	isAdmin := role == "admin"

	var req struct {
		Message     string   `json:"message" binding:"required"`
		Attachments []string `json:"attachments,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.supportService.AddMessage(c.Request.Context(), id, userIDObj, req.Message, isAdmin, req.Attachments); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message added successfully"})
}

func (h *SupportHandler) UpdateStatus(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.supportService.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ticket status updated successfully"})
}

func (h *SupportHandler) Assign(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var req struct {
		AssignedTo string `json:"assigned_to" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	assignedTo, err := primitive.ObjectIDFromHex(req.AssignedTo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid assigned_to ID"})
		return
	}

	if err := h.supportService.Assign(c.Request.Context(), id, assignedTo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ticket assigned successfully"})
}
