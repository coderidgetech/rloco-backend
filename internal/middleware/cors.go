package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSAllowedOrigins is set by main. In production set CORS_ALLOWED_ORIGINS to comma-separated list; empty = allow request Origin (dev).
var CORSAllowedOrigins string

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowedOrigin := "*"
		if CORSAllowedOrigins != "" {
			allowedOrigin = ""
			for _, o := range strings.Split(strings.TrimSpace(CORSAllowedOrigins), ",") {
				if strings.TrimSpace(o) == origin {
					allowedOrigin = origin
					break
				}
			}
		} else if origin != "" {
			allowedOrigin = origin
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		// Include Idempotency-Key so checkout/order POST preflight succeeds (browser sends custom header).
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Idempotency-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

