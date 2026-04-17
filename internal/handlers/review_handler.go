package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
	"rloco-backend/internal/services"
)

type ReviewHandler struct {
	reviewService services.ReviewService
	productRepo   repositories.ProductRepository
}

func NewReviewHandler(reviewService services.ReviewService, productRepo repositories.ProductRepository) *ReviewHandler {
	return &ReviewHandler{reviewService: reviewService, productRepo: productRepo}
}

// GetByUser returns the current user's reviews with product name and image.
func (h *ReviewHandler) GetByUser(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if skip < 0 {
		skip = 0
	}
	reviews, total, err := h.reviewService.GetByUserID(c.Request.Context(), userIDObj, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	out := make([]map[string]interface{}, 0, len(reviews))
	for _, r := range reviews {
		productName := ""
		productImage := ""
		prod, _ := h.productRepo.GetByID(ctx, r.ProductID)
		if prod != nil {
			productName = prod.Name
			if len(prod.Images) > 0 {
				productImage = prod.Images[0]
			}
		}
		item := reviewToMap(r)
		item["product_name"] = productName
		item["product_image"] = productImage
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{
		"reviews": out,
		"total":   total,
		"limit":   limit,
		"skip":    skip,
	})
}

func reviewToMap(r *models.ProductReview) map[string]interface{} {
	m := map[string]interface{}{
		"id":         r.ID.Hex(),
		"product_id": r.ProductID.Hex(),
		"user_id":    r.UserID.Hex(),
		"user_name":  r.UserName,
		"rating":     r.Rating,
		"title":      r.Title,
		"comment":    r.Comment,
		"images":     r.Images,
		"verified":   r.Verified,
		"helpful":    r.Helpful,
		"status":     r.Status,
		"created_at": r.CreatedAt,
		"updated_at": r.UpdatedAt,
	}
	return m
}

func (h *ReviewHandler) Create(c *gin.Context) {
	productIDStr := c.Param("id")
	productID, err := primitive.ObjectIDFromHex(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	userName, _ := c.Get("email")
	userNameStr, _ := userName.(string)

	var req struct {
		Rating   int      `json:"rating" binding:"required,min=1,max=5"`
		Title    string   `json:"title" binding:"required"`
		Comment  string   `json:"comment" binding:"required"`
		Images   []string `json:"images,omitempty"`
		Verified bool     `json:"verified"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	review, err := h.reviewService.Create(
		c.Request.Context(),
		productID,
		userIDObj,
		userNameStr,
		req.Rating,
		req.Title,
		req.Comment,
		req.Images,
		req.Verified,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, review)
}

func (h *ReviewHandler) GetByProduct(c *gin.Context) {
	productIDStr := c.Param("id")
	productID, err := primitive.ObjectIDFromHex(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))

	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if skip < 0 {
		skip = 0
	}

	reviews, total, err := h.reviewService.GetByProductID(c.Request.Context(), productID, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reviews": reviews,
		"total":   total,
		"limit":   limit,
		"skip":    skip,
	})
}

func (h *ReviewHandler) Update(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid review ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Title   string   `json:"title" binding:"required"`
		Comment string   `json:"comment" binding:"required"`
		Images  []string `json:"images,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	review, err := h.reviewService.Update(c.Request.Context(), id, userIDObj, req.Title, req.Comment, req.Images)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, review)
}

func (h *ReviewHandler) Delete(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid review ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	userIDObj, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.reviewService.Delete(c.Request.Context(), id, userIDObj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Review deleted successfully"})
}

func (h *ReviewHandler) MarkHelpful(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("reviewId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid review ID"})
		return
	}

	if err := h.reviewService.IncrementHelpful(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Review marked as helpful"})
}

func (h *ReviewHandler) UpdateStatus(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid review ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.reviewService.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Review status updated successfully"})
}

func (h *ReviewHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))
	status := c.DefaultQuery("status", "approved")

	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 20
	}
	if skip < 0 {
		skip = 0
	}

	reviews, total, err := h.reviewService.ListByStatus(c.Request.Context(), status, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if reviews == nil {
		reviews = []*models.ProductReview{}
	}
	c.JSON(http.StatusOK, gin.H{
		"reviews": reviews,
		"total":   total,
		"limit":   limit,
		"skip":    skip,
		"status":  status,
	})
}
