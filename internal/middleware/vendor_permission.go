package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/models"
)

// RequireVendorPermission enforces a vendor's granular permission flag for an
// action. It only gates vendors — admin and other roles pass through (admin is a
// superuser; non-vendors are handled by RequireRole). It is backward-compatible:
// a vendor with no permissions blob configured keeps full vendor access, and a
// flag is only treated as a denial when an admin has explicitly set it to false.
//
// Requires LoadUserMiddleware to have run first (it loads the vendor into context).
func RequireVendorPermission(category, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "vendor" {
			c.Next()
			return
		}

		v, ok := c.Get("vendor")
		if !ok || v == nil {
			c.Next() // vendor record unavailable → don't block (fail open, status/ownership still apply)
			return
		}
		vendor, ok := v.(*models.Vendor)
		if !ok || len(vendor.Permissions) == 0 {
			c.Next() // no permissions configured → full vendor access
			return
		}

		if !vendorPermissionAllows(vendor.Permissions, category, action) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("Your account does not have permission to %s %s", action, category),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// vendorPermissionAllows navigates the nested permissions blob
// (e.g. permissions["products"]["delete"]). It returns false ONLY when the flag
// is explicitly set to false; missing categories/keys default to allowed so a
// partial blob never locks a vendor out of a capability the admin didn't revoke.
func vendorPermissionAllows(perms map[string]interface{}, category, action string) bool {
	cat, ok := perms[category].(map[string]interface{})
	if !ok {
		return true
	}
	val, ok := cat[action]
	if !ok {
		return true
	}
	allowed, ok := val.(bool)
	if !ok {
		return true
	}
	return allowed
}
