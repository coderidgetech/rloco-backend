package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Stricter limiter for checkout (order create + payment intent) to reduce abuse at scale.
type checkoutLimiter struct {
	visitors map[string]*checkoutVisitor
	mu       sync.RWMutex
	rate     int
	window   time.Duration
}

type checkoutVisitor struct {
	count       int
	windowStart time.Time
}

var checkoutLimit *checkoutLimiter

func init() {
	checkoutLimit = &checkoutLimiter{
		visitors: make(map[string]*checkoutVisitor),
		rate:     60,
		window:   1 * time.Minute,
	}
	go checkoutLimit.cleanup()
}

func (rl *checkoutLimiter) cleanup() {
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

func (rl *checkoutLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &checkoutVisitor{count: 1, windowStart: now}
		return true
	}
	if now.Sub(v.windowStart) > rl.window {
		v.count = 1
		v.windowStart = now
		return true
	}
	if v.count >= rl.rate {
		return false
	}
	v.count++
	return true
}

// CheckoutRateLimit limits POST /orders and payment intent calls per IP (default 60/min).
func CheckoutRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !checkoutLimit.allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many checkout attempts. Please wait a minute and try again.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
