package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/middleware"
	"rloco-backend/internal/repositories"
)

// NotificationHandler manages FCM device token registration.
type NotificationHandler struct {
	userRepo repositories.UserRepository
}

func NewNotificationHandler(userRepo repositories.UserRepository) *NotificationHandler {
	return &NotificationHandler{userRepo: userRepo}
}

type deviceTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform"` // "ios", "android", "web"
}

// RegisterDeviceToken adds an FCM token to the authenticated user's token list.
// POST /notifications/device-token
func (h *NotificationHandler) RegisterDeviceToken(c *gin.Context) {
	uid, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req deviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.userRepo.AddFCMToken(c.Request.Context(), uid, req.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Device token registered"})
}

type removeTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// RemoveDeviceToken removes an FCM token for the authenticated user.
// DELETE /notifications/device-token
func (h *NotificationHandler) RemoveDeviceToken(c *gin.Context) {
	uid, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req removeTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.userRepo.RemoveFCMToken(c.Request.Context(), uid, req.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Device token removed"})
}
