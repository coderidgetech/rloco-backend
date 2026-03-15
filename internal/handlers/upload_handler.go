package handlers

import (
	"net/http"
	"strings"

	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
)

const (
	MaxFileSize = 5 * 1024 * 1024 // 5MB
)

var AllowedFileTypes = []string{
	"image/jpeg",
	"image/jpg",
	"image/png",
	"image/webp",
	"image/gif",
}

type UploadHandler struct {
	storageService services.StorageService
}

func NewUploadHandler(storageService services.StorageService) *UploadHandler {
	return &UploadHandler{storageService: storageService}
}

func (h *UploadHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}

	// Validate file size
	if file.Size > MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size exceeds maximum limit of 5MB"})
		return
	}

	// Validate file type
	contentType := file.Header.Get("Content-Type")
	allowed := false
	for _, t := range AllowedFileTypes {
		if strings.EqualFold(contentType, t) {
			allowed = true
			break
		}
	}
	if !allowed {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid file type. Only images (JPEG, PNG, WebP, GIF) are allowed",
		})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	url, err := h.storageService.Upload(c.Request.Context(), src, file.Filename, file.Header.Get("Content-Type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *UploadHandler) Delete(c *gin.Context) {
	filename := c.Param("filename")
	if err := h.storageService.Delete(c.Request.Context(), filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}
