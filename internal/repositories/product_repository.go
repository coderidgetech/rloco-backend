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
	GetFeatured(ctx context.Context, limit int, market string) ([]*models.Product, error)
	GetNewArrivals(ctx context.Context, limit int, market string) ([]*models.Product, error)
	GetOnSale(ctx context.Context, limit int, market string) ([]*models.Product, error)
	Search(ctx context.Context, query string, limit, skip int, market string) ([]*models.Product, int64, error)
	// AtomicStockUpdate atomically decrements stock if available
	AtomicStockUpdate(ctx context.Context, productID primitive.ObjectID, size string, quantity int) error
	// AtomicStockIncrement restores stock (e.g. order cancellation)
	AtomicStockIncrement(ctx context.Context, productID primitive.ObjectID, size string, quantity int) error
	// Variant group methods
	GetVariantsByGroupID(ctx context.Context, groupID primitive.ObjectID) ([]*models.Product, error)
	SetVariantGroup(ctx context.Context, productID primitive.ObjectID, groupID primitive.ObjectID, color string, isMain bool) error
	UnsetVariantGroup(ctx context.Context, productID primitive.ObjectID) error
	BulkInsert(ctx context.Context, products []*models.Product) error
	// LowStockItems returns one row per product×size whose stock is ≤ threshold,
	// filtered at the DB level via an aggregation pipeline.
	LowStockItems(ctx context.Context, threshold int) ([]models.LowStockItem, error)
}

type productRepository struct {
	collection *mongo.Collection
}

func NewProductRepository(db *MongoDB) ProductRepository {
	return &productRepository{
		collection: db.GetCollection("products"),
	}
}

// MergeMarketFilter ANDs base with catalog market visibility when market is IN or US.
// Products with missing or empty available_markets remain visible in all markets until backfilled.
func MergeMarketFilter(base bson.M, market string) bson.M {
	if market != "IN" && market != "US" {
		return base
	}
	mf := bson.M{"$or": []bson.M{
		{"available_markets": market},
		{"available_markets": bson.M{"$exists": false}},
		{"available_markets": bson.M{"$size": 0}},
	}}
	if len(base) == 0 {
		return mf
	}
	return bson.M{"$and": []bson.M{base, mf}}
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
		"reviews":             product.Reviews,
		"available_markets":   product.AvailableMarkets,
		"updated_at":          product.UpdatedAt,
	}
	
	if product.VendorID != nil {
		updateDoc["vendor_id"] = product.VendorID
	}
	if product.VariantGroupID != nil {
		updateDoc["variant_group_id"] = product.VariantGroupID
		updateDoc["color"] = product.Color
		updateDoc["is_main_variant"] = product.IsMainVariant
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

func (r *productRepository) GetFeatured(ctx context.Context, limit int, market string) ([]*models.Product, error) {
	filter := MergeMarketFilter(bson.M{
		"featured": true,
		"$or":      []bson.M{{"variant_group_id": nil}, {"is_main_variant": true}},
	}, market)
	cursor, err := r.collection.Find(
		ctx,
		filter,
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

func (r *productRepository) GetNewArrivals(ctx context.Context, limit int, market string) ([]*models.Product, error) {
	filter := MergeMarketFilter(bson.M{
		"new_arrival": true,
		"$or":         []bson.M{{"variant_group_id": nil}, {"is_main_variant": true}},
	}, market)
	cursor, err := r.collection.Find(
		ctx,
		filter,
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

func (r *productRepository) GetOnSale(ctx context.Context, limit int, market string) ([]*models.Product, error) {
	filter := MergeMarketFilter(bson.M{"on_sale": true}, market)
	cursor, err := r.collection.Find(
		ctx,
		filter,
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

func (r *productRepository) Search(ctx context.Context, query string, limit, skip int, market string) ([]*models.Product, int64, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"name": bson.M{"$regex": query, "$options": "i"}},
			{"sku": bson.M{"$regex": query, "$options": "i"}},
			{"description": bson.M{"$regex": query, "$options": "i"}},
			{"category": bson.M{"$regex": query, "$options": "i"}},
			{"subcategory": bson.M{"$regex": query, "$options": "i"}},
			{"material": bson.M{"$regex": query, "$options": "i"}},
		},
	}
	filter = MergeMarketFilter(filter, market)

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

func (r *productRepository) AtomicStockIncrement(ctx context.Context, productID primitive.ObjectID, size string, quantity int) error {
	if quantity <= 0 {
		return nil
	}
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": productID},
		bson.M{
			"$inc": bson.M{fmt.Sprintf("stock.%s", size): quantity},
			"$set": bson.M{"updated_at": time.Now()},
		},
	)
	return err
}

func (r *productRepository) GetVariantsByGroupID(ctx context.Context, groupID primitive.ObjectID) ([]*models.Product, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"variant_group_id": groupID},
		options.Find().SetSort(bson.M{"is_main_variant": -1, "color": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var products []*models.Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, err
	}
	return products, nil
}

func (r *productRepository) SetVariantGroup(ctx context.Context, productID primitive.ObjectID, groupID primitive.ObjectID, color string, isMain bool) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": productID},
		bson.M{"$set": bson.M{
			"variant_group_id": groupID,
			"color":            color,
			"is_main_variant":  isMain,
			"updated_at":       time.Now(),
		}},
	)
	return err
}

func (r *productRepository) UnsetVariantGroup(ctx context.Context, productID primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": productID},
		bson.M{
			"$unset": bson.M{"variant_group_id": "", "color": "", "is_main_variant": ""},
			"$set":   bson.M{"updated_at": time.Now()},
		},
	)
	return err
}

func (r *productRepository) LowStockItems(ctx context.Context, threshold int) ([]models.LowStockItem, error) {
	pipeline := mongo.Pipeline{
		// Explode the stock map into an array of {k: size, v: qty} pairs.
		{{Key: "$addFields", Value: bson.D{
			{Key: "stockArray", Value: bson.D{
				{Key: "$objectToArray", Value: "$stock"},
			}},
		}}},
		// One document per size.
		{{Key: "$unwind", Value: "$stockArray"}},
		// Keep only sizes whose qty ≤ threshold.
		{{Key: "$match", Value: bson.D{
			{Key: "stockArray.v", Value: bson.D{{Key: "$lte", Value: threshold}}},
		}}},
		// Project the fields the service/handler need.
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "product_id", Value: "$_id"},
			{Key: "name", Value: 1},
			{Key: "sku", Value: 1},
			{Key: "size", Value: "$stockArray.k"},
			{Key: "stock", Value: "$stockArray.v"},
			{Key: "price", Value: 1},
			{Key: "price_inr", Value: 1},
			{Key: "category", Value: 1},
			{Key: "image", Value: bson.D{
				{Key: "$arrayElemAt", Value: bson.A{"$images", 0}},
			}},
		}}},
		// Sort cheapest-stock first so the worst offenders appear at the top.
		{{Key: "$sort", Value: bson.D{{Key: "stock", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.LowStockItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *productRepository) BulkInsert(ctx context.Context, products []*models.Product) error {
	if len(products) == 0 {
		return nil
	}
	docs := make([]interface{}, len(products))
	for i, p := range products {
		p.ID = primitive.NewObjectID()
		p.CreatedAt = time.Now()
		p.UpdatedAt = time.Now()
		docs[i] = p
	}
	_, err := r.collection.InsertMany(ctx, docs)
	return err
}
