package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout creates a middleware that sets a timeout for request context
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}