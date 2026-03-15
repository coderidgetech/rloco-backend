package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/repositories"
)

// LoadUserMiddleware loads full user data including vendor_id into context
// This should be used after AuthRequired to enrich context with user data
func LoadUserMiddleware(userRepo repositories.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			c.Abort()
			return
		}

		// Load user from repository to get vendor_id
		user, err := userRepo.GetByID(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// Set vendor_id in context if user has one
		if user.VendorID != nil {
			c.Set("vendor_id", user.VendorID)
		}

		// Set user object for handlers that need it
		c.Set("user_obj", user)

		c.Next()
	}
}
