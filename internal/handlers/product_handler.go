package handlers

import (
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
	"rloco-backend/internal/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProductHandler struct {
	productService      services.ProductService
	storageService      services.StorageService
	advancedAnalyticsRepo repositories.AdvancedAnalyticsRepository
}

func NewProductHandler(productService services.ProductService, storageService services.StorageService, advancedAnalyticsRepo repositories.AdvancedAnalyticsRepository) *ProductHandler {
	return &ProductHandler{
		productService:      productService,
		storageService:      storageService,
		advancedAnalyticsRepo: advancedAnalyticsRepo,
	}
}

// checkProductOwnership verifies that a vendor owns the product
// vendorObjectIDFromContext resolves the authenticated vendor's id from the gin
// context (set by LoadUserMiddleware from the persisted user, not the JWT).
func vendorObjectIDFromContext(c *gin.Context) *primitive.ObjectID {
	v, _ := c.Get("vendor_id")
	if v == nil {
		return nil
	}
	if id, ok := v.(*primitive.ObjectID); ok {
		return id
	}
	if s, ok := v.(string); ok {
		if id, err := primitive.ObjectIDFromHex(s); err == nil {
			return &id
		}
	}
	return nil
}

func (h *ProductHandler) checkProductOwnership(c *gin.Context, productID primitive.ObjectID) bool {
	role, exists := c.Get("role")
	if !exists {
		return false
	}
	if role == "admin" {
		return true // Admin can access all products
	}

	if role == "staff" {
		// Staff own the house / first-party catalog only (products with no vendor).
		product, err := h.productService.GetByID(c.Request.Context(), productID)
		if err != nil {
			return false
		}
		return product.VendorID == nil
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
	// Staff manage only the house / first-party catalog (products with no vendor).
	if exists && role == "staff" {
		filter["house_only"] = true
	}
	// Public (unauthenticated / customer) requests never see draft products.
	if !(exists && (role == "admin" || role == "vendor" || role == "staff")) {
		filter["exclude_draft"] = true
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
	if c.Query("deduplicate") == "true" {
		filter["deduplicate"] = true
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
		Badge            *string        `json:"badge,omitempty"`
		VideoURL         *string        `json:"video_url,omitempty"`
		Status           string         `json:"status"`
		Brand            string         `json:"brand"`
		Barcode          string         `json:"barcode"`
		CountryOfOrigin  string         `json:"country_of_origin"`
		Tags             []string       `json:"tags"`
		CostPrice        *float64       `json:"cost_price,omitempty"`
		Weight           *float64       `json:"weight,omitempty"`
		LengthCm         *float64       `json:"length_cm,omitempty"`
		WidthCm          *float64       `json:"width_cm,omitempty"`
		HeightCm         *float64       `json:"height_cm,omitempty"`
		HSNCode          string         `json:"hsn_code"`
		TaxCode          string         `json:"tax_code"`
		GSTPercent       *float64       `json:"gst_percent,omitempty"`
		MetaTitle        string         `json:"meta_title"`
		MetaDescription  string         `json:"meta_description"`
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
		Badge:            product.Badge,
		VideoURL:         product.VideoURL,
		Status:           product.Status,
		Brand:            product.Brand,
		Barcode:          product.Barcode,
		CountryOfOrigin:  product.CountryOfOrigin,
		Tags:             product.Tags,
		CostPrice:        product.CostPrice,
		Weight:           product.Weight,
		LengthCm:         product.LengthCm,
		WidthCm:          product.WidthCm,
		HeightCm:         product.HeightCm,
		HSNCode:          product.HSNCode,
		TaxCode:          product.TaxCode,
		GSTPercent:       product.GSTPercent,
		MetaTitle:        product.MetaTitle,
		MetaDescription:  product.MetaDescription,
		Rating:           0,
		Reviews:          0,
	}
	if len(product.AvailableMarkets) > 0 {
		productModel.AvailableMarkets = product.AvailableMarkets
	} else {
		productModel.AvailableMarkets = []string{"IN", "US"}
	}

	// Stamp vendor_id for vendor-created products. A vendor MUST have a resolved
	// vendor id — otherwise we'd create an ownerless product that's live in the
	// catalog yet invisible to the vendor's own list/ownership checks.
	role, exists := c.Get("role")
	if exists && role == "vendor" {
		vendorIDObj := vendorObjectIDFromContext(c)
		if vendorIDObj == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Your vendor account is not fully set up; products cannot be created"})
			return
		}
		productModel.VendorID = vendorIDObj
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

	// Newly-captured product fields
	if v, ok := updates["status"].(string); ok {
		existingProduct.Status = v
	}
	if v, ok := updates["brand"].(string); ok {
		existingProduct.Brand = v
	}
	if v, ok := updates["barcode"].(string); ok {
		existingProduct.Barcode = v
	}
	if v, ok := updates["country_of_origin"].(string); ok {
		existingProduct.CountryOfOrigin = v
	}
	if v, ok := updates["hsn_code"].(string); ok {
		existingProduct.HSNCode = v
	}
	if v, ok := updates["tax_code"].(string); ok {
		existingProduct.TaxCode = v
	}
	if v, ok := updates["meta_title"].(string); ok {
		existingProduct.MetaTitle = v
	}
	if v, ok := updates["meta_description"].(string); ok {
		existingProduct.MetaDescription = v
	}
	if v, ok := updates["gender"].(string); ok && v != "" {
		existingProduct.Gender = v
	}
	if v, ok := updates["category"].(string); ok && v != "" {
		existingProduct.Category = v
	}
	if v, ok := updates["badge"].(string); ok {
		if v == "" {
			existingProduct.Badge = nil
		} else {
			b := v
			existingProduct.Badge = &b
		}
	}
	if v, ok := updates["video_url"].(string); ok {
		if v == "" {
			existingProduct.VideoURL = nil
		} else {
			u := v
			existingProduct.VideoURL = &u
		}
	}
	if raw, ok := updates["tags"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			tags := make([]string, 0, len(arr))
			for _, v := range arr {
				if s, ok := v.(string); ok {
					tags = append(tags, s)
				}
			}
			existingProduct.Tags = tags
		}
	}
	setFloatPtr := func(key string, dst **float64) {
		if raw, ok := updates[key]; ok {
			if f, ok := raw.(float64); ok {
				val := f
				*dst = &val
			} else if raw == nil {
				*dst = nil
			}
		}
	}
	setFloatPtr("cost_price", &existingProduct.CostPrice)
	setFloatPtr("weight", &existingProduct.Weight)
	setFloatPtr("length_cm", &existingProduct.LengthCm)
	setFloatPtr("width_cm", &existingProduct.WidthCm)
	setFloatPtr("height_cm", &existingProduct.HeightCm)
	setFloatPtr("gst_percent", &existingProduct.GSTPercent)
	setFloatPtr("price_inr", &existingProduct.PriceINR)
	setFloatPtr("original_price_inr", &existingProduct.OriginalPriceINR)

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

	// Enforce ownership in the handler, not just at the route: a vendor may only
	// delete their own product (admin passes through checkProductOwnership).
	if !h.checkProductOwnership(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to delete this product"})
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
	if parseErr := c.Request.ParseMultipartForm(32 << 20); parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form: " + parseErr.Error()})
		return
	}
	if mf := c.Request.MultipartForm; mf != nil {
		// Accept any file field — covers "images", "file", or whatever the client sends
		for _, fhs := range mf.File {
			files = append(files, fhs...)
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


// GetRecommendations returns products frequently bought alongside this product.
// GET /products/:id/recommendations
func (h *ProductHandler) GetRecommendations(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	recommendedIDs, err := h.advancedAnalyticsRepo.GetRecommendedForProduct(c.Request.Context(), id, 8)
	if err != nil || len(recommendedIDs) == 0 {
		// Fallback: return same-category products
		product, ferr := h.productService.GetByID(c.Request.Context(), id)
		if ferr != nil {
			c.JSON(http.StatusOK, gin.H{"products": []interface{}{}})
			return
		}
		filter := map[string]interface{}{"category": product.Category}
		products, _, ferr := h.productService.List(c.Request.Context(), filter, 8, 0, "newest")
		if ferr != nil {
			c.JSON(http.StatusOK, gin.H{"products": []interface{}{}})
			return
		}
		// exclude the requested product
		var out []*models.Product
		for _, p := range products {
			if p.ID != id {
				out = append(out, p)
			}
		}
		c.JSON(http.StatusOK, gin.H{"products": out})
		return
	}

	var products []*models.Product
	for _, pid := range recommendedIDs {
		p, ferr := h.productService.GetByID(c.Request.Context(), pid)
		if ferr == nil {
			products = append(products, p)
		}
	}
	c.JSON(http.StatusOK, gin.H{"products": products})
}

// GetVariants returns all color variants sharing the same variant_group_id as the given product.
func (h *ProductHandler) GetVariants(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	variants, err := h.productService.GetVariants(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"variants": variants})
}

// SetVariantGroup assigns a product to a variant group (admin/vendor only).
// Body: { "variant_group_id": "<hex>", "color": "Navy", "is_main_variant": false }
// Omit variant_group_id to create a new group.
func (h *ProductHandler) SetVariantGroup(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	if !h.checkProductOwnership(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	var body struct {
		VariantGroupID string `json:"variant_group_id"`
		Color          string `json:"color"`
		IsMainVariant  bool   `json:"is_main_variant"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Color == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "color is required"})
		return
	}

	ctx := c.Request.Context()

	if body.VariantGroupID == "" {
		// Create a new group with this product as the main variant
		groupID, err := h.productService.CreateVariantGroup(ctx, id, body.Color)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"variant_group_id": groupID.Hex()})
		return
	}

	groupID, err := primitive.ObjectIDFromHex(body.VariantGroupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid variant_group_id"})
		return
	}
	if err := h.productService.SetVariantGroup(ctx, id, groupID, body.Color, body.IsMainVariant); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// UnsetVariantGroup removes a product from its variant group (admin/vendor only).
func (h *ProductHandler) UnsetVariantGroup(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	if !h.checkProductOwnership(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}
	if err := h.productService.UnsetVariantGroup(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// AutoSplitVariants runs the migration that splits multi-color products into variant siblings.
// Admin only.
func (h *ProductHandler) AutoSplitVariants(c *gin.Context) {
	groups, variants, err := h.productService.AutoSplitVariants(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"groups_created":   groups,
		"variants_created": variants,
	})
}
