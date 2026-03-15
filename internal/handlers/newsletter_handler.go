package handlers

import (
	"net/http"

	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
)

type NewsletterHandler struct {
	newsletterService services.NewsletterService
}

func NewNewsletterHandler(newsletterService services.NewsletterService) *NewsletterHandler {
	return &NewsletterHandler{newsletterService: newsletterService}
}

func (h *NewsletterHandler) Subscribe(c *gin.Context) {
	var req struct {
		Email string  `json:"email" binding:"required,email"`
		Name  *string `json:"name,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.newsletterService.Subscribe(c.Request.Context(), req.Email, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully subscribed to newsletter"})
}

func (h *NewsletterHandler) Unsubscribe(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.newsletterService.Unsubscribe(c.Request.Context(), req.Email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully unsubscribed from newsletter"})
}
