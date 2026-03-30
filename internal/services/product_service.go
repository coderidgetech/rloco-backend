package services

import (
	"context"
	"errors"
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
	Search(ctx context.Context, query string, limit, skip int, market string) ([]*models.Product, int64, error)
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
			bsonFilter["$or"] = []bson.M{
				{"name": bson.M{"$regex": re}},
				{"sku": bson.M{"$regex": re}},
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
	}

	if market, ok := filter["market"].(string); ok && (market == "IN" || market == "US") {
		bsonFilter = repositories.MergeMarketFilter(bsonFilter, market)
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

func (s *productService) Search(ctx context.Context, query string, limit, skip int, market string) ([]*models.Product, int64, error) {
	if query == "" {
		return nil, 0, errors.New("search query cannot be empty")
	}
	return s.repo.Search(ctx, query, limit, skip, market)
}

