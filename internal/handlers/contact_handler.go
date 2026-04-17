package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/services"
)

type ContactHandler struct {
	email services.EmailService
}

func NewContactHandler(email services.EmailService) *ContactHandler {
	return &ContactHandler{email: email}
}

// Submit accepts public contact form submissions and emails operations (SMTP + ADMIN_EMAIL).
func (h *ContactHandler) Submit(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Email   string `json:"email" binding:"required,email"`
		Phone   string `json:"phone"`
		Subject string `json:"subject" binding:"required"`
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(strings.TrimSpace(req.Message)) < 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message is too short"})
		return
	}
	if err := h.email.SendContactInquiry(req.Name, req.Email, req.Phone, req.Subject, req.Message); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Unable to send message right now. Please try again later."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Message received. We'll get back to you soon."})
}
