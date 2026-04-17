package handlers

import (
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"rloco-backend/internal/models"
	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProductHandler struct {
	productService services.ProductService
	storageService services.StorageService
}

func NewProductHandler(productService services.ProductService, storageService services.StorageService) *ProductHandler {
	return &ProductHandler{
		productService: productService,
		storageService: storageService,
	}
}

// checkProductOwnership verifies that a vendor owns the product
func (h *ProductHandler) checkProductOwnership(c *gin.Context, productID primitive.ObjectID) bool {
	role, exists := c.Get("role")
	if !exists {
		return false
	}
	if role == "admin" {
		return true // Admin can access all products
	}

	if role == "vendor" {
		// Get vendor ID from context
		vendorID, _ := c.Get("vendor_id")
		if vendorID == nil {
			return false
		}

		// Get product to check ownership
		product, err := h.productService.GetByID(c.Request.Context(), productID)
		if err != nil {
			return false
		}

		// Check if product belongs to vendor
		if product.VendorID == nil {
			return false // Product has no vendor (admin-only product)
		}

		vendorIDObj, ok := vendorID.(*primitive.ObjectID)
		if !ok {
			// Try string conversion
			vendorIDStr, ok := vendorID.(string)
			if ok {
				id, err := primitive.ObjectIDFromHex(vendorIDStr)
				if err == nil {
					vendorIDObj = &id
				}
			}
		}

		if vendorIDObj == nil {
			return false
		}

		return product.VendorID.Hex() == vendorIDObj.Hex()
	}

	return false
}

func (h *ProductHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	skip, _ := strconv.Atoi(c.DefaultQuery("skip", "0"))
	sort := c.DefaultQuery("sort", "newest")

	// Enforce pagination limits to prevent memory issues
	if limit > 100 {
		limit = 100 // Max limit
	}
	if limit < 1 {
		limit = 20 // Min limit
	}
	if skip < 0 {
		skip = 0
	}

	filter := make(map[string]interface{})

	// Vendor filtering: vendors can only see their own products
	role, exists := c.Get("role")
	if exists && role == "vendor" {
		vendorID, _ := c.Get("vendor_id")
		if vendorID != nil {
			vendorIDObj, ok := vendorID.(*primitive.ObjectID)
			if !ok {
				// Try string conversion
				vendorIDStr, ok := vendorID.(string)
				if ok {
					id, err := primitive.ObjectIDFromHex(vendorIDStr)
					if err == nil {
						vendorIDObj = &id
					}
				}
			}
			if vendorIDObj != nil {
				filter["vendor_id"] = vendorIDObj
			}
		}
	}

	if category := c.Query("category"); category != "" {
		filter["category"] = category
	}
	if gender := c.Query("gender"); gender != "" {
		filter["gender"] = gender
	}
	if onSale := c.Query("on_sale"); onSale == "true" {
		filter["on_sale"] = true
	}
	if featured := c.Query("featured"); featured == "true" {
		filter["featured"] = true
	}
	if newArrival := c.Query("new_arrival"); newArrival == "true" {
		filter["new_arrival"] = true
	}
	if gift := c.Query("gift"); gift == "true" || c.Query("is_gift") == "true" {
		filter["is_gift"] = true
	}
	if search := c.Query("search"); search != "" {
		filter["search"] = search
	}
	if minPrice := c.Query("min_price"); minPrice != "" {
		if price, err := strconv.ParseFloat(minPrice, 64); err == nil {
			filter["min_price"] = price
		}
	}
	if maxPrice := c.Query("max_price"); maxPrice != "" {
		if price, err := strconv.ParseFloat(maxPrice, 64); err == nil {
			filter["max_price"] = price
		}
	}
	if m := c.Query("market"); m == "IN" || m == "US" {
		filter["market"] = m
	}

	products, total, err := h.productService.List(c.Request.Context(), filter, limit, skip, sort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"products": products,
		"total":    total,
		"limit":    limit,
		"skip":     skip,
	})
}

func (h *ProductHandler) Get(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	product, err := h.productService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	if m := c.Query("market"); m == "IN" || m == "US" {
		if !productVisibleInMarket(product, m) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}
	}

	c.JSON(http.StatusOK, product)
}

func productVisibleInMarket(p *models.Product, market string) bool {
	if len(p.AvailableMarkets) == 0 {
		return true
	}
	for _, m := range p.AvailableMarkets {
		if m == market {
			return true
		}
	}
	return false
}

func (h *ProductHandler) Create(c *gin.Context) {
	var product struct {
		Name             string         `json:"name" binding:"required"`
		SKU              string         `json:"sku"`
		Price            float64        `json:"price" binding:"required"`
		OriginalPrice    *float64       `json:"original_price,omitempty"`
		PriceINR         *float64       `json:"price_inr,omitempty"`
		OriginalPriceINR *float64       `json:"original_price_inr,omitempty"`
		AvailableMarkets []string       `json:"available_markets"`
		Images           []string       `json:"images"`
		Category         string         `json:"category" binding:"required"`
		Subcategory      string         `json:"subcategory"`
		Gender           string         `json:"gender" binding:"required"`
		Colors           []string       `json:"colors"`
		Sizes            []string       `json:"sizes"`
		Description      string         `json:"description"`
		Details          []string       `json:"details"`
		Material         string         `json:"material"`
		Care             string         `json:"care"`
		Featured         bool           `json:"featured"`
		NewArrival       bool           `json:"new_arrival"`
		OnSale           bool           `json:"on_sale"`
		IsGift           bool           `json:"is_gift"`
		Stock            map[string]int `json:"stock"`
	}

	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to model
	productModel := &models.Product{
		Name:             product.Name,
		SKU:              product.SKU,
		Price:            product.Price,
		OriginalPrice:    product.OriginalPrice,
		PriceINR:         product.PriceINR,
		OriginalPriceINR: product.OriginalPriceINR,
		Images:           product.Images,
		Category:         product.Category,
		Subcategory:      product.Subcategory,
		Gender:           product.Gender,
		Colors:           product.Colors,
		Sizes:            product.Sizes,
		Description:      product.Description,
		Details:          product.Details,
		Material:         product.Material,
		Care:             product.Care,
		Featured:         product.Featured,
		NewArrival:       product.NewArrival,
		OnSale:           product.OnSale,
		IsGift:           product.IsGift,
		Stock:            product.Stock,
		Rating:           0,
		Reviews:          0,
	}
	if len(product.AvailableMarkets) > 0 {
		productModel.AvailableMarkets = product.AvailableMarkets
	} else {
		productModel.AvailableMarkets = []string{"IN", "US"}
	}

	// Set vendor_id if user is a vendor (not admin)
	role, exists := c.Get("role")
	if exists && role == "vendor" {
		vendorID, _ := c.Get("vendor_id")
		if vendorID != nil {
			vendorIDObj, ok := vendorID.(*primitive.ObjectID)
			if !ok {
				vendorIDStr, ok := vendorID.(string)
				if ok {
					id, err := primitive.ObjectIDFromHex(vendorIDStr)
					if err == nil {
						vendorIDObj = &id
					}
				}
			}
			if vendorIDObj != nil {
				productModel.VendorID = vendorIDObj
			}
		}
	}

	if err := h.productService.Create(c.Request.Context(), productModel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, productModel)
}

func (h *ProductHandler) Update(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	// Check ownership for vendors
	if !h.checkProductOwnership(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: You can only modify your own products"})
		return
	}

	// Get existing product first
	existingProduct, err := h.productService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Use map to allow partial updates
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply updates to existing product
	if newArrival, ok := updates["new_arrival"].(bool); ok {
		existingProduct.NewArrival = newArrival
	}
	if featured, ok := updates["featured"].(bool); ok {
		existingProduct.Featured = featured
	}
	if onSale, ok := updates["on_sale"].(bool); ok {
		existingProduct.OnSale = onSale
	}
	if isGift, ok := updates["is_gift"].(bool); ok {
		existingProduct.IsGift = isGift
	}
	if name, ok := updates["name"].(string); ok {
		existingProduct.Name = name
	}
	if price, ok := updates["price"].(float64); ok {
		existingProduct.Price = price
	}
	if sku, ok := updates["sku"].(string); ok {
		existingProduct.SKU = sku
	}
	if care, ok := updates["care"].(string); ok {
		existingProduct.Care = care
	}
	if subcategory, ok := updates["subcategory"].(string); ok {
		existingProduct.Subcategory = subcategory
	}
	if description, ok := updates["description"].(string); ok {
		existingProduct.Description = description
	}
	if material, ok := updates["material"].(string); ok {
		existingProduct.Material = material
	}
	if details, ok := updates["details"].([]interface{}); ok {
		strs := make([]string, 0, len(details))
		for _, v := range details {
			if s, ok := v.(string); ok {
				strs = append(strs, s)
			}
		}
		existingProduct.Details = strs
	}
	if images, ok := updates["images"].([]interface{}); ok {
		strs := make([]string, 0, len(images))
		for _, v := range images {
			if s, ok := v.(string); ok {
				strs = append(strs, s)
			}
		}
		existingProduct.Images = strs
	}
	if sizes, ok := updates["sizes"].([]interface{}); ok {
		strs := make([]string, 0, len(sizes))
		for _, v := range sizes {
			if s, ok := v.(string); ok {
				strs = append(strs, s)
			}
		}
		existingProduct.Sizes = strs
	}
	if colors, ok := updates["colors"].([]interface{}); ok {
		strs := make([]string, 0, len(colors))
		for _, v := range colors {
			if s, ok := v.(string); ok {
				strs = append(strs, s)
			}
		}
		existingProduct.Colors = strs
	}
	if stock, ok := updates["stock"].(map[string]interface{}); ok {
		m := make(map[string]int)
		for k, v := range stock {
			switch n := v.(type) {
			case float64:
				m[k] = int(n)
			case int:
				m[k] = n
			}
		}
		existingProduct.Stock = m
	}
	if originalPrice, ok := updates["original_price"].(float64); ok {
		existingProduct.OriginalPrice = &originalPrice
	}
	if raw, ok := updates["available_markets"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			strs := make([]string, 0, len(arr))
			for _, v := range arr {
				if s, ok := v.(string); ok && (s == "IN" || s == "US") {
					strs = append(strs, s)
				}
			}
			if len(strs) > 0 {
				existingProduct.AvailableMarkets = strs
			}
		}
	}

	// Ensure vendor_id cannot be changed by vendors
	role, exists := c.Get("role")
	if exists && role == "vendor" {
		// Vendor ID is already preserved from existingProduct
	}

	if err := h.productService.Update(c.Request.Context(), id, existingProduct); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return updated product
	updatedProduct, err := h.productService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Product updated successfully"})
		return
	}
	c.JSON(http.StatusOK, updatedProduct)
}

func (h *ProductHandler) Delete(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	if err := h.productService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
}

func (h *ProductHandler) GetFeatured(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 10
	}
	market := c.Query("market")
	products, err := h.productService.GetFeatured(c.Request.Context(), limit, market)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) GetNewArrivals(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 1000 {
		limit = 1000
	}
	if limit < 1 {
		limit = 10
	}
	market := c.Query("market")
	products, err := h.productService.GetNewArrivals(c.Request.Context(), limit, market)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Ensure we always return an array, not null
	if products == nil {
		products = []*models.Product{}
	}
	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) GetOnSale(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 10
	}
	market := c.Query("market")
	products, err := h.productService.GetOnSale(c.Request.Context(), limit, market)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) UploadImages(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	// Check ownership for vendors
	if !h.checkProductOwnership(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: You can only upload images for your own products"})
		return
	}

	product, err := h.productService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var files []*multipart.FileHeader
	if mf, err := c.MultipartForm(); err == nil && mf != nil {
		if v := mf.File["images"]; len(v) > 0 {
			files = v
		}
	}
	if len(files) == 0 {
		if fh, err := c.FormFile("file"); err == nil {
			files = []*multipart.FileHeader{fh}
		}
	}
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No images provided (use multipart field \"images\" or \"file\")"})
		return
	}

	allowedCT := map[string]bool{
		"image/jpeg": true, "image/jpg": true, "image/png": true, "image/webp": true, "image/gif": true,
	}
	const maxSize = 5 * 1024 * 1024
	urls := append([]string{}, product.Images...)

	for _, fh := range files {
		if fh.Size > maxSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Each image must be 5MB or smaller"})
			return
		}
		ct := strings.ToLower(strings.TrimSpace(fh.Header.Get("Content-Type")))
		if !allowedCT[ct] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image type"})
			return
		}
		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		url, err := h.storageService.Upload(c.Request.Context(), src, fh.Filename, fh.Header.Get("Content-Type"))
		_ = src.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		urls = append(urls, url)
	}

	product.Images = urls
	if err := h.productService.Update(c.Request.Context(), id, product); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Images uploaded", "images": urls})
}
