package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type VendorAnalyticsRepository interface {
	GetVendorRevenueSeries(ctx context.Context, vendorID primitive.ObjectID, start, end time.Time) ([]map[string]interface{}, error)
	GetVendorTopProducts(ctx context.Context, vendorID primitive.ObjectID, limit int) ([]map[string]interface{}, error)
	GetVendorOrderStats(ctx context.Context, vendorID primitive.ObjectID, start, end time.Time) (map[string]interface{}, error)
	GetVendorSummary(ctx context.Context, vendorID primitive.ObjectID) (map[string]interface{}, error)
}

type vendorAnalyticsRepository struct {
	orders   *mongo.Collection
	products *mongo.Collection
}

func NewVendorAnalyticsRepository(db *MongoDB) VendorAnalyticsRepository {
	return &vendorAnalyticsRepository{
		orders:   db.GetCollection("orders"),
		products: db.GetCollection("products"),
	}
}

// GetVendorRevenueSeries returns daily revenue for the vendor's products within [start, end].
func (r *vendorAnalyticsRepository) GetVendorRevenueSeries(ctx context.Context, vendorID primitive.ObjectID, start, end time.Time) ([]map[string]interface{}, error) {
	pipeline := mongo.Pipeline{
		// only non-cancelled orders in range
		{{Key: "$match", Value: bson.M{
			"status":     bson.M{"$ne": "cancelled"},
			"created_at": bson.M{"$gte": start, "$lte": end},
		}}},
		{{Key: "$unwind", Value: "$items"}},
		// join product to check vendor_id
		{{Key: "$lookup", Value: bson.M{
			"from":         "products",
			"localField":   "items.product_id",
			"foreignField": "_id",
			"as":           "product",
		}}},
		{{Key: "$unwind", Value: "$product"}},
		{{Key: "$match", Value: bson.M{"product.vendor_id": vendorID}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
				"day":   bson.M{"$dayOfMonth": "$created_at"},
			},
			"revenue": bson.M{"$sum": bson.M{"$multiply": bson.A{"$items.price", "$items.quantity"}}},
			"orders":  bson.M{"$addToSet": "$_id"},
		}}},
		{{Key: "$project", Value: bson.M{
			"date":        bson.M{"$dateFromParts": bson.M{"year": "$_id.year", "month": "$_id.month", "day": "$_id.day"}},
			"revenue":     1,
			"order_count": bson.M{"$size": "$orders"},
		}}},
		{{Key: "$sort", Value: bson.M{"date": 1}}},
	}
	return runPipeline(ctx, r.orders, pipeline)
}

// GetVendorTopProducts returns top products by revenue for this vendor.
func (r *vendorAnalyticsRepository) GetVendorTopProducts(ctx context.Context, vendorID primitive.ObjectID, limit int) ([]map[string]interface{}, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": bson.M{"$ne": "cancelled"}}}},
		{{Key: "$unwind", Value: "$items"}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "products",
			"localField":   "items.product_id",
			"foreignField": "_id",
			"as":           "product",
		}}},
		{{Key: "$unwind", Value: "$product"}},
		{{Key: "$match", Value: bson.M{"product.vendor_id": vendorID}}},
		{{Key: "$group", Value: bson.M{
			"_id":          "$items.product_id",
			"product_name": bson.M{"$first": "$items.product_name"},
			"revenue":      bson.M{"$sum": bson.M{"$multiply": bson.A{"$items.price", "$items.quantity"}}},
			"units_sold":   bson.M{"$sum": "$items.quantity"},
			"orders_count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"revenue": -1}}},
		{{Key: "$limit", Value: limit}},
	}
	return runPipeline(ctx, r.orders, pipeline)
}

// GetVendorOrderStats returns order status breakdown for vendor products.
func (r *vendorAnalyticsRepository) GetVendorOrderStats(ctx context.Context, vendorID primitive.ObjectID, start, end time.Time) (map[string]interface{}, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"created_at": bson.M{"$gte": start, "$lte": end},
		}}},
		{{Key: "$unwind", Value: "$items"}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "products",
			"localField":   "items.product_id",
			"foreignField": "_id",
			"as":           "product",
		}}},
		{{Key: "$unwind", Value: "$product"}},
		{{Key: "$match", Value: bson.M{"product.vendor_id": vendorID}}},
		{{Key: "$group", Value: bson.M{
			"_id":    "$_id",
			"status": bson.M{"$first": "$status"},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		}}},
	}
	rows, err := runPipeline(ctx, r.orders, pipeline)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{}
	for _, row := range rows {
		if s, ok := row["_id"].(string); ok {
			result[s] = row["count"]
		}
	}
	return result, nil
}

// GetVendorSummary returns lifetime totals for the vendor.
func (r *vendorAnalyticsRepository) GetVendorSummary(ctx context.Context, vendorID primitive.ObjectID) (map[string]interface{}, error) {
	// product count
	productCount, err := r.products.CountDocuments(ctx, bson.M{"vendor_id": vendorID})
	if err != nil {
		return nil, err
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": bson.M{"$ne": "cancelled"}}}},
		{{Key: "$unwind", Value: "$items"}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "products",
			"localField":   "items.product_id",
			"foreignField": "_id",
			"as":           "product",
		}}},
		{{Key: "$unwind", Value: "$product"}},
		{{Key: "$match", Value: bson.M{"product.vendor_id": vendorID}}},
		{{Key: "$group", Value: bson.M{
			"_id":              nil,
			"total_revenue":    bson.M{"$sum": bson.M{"$multiply": bson.A{"$items.price", "$items.quantity"}}},
			"total_orders":     bson.M{"$addToSet": "$_id"},
			"total_customers":  bson.M{"$addToSet": "$user_id"},
		}}},
		{{Key: "$project", Value: bson.M{
			"total_revenue":   1,
			"total_orders":    bson.M{"$size": "$total_orders"},
			"total_customers": bson.M{"$size": "$total_customers"},
		}}},
	}
	rows, err := runPipeline(ctx, r.orders, pipeline)
	if err != nil {
		return nil, err
	}
	summary := map[string]interface{}{
		"total_revenue":    0,
		"total_orders":     0,
		"total_customers":  0,
		"total_products":   productCount,
	}
	if len(rows) > 0 {
		for k, v := range rows[0] {
			summary[k] = v
		}
		summary["total_products"] = productCount
	}
	return summary, nil
}

// runPipeline is a helper to execute an aggregation and decode results.
func runPipeline(ctx context.Context, coll *mongo.Collection, pipeline mongo.Pipeline) ([]map[string]interface{}, error) {
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}
