package handlers

import (
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/middleware"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

// RewardsHandler exposes customer-facing reward summary, redemption, and history.
type RewardsHandler struct {
	orderRepo   repositories.OrderRepository
	rewardsRepo repositories.RewardsRepository
}

func NewRewardsHandler(orderRepo repositories.OrderRepository, rewardsRepo repositories.RewardsRepository) *RewardsHandler {
	return &RewardsHandler{orderRepo: orderRepo, rewardsRepo: rewardsRepo}
}

// GetSummary returns order count, lifetime spend, reward points balance, and exchange rate.
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
	// Earned points are the floor of lifetime spend (1 pt / $1)
	earned := int64(math.Floor(spend))
	if earned < 0 {
		earned = 0
	}
	// Redeemed points from the ledger
	balance, err := h.rewardsRepo.GetUserBalance(c.Request.Context(), uid)
	if err != nil {
		// Fall back to showing earned as balance if ledger is unavailable
		balance = earned
	}
	c.JSON(http.StatusOK, gin.H{
		"order_count":     n,
		"lifetime_spend":  spend,
		"reward_points":   earned,
		"balance":         balance,
		"points_value_usd": float64(balance) / 100, // 100 pts = $1
		"points_rule":     "1 point per $1 of order total (non-cancelled orders). 100 points = $1 discount.",
	})
}

// RedeemPoints redeems reward points for a discount credit.
// POST /rewards/redeem
func (h *RewardsHandler) RedeemPoints(c *gin.Context) {
	uid, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Points int64 `json:"points" binding:"required,min=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimum redemption is 100 points"})
		return
	}

	balance, err := h.rewardsRepo.GetUserBalance(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch balance"})
		return
	}
	if req.Points > balance {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient points balance"})
		return
	}

	tx := &models.RewardsTransaction{
		UserID:      uid,
		Type:        "redeemed",
		Points:      req.Points,
		Reference:   "manual_redemption",
		Description: "Points redeemed for discount",
		CreatedAt:   time.Now(),
	}
	if err := h.rewardsRepo.AddTransaction(c.Request.Context(), tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to redeem points"})
		return
	}

	discountUSD := float64(req.Points) / 100
	newBalance := balance - req.Points
	c.JSON(http.StatusOK, gin.H{
		"redeemed_points": req.Points,
		"discount_usd":    discountUSD,
		"new_balance":     newBalance,
		"message":         "Points redeemed successfully",
	})
}

// GetTransactions returns the rewards transaction history for the authenticated user.
// GET /rewards/transactions
func (h *RewardsHandler) GetTransactions(c *gin.Context) {
	uid, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	limit := 20
	skip := 0

	txs, total, err := h.rewardsRepo.GetUserTransactions(c.Request.Context(), uid, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": txs,
		"total":        total,
		"limit":        limit,
		"skip":         skip,
	})
}

