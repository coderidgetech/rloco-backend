package handlers

import (
	"net/http"
	"strconv"

	"rloco-backend/internal/models"
	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VideoHandler struct {
	videoService services.VideoService
}

func NewVideoHandler(videoService services.VideoService) *VideoHandler {
	return &VideoHandler{
		videoService: videoService,
	}
}

// List videos (public endpoint)
func (h *VideoHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))
	category := c.Query("category")
	featured := c.Query("featured")

	filter := make(map[string]interface{})
	if category != "" {
		filter["category"] = category
	}
	if featured == "true" {
		filter["featured"] = true
	}

	videos, total, err := h.videoService.List(c.Request.Context(), filter, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"total":  total,
		"limit":  limit,
		"skip":   skip,
	})
}

// Get video by ID (public endpoint)
func (h *VideoHandler) Get(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	video, err := h.videoService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	c.JSON(http.StatusOK, video)
}

// Get featured videos (public endpoint)
func (h *VideoHandler) GetFeatured(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	videos, err := h.videoService.GetFeatured(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, videos)
}

// Create video (admin/vendor only)
func (h *VideoHandler) Create(c *gin.Context) {
	var video models.InspirationVideo
	if err := c.ShouldBindJSON(&video); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID, exists := c.Get("user_id")
	if exists {
		if id, ok := userID.(primitive.ObjectID); ok {
			video.UploadedBy = &id
		}
	}

	userName, exists := c.Get("user_name")
	if exists {
		if name, ok := userName.(string); ok {
			video.UploadedByName = name
		}
	}

	if err := h.videoService.Create(c.Request.Context(), &video); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, video)
}

// Update video (admin/vendor only)
func (h *VideoHandler) Update(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	var video models.InspirationVideo
	if err := c.ShouldBindJSON(&video); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.videoService.Update(c.Request.Context(), id, &video); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updatedVideo, err := h.videoService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedVideo)
}

// Delete video (admin/vendor only)
func (h *VideoHandler) Delete(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid video ID"})
		return
	}

	if err := h.videoService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Video deleted successfully"})
}
