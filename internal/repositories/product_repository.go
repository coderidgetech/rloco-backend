package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"rloco-backend/internal/models"
)

type ProductRepository interface {
	Create(ctx context.Context, product *models.Product) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Product, error)
	Update(ctx context.Context, id primitive.ObjectID, product *models.Product) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, filter bson.M, limit, skip int, sort bson.M) ([]*models.Product, int64, error)
	GetFeatured(ctx context.Context, limit int) ([]*models.Product, error)
	GetNewArrivals(ctx context.Context, limit int) ([]*models.Product, error)
	GetOnSale(ctx context.Context, limit int) ([]*models.Product, error)
	Search(ctx context.Context, query string, limit, skip int) ([]*models.Product, int64, error)
	// AtomicStockUpdate atomically decrements stock if available
	AtomicStockUpdate(ctx context.Context, productID primitive.ObjectID, size string, quantity int) error
}

type productRepository struct {
	collection *mongo.Collection
}

func NewProductRepository(db *MongoDB) ProductRepository {
	return &productRepository{
		collection: db.GetCollection("products"),
	}
}

func (r *productRepository) Create(ctx context.Context, product *models.Product) error {
	product.ID = primitive.NewObjectID()
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, product)
	return err
}

func (r *productRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Product, error) {
	var product models.Product
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&product)
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *productRepository) Update(ctx context.Context, id primitive.ObjectID, product *models.Product) error {
	product.UpdatedAt = time.Now()
	
	// Convert product to bson.M, excluding immutable fields
	updateDoc := bson.M{
		"name":               product.Name,
		"sku":                product.SKU,
		"price":              product.Price,
		"original_price":     product.OriginalPrice,
		"price_inr":          product.PriceINR,
		"original_price_inr": product.OriginalPriceINR,
		"images":             product.Images,
		"category":           product.Category,
		"subcategory":        product.Subcategory,
		"gender":             product.Gender,
		"colors":             product.Colors,
		"sizes":              product.Sizes,
		"description":        product.Description,
		"details":            product.Details,
		"material":           product.Material,
		"care":               product.Care,
		"featured":           product.Featured,
		"new_arrival":        product.NewArrival,
		"on_sale":            product.OnSale,
		"is_gift":            product.IsGift,
		"badge":              product.Badge,
		"video_url":          product.VideoURL,
		"stock":              product.Stock,
		"rating":             product.Rating,
		"reviews":            product.Reviews,
		"updated_at":         product.UpdatedAt,
	}
	
	// Only include vendor_id if it's set
	if product.VendorID != nil {
		updateDoc["vendor_id"] = product.VendorID
	}
	
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": updateDoc},
	)
	return err
}

func (r *productRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *productRepository) List(ctx context.Context, filter bson.M, limit, skip int, sort bson.M) ([]*models.Product, int64, error) {
	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip))
	if sort != nil {
		opts.SetSort(sort)
	}

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	products := []*models.Product{}
	if err := cursor.All(ctx, &products); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func (r *productRepository) GetFeatured(ctx context.Context, limit int) ([]*models.Product, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"featured": true},
		options.Find().SetLimit(int64(limit)).SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	products := []*models.Product{}
	if err := cursor.All(ctx, &products); err != nil {
		return nil, err
	}
	return products, nil
}

func (r *productRepository) GetNewArrivals(ctx context.Context, limit int) ([]*models.Product, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"new_arrival": true},
		options.Find().SetLimit(int64(limit)).SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return []*models.Product{}, err
	}
	defer cursor.Close(ctx)

	products := []*models.Product{}
	if err := cursor.All(ctx, &products); err != nil {
		return []*models.Product{}, err
	}
	return products, nil
}

func (r *productRepository) GetOnSale(ctx context.Context, limit int) ([]*models.Product, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"on_sale": true},
		options.Find().SetLimit(int64(limit)).SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	products := []*models.Product{}
	if err := cursor.All(ctx, &products); err != nil {
		return nil, err
	}
	return products, nil
}

func (r *productRepository) Search(ctx context.Context, query string, limit, skip int) ([]*models.Product, int64, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"description": bson.M{"$regex": query, "$options": "i"}},
			{"category": bson.M{"$regex": query, "$options": "i"}},
		},
	}

	return r.List(ctx, filter, limit, skip, bson.M{"created_at": -1})
}

// AtomicStockUpdate atomically decrements stock if available, preventing race conditions
func (r *productRepository) AtomicStockUpdate(ctx context.Context, productID primitive.ObjectID, size string, quantity int) error {
	filter := bson.M{
		"_id": productID,
		fmt.Sprintf("stock.%s", size): bson.M{"$gte": quantity},
	}
	update := bson.M{
		"$inc": bson.M{fmt.Sprintf("stock.%s", size): -quantity},
		"$set": bson.M{"updated_at": time.Now()},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return errors.New("insufficient stock")
	}
	return nil
}


