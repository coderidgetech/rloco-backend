package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	sanitizeAPIErrors bool
	errorCfgMu        sync.RWMutex
)

// ConfigureErrorResponses hides raw internal errors in production responses.
func ConfigureErrorResponses(isProduction bool) {
	errorCfgMu.Lock()
	sanitizeAPIErrors = isProduction
	errorCfgMu.Unlock()
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			errorCfgMu.RLock()
			san := sanitizeAPIErrors
			errorCfgMu.RUnlock()
			msg := err.Error()
			if san {
				msg = "internal server error"
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": msg,
			})
		}
	}
}

