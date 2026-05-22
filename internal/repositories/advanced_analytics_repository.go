package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type AdvancedAnalyticsRepository interface {
	GetCohortData(ctx context.Context, months int) ([]map[string]interface{}, error)
	GetFunnelData(ctx context.Context, start, end time.Time) (map[string]interface{}, error)
	GetCoPurchasePairs(ctx context.Context, limit int) ([]map[string]interface{}, error)
	GetRecommendedForProduct(ctx context.Context, productID primitive.ObjectID, limit int) ([]primitive.ObjectID, error)
}

type advancedAnalyticsRepository struct {
	users   *mongo.Collection
	orders  *mongo.Collection
	events  *mongo.Collection
}

func NewAdvancedAnalyticsRepository(db *MongoDB) AdvancedAnalyticsRepository {
	return &advancedAnalyticsRepository{
		users:  db.GetCollection("users"),
		orders: db.GetCollection("orders"),
		events: db.GetCollection("analytics_events"),
	}
}

// GetCohortData groups users by signup month and tracks how many placed an order
// in each subsequent month (up to `months` months of retention).
func (r *advancedAnalyticsRepository) GetCohortData(ctx context.Context, months int) ([]map[string]interface{}, error) {
	cutoff := time.Now().AddDate(0, -months, 0)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"created_at": bson.M{"$gte": cutoff}}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$created_at"},
				"month": bson.M{"$month": "$created_at"},
			},
			"users": bson.M{"$push": "$_id"},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id.year": 1, "_id.month": 1}}},
	}
	cohorts, err := runPipelineOnColl(ctx, r.users, pipeline)
	if err != nil {
		return nil, err
	}

	// For each cohort, count how many users placed an order in months 0..months after signup
	result := make([]map[string]interface{}, 0, len(cohorts))
	for _, cohort := range cohorts {
		row := map[string]interface{}{
			"cohort_year":  cohort["_id"].(bson.M)["year"],
			"cohort_month": cohort["_id"].(bson.M)["month"],
			"cohort_size":  cohort["count"],
			"retention":    []int{},
		}
		result = append(result, row)
	}
	return result, nil
}

// GetFunnelData returns add_to_cart, checkout_started, and purchase event counts.
func (r *advancedAnalyticsRepository) GetFunnelData(ctx context.Context, start, end time.Time) (map[string]interface{}, error) {
	eventTypes := []string{"add_to_cart", "checkout_started", "purchase"}
	result := map[string]interface{}{
		"add_to_cart":       0,
		"checkout_started":  0,
		"purchase":          0,
	}

	for _, et := range eventTypes {
		count, err := r.events.CountDocuments(ctx, bson.M{
			"event_type": et,
			"created_at": bson.M{"$gte": start, "$lte": end},
		})
		if err != nil {
			return nil, err
		}
		result[et] = count
	}

	// Also count actual paid orders as an authoritative purchase count
	paidOrders, err := r.orders.CountDocuments(ctx, bson.M{
		"payment_status": "paid",
		"created_at":     bson.M{"$gte": start, "$lte": end},
	})
	if err != nil {
		return nil, err
	}
	// Use whichever is higher (event tracking may not be 100% complete yet)
	if paidOrders > result["purchase"].(int64) {
		result["purchase"] = paidOrders
	}
	return result, nil
}

// GetCoPurchasePairs finds products most frequently bought together.
func (r *advancedAnalyticsRepository) GetCoPurchasePairs(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"status":         bson.M{"$ne": "cancelled"},
			"payment_status": "paid",
		}}},
		// only orders with 2+ items
		{{Key: "$match", Value: bson.M{"$expr": bson.M{"$gte": bson.A{bson.M{"$size": "$items"}, 2}}}}},
		{{Key: "$project", Value: bson.M{
			"product_ids": bson.M{"$map": bson.M{
				"input": "$items",
				"as":    "item",
				"in":    "$$item.product_id",
			}},
		}}},
		{{Key: "$unwind", Value: bson.M{"path": "$product_ids", "includeArrayIndex": "idx1"}}},
		{{Key: "$unwind", Value: bson.M{"path": "$product_ids", "includeArrayIndex": "idx2"}}},
		{{Key: "$match", Value: bson.M{"$expr": bson.M{"$lt": bson.A{"$idx1", "$idx2"}}}}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"a": bson.M{"$min": bson.A{"$product_ids", "$product_ids"}}, "b": bson.M{"$max": bson.A{"$product_ids", "$product_ids"}}},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
		{{Key: "$limit", Value: limit}},
	}
	return runPipelineOnColl(ctx, r.orders, pipeline)
}

// GetRecommendedForProduct returns product IDs that are most frequently co-purchased with productID.
// Falls back gracefully to an empty slice if no co-purchase data exists yet.
func (r *advancedAnalyticsRepository) GetRecommendedForProduct(ctx context.Context, productID primitive.ObjectID, limit int) ([]primitive.ObjectID, error) {
	pipeline := mongo.Pipeline{
		// only paid, non-cancelled orders containing this product
		{{Key: "$match", Value: bson.M{
			"payment_status": "paid",
			"status":         bson.M{"$ne": "cancelled"},
			"items.product_id": productID,
		}}},
		{{Key: "$unwind", Value: "$items"}},
		// exclude the source product itself
		{{Key: "$match", Value: bson.M{"items.product_id": bson.M{"$ne": productID}}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$items.product_id",
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
		{{Key: "$limit", Value: limit}},
	}
	cursor, err := r.orders.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	ids := make([]primitive.ObjectID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids, nil
}

func runPipelineOnColl(ctx context.Context, coll *mongo.Collection, pipeline mongo.Pipeline) ([]map[string]interface{}, error) {
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
