package handlers

import (
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/middleware"
	"rloco-backend/internal/repositories"
)

// RewardsHandler exposes customer-facing reward summary derived from real order data.
type RewardsHandler struct {
	orderRepo repositories.OrderRepository
}

func NewRewardsHandler(orderRepo repositories.OrderRepository) *RewardsHandler {
	return &RewardsHandler{orderRepo: orderRepo}
}

// GetSummary returns order count, lifetime spend, and reward points (1 point per whole currency unit of lifetime spend, floor).
// GET /rewards/summary
func (h *RewardsHandler) GetSummary(c *gin.Context) {
	uid, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	n, spend, err := h.orderRepo.GetUserRewardStats(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	points := int64(math.Floor(spend))
	if points < 0 {
		points = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"order_count":     n,
		"lifetime_spend":  spend,
		"reward_points":   points,
		"points_rule":     "1 point per whole unit of order total (non-cancelled orders)",
	})
}
