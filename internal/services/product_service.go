package services

import (
	"context"
	"fmt"
	"regexp"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type ProductService interface {
	Create(ctx context.Context, product *models.Product) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Product, error)
	Update(ctx context.Context, id primitive.ObjectID, product *models.Product) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, filter map[string]interface{}, limit, skip int, sort string) ([]*models.Product, int64, error)
	GetFeatured(ctx context.Context, limit int, market string) ([]*models.Product, error)
	GetNewArrivals(ctx context.Context, limit int, market string) ([]*models.Product, error)
	GetOnSale(ctx context.Context, limit int, market string) ([]*models.Product, error)
	// Variant group methods
	GetVariants(ctx context.Context, productID primitive.ObjectID) ([]*models.Product, error)
	SetVariantGroup(ctx context.Context, productID primitive.ObjectID, groupID primitive.ObjectID, color string, isMain bool) error
	CreateVariantGroup(ctx context.Context, productID primitive.ObjectID, color string) (primitive.ObjectID, error)
	UnsetVariantGroup(ctx context.Context, productID primitive.ObjectID) error
	AutoSplitVariants(ctx context.Context) (int, int, error)
}

type productService struct {
	repo repositories.ProductRepository
}

func NewProductService(repo repositories.ProductRepository) ProductService {
	return &productService{repo: repo}
}

func (s *productService) Create(ctx context.Context, product *models.Product) error {
	return s.repo.Create(ctx, product)
}

func (s *productService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Product, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *productService) Update(ctx context.Context, id primitive.ObjectID, product *models.Product) error {
	return s.repo.Update(ctx, id, product)
}

func (s *productService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.Delete(ctx, id)
}

func (s *productService) List(ctx context.Context, filter map[string]interface{}, limit, skip int, sort string) ([]*models.Product, int64, error) {
	bsonFilter := bson.M{}
	
	// Convert filter map to bson.M
	if filter != nil {
		if category, ok := filter["category"].(string); ok && category != "" {
			// Case-insensitive category matching using regex
			bsonFilter["category"] = bson.M{"$regex": primitive.Regex{Pattern: "^" + category + "$", Options: "i"}}
		}
		if gender, ok := filter["gender"].(string); ok && gender != "" {
			// Case-insensitive gender matching using regex
			bsonFilter["gender"] = bson.M{"$regex": primitive.Regex{Pattern: "^" + gender + "$", Options: "i"}}
		}
		if onSale, ok := filter["on_sale"].(bool); ok {
			bsonFilter["on_sale"] = onSale
		}
		if featured, ok := filter["featured"].(bool); ok {
			bsonFilter["featured"] = featured
		}
		if newArrival, ok := filter["new_arrival"].(bool); ok {
			bsonFilter["new_arrival"] = newArrival
		}
		if isGift, ok := filter["is_gift"].(bool); ok {
			bsonFilter["is_gift"] = isGift
		}
		if search, ok := filter["search"].(string); ok && search != "" {
			pattern := ".*" + regexp.QuoteMeta(search) + ".*"
			re := primitive.Regex{Pattern: pattern, Options: "i"}
			// `gender` is matched EXACTLY (anchored), unlike the substring match used for
			// text fields. Substring matching on gender is broken: "men" is contained in
			// "women", so searching "Men" would return every women's product too. Exact
			// match means "Women" → women only, "Men" → men only, while still letting a
			// user search a gender word and get that catalog.
			genderRe := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(search) + "$", Options: "i"}
			bsonFilter["$or"] = []bson.M{
				{"name": bson.M{"$regex": re}},
				{"sku": bson.M{"$regex": re}},
				{"category": bson.M{"$regex": re}},
				{"subcategory": bson.M{"$regex": re}},
				{"description": bson.M{"$regex": re}},
				{"material": bson.M{"$regex": re}},
				{"gender": bson.M{"$regex": genderRe}},
			}
		}
		if minPrice, ok := filter["min_price"].(float64); ok {
			bsonFilter["price"] = bson.M{"$gte": minPrice}
		}
		if maxPrice, ok := filter["max_price"].(float64); ok {
			if _, exists := bsonFilter["price"]; exists {
				bsonFilter["price"].(bson.M)["$lte"] = maxPrice
			} else {
				bsonFilter["price"] = bson.M{"$lte": maxPrice}
			}
		}
		if vendorID, ok := filter["vendor_id"]; ok && vendorID != nil {
			bsonFilter["vendor_id"] = vendorID
		}
		// House / first-party catalog: products with no vendor (staff scope).
		// In Mongo {vendor_id: nil} matches both explicit null and a missing field.
		if houseOnly, ok := filter["house_only"].(bool); ok && houseOnly {
			bsonFilter["vendor_id"] = nil
		}
		// Hide draft products from the public storefront. {$ne:"draft"} keeps legacy
		// products (no status field) and active ones visible.
		if excl, ok := filter["exclude_draft"].(bool); ok && excl {
			bsonFilter["status"] = bson.M{"$ne": "draft"}
		}
	}

	if market, ok := filter["market"].(string); ok && (market == "IN" || market == "US") {
		bsonFilter = repositories.MergeMarketFilter(bsonFilter, market)
	}
	// deduplicate=true → only return is_main_variant=true (or products with no group)
	if dedupe, ok := filter["deduplicate"].(bool); ok && dedupe {
		bsonFilter = bson.M{"$and": []bson.M{
			bsonFilter,
			{"$or": []bson.M{
				{"variant_group_id": bson.M{"$exists": false}},
				{"is_main_variant": true},
			}},
		}}
	}

	sortBSON := bson.M{}
	if sort != "" {
		switch sort {
		case "price_asc":
			sortBSON["price"] = 1
		case "price_desc":
			sortBSON["price"] = -1
		case "newest":
			sortBSON["created_at"] = -1
		case "oldest":
			sortBSON["created_at"] = 1
		default:
			sortBSON["created_at"] = -1
		}
	}

	return s.repo.List(ctx, bsonFilter, limit, skip, sortBSON)
}

func (s *productService) GetFeatured(ctx context.Context, limit int, market string) ([]*models.Product, error) {
	return s.repo.GetFeatured(ctx, limit, market)
}

func (s *productService) GetNewArrivals(ctx context.Context, limit int, market string) ([]*models.Product, error) {
	return s.repo.GetNewArrivals(ctx, limit, market)
}

func (s *productService) GetOnSale(ctx context.Context, limit int, market string) ([]*models.Product, error) {
	return s.repo.GetOnSale(ctx, limit, market)
}

func (s *productService) GetVariants(ctx context.Context, productID primitive.ObjectID) ([]*models.Product, error) {
	product, err := s.repo.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.VariantGroupID == nil {
		// No group — return just this product as a single-element slice
		return []*models.Product{product}, nil
	}
	return s.repo.GetVariantsByGroupID(ctx, *product.VariantGroupID)
}

func (s *productService) SetVariantGroup(ctx context.Context, productID primitive.ObjectID, groupID primitive.ObjectID, color string, isMain bool) error {
	return s.repo.SetVariantGroup(ctx, productID, groupID, color, isMain)
}

// CreateVariantGroup creates a new variant group for a product (becomes the main variant).
func (s *productService) CreateVariantGroup(ctx context.Context, productID primitive.ObjectID, color string) (primitive.ObjectID, error) {
	groupID := primitive.NewObjectID()
	if err := s.repo.SetVariantGroup(ctx, productID, groupID, color, true); err != nil {
		return primitive.NilObjectID, err
	}
	return groupID, nil
}

func (s *productService) UnsetVariantGroup(ctx context.Context, productID primitive.ObjectID) error {
	return s.repo.UnsetVariantGroup(ctx, productID)
}

// AutoSplitVariants finds products with multiple colors and no variant_group_id,
// then creates one variant product per extra color (copying all fields).
// Returns (groupsCreated, variantsCreated, error).
func (s *productService) AutoSplitVariants(ctx context.Context) (int, int, error) {
	// Fetch all products that have >1 color and no variant_group_id yet
	filter := map[string]interface{}{}
	all, _, err := s.List(ctx, filter, 10000, 0, "newest")
	if err != nil {
		return 0, 0, err
	}

	groupsCreated := 0
	variantsCreated := 0

	for _, product := range all {
		// Skip products already in a group or with ≤1 color
		if product.VariantGroupID != nil || len(product.Colors) <= 1 {
			continue
		}

		groupID := primitive.NewObjectID()

		// Mark original product as main variant (first color)
		if err := s.repo.SetVariantGroup(ctx, product.ID, groupID, product.Colors[0], true); err != nil {
			return groupsCreated, variantsCreated, err
		}
		groupsCreated++

		// Create a new product record for each additional color
		newProducts := make([]*models.Product, 0, len(product.Colors)-1)
		for i, color := range product.Colors[1:] {
			clone := *product // shallow copy
			clone.ID = primitive.NewObjectID()
			clone.Color = color
			clone.Colors = []string{color}
			clone.IsMainVariant = false
			clone.VariantGroupID = &groupID
			clone.SKU = product.SKU + "-" + sanitizeSKU(color) + "-" + fmt.Sprintf("%d", i+2)
			newProducts = append(newProducts, &clone)
		}

		if err := s.repo.BulkInsert(ctx, newProducts); err != nil {
			return groupsCreated, variantsCreated, err
		}
		variantsCreated += len(newProducts)
	}

	return groupsCreated, variantsCreated, nil
}

func sanitizeSKU(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		}
	}
	if len(result) > 6 {
		result = result[:6]
	}
	return string(result)
}

