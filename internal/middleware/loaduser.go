package middleware

import (
	"net/http"
	"strings"

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

		// Force a vendor issued a temporary password to set a new one before any
		// write. Reads are allowed so they can navigate to the password screen, and
		// the password-change endpoint itself is exempt.
		if user.MustResetPassword && c.Request.Method != http.MethodGet && !strings.HasSuffix(c.FullPath(), "/auth/password") {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "Please set a new password before continuing.",
				"must_reset_password": true,
			})
			c.Abort()
			return
		}

		// Set vendor_id in context if user has one
		if user.VendorID != nil {
			c.Set("vendor_id", user.VendorID)
			// Load the vendor once: expose it for permission checks and enforce
			// suspension (blocked everywhere, covering live sessions not just login).
			if vendorRepoForStatus != nil {
				if v, verr := vendorRepoForStatus.GetByID(c.Request.Context(), *user.VendorID); verr == nil && v != nil {
					c.Set("vendor", v)
					if v.Status == "suspended" {
						c.JSON(http.StatusForbidden, gin.H{"error": "Your vendor account is suspended. Please contact support."})
						c.Abort()
						return
					}
				}
			}
		}

		// Set user object for handlers that need it
		c.Set("user_obj", user)

		c.Next()
	}
}
