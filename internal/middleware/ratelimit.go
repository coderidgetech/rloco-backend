package middleware

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Simple in-memory rate limiter
type rateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int           // requests per minute
	window   time.Duration // time window
}

type visitor struct {
	count     int
	windowStart time.Time // When the current window started
}

var limiter *rateLimiter

func init() {
	// Check environment - be more lenient in development
	env := os.Getenv("ENV")
	rate := 1000 // Default: 1000 requests per minute (very lenient)
	if env == "production" {
		rate = 100 // Production: 100 requests per minute
	}
	
	limiter = &rateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   1 * time.Minute, // 1 minute window
	}
	// Clean up old visitors every 5 minutes
	go limiter.cleanup()
}

func (rl *rateLimiter) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.windowStart) > rl.window {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[ip]

	if !exists {
		rl.visitors[ip] = &visitor{
			count:       1,
			windowStart: now,
		}
		return true
	}

	// Reset if window has passed
	if now.Sub(v.windowStart) > rl.window {
		v.count = 1
		v.windowStart = now
		return true
	}

	// Check if limit exceeded
	if v.count >= rl.rate {
		return false
	}

	v.count++
	return true
}

// RateLimit returns a middleware that limits requests per IP
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}