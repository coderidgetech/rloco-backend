package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/repositories"
)

// vendorRepoForStatus is an optional repo used to enforce vendor suspension.
// Set once at startup via SetVendorRepoForStatusCheck; if nil, the check is skipped.
var vendorRepoForStatus repositories.VendorRepository

// SetVendorRepoForStatusCheck wires the vendor repo so LoadUserMiddleware can block
// suspended vendors on every request (not just login). Optional.
func SetVendorRepoForStatusCheck(r repositories.VendorRepository) {
	vendorRepoForStatus = r
}

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
			// Enforce suspension: a suspended vendor is blocked everywhere they would
			// act as a vendor (covers live sessions, not just login).
			if vendorRepoForStatus != nil {
				if v, verr := vendorRepoForStatus.GetByID(c.Request.Context(), *user.VendorID); verr == nil && v != nil && v.Status == "suspended" {
					c.JSON(http.StatusForbidden, gin.H{"error": "Your vendor account is suspended. Please contact support."})
					c.Abort()
					return
				}
			}
		}

		// Set user object for handlers that need it
		c.Set("user_obj", user)

		c.Next()
	}
}
