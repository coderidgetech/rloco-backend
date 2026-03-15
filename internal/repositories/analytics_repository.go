package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"rloco-backend/internal/models"
)

type AnalyticsRepository interface {
	CreateEvent(ctx context.Context, event *models.AnalyticsEvent) error
	GetEvents(ctx context.Context, filter bson.M, limit, skip int) ([]*models.AnalyticsEvent, int64, error)
	GetPageViews(ctx context.Context, startDate, endDate time.Time) ([]map[string]interface{}, error)
	GetConversionRate(ctx context.Context, startDate, endDate time.Time) (float64, error)
}

type analyticsRepository struct {
	collection *mongo.Collection
}

func NewAnalyticsRepository(db *MongoDB) AnalyticsRepository {
	return &analyticsRepository{
		collection: db.GetCollection("analytics_events"),
	}
}

func (r *analyticsRepository) CreateEvent(ctx context.Context, event *models.AnalyticsEvent) error {
	event.ID = primitive.NewObjectID()
	event.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, event)
	return err
}

func (r *analyticsRepository) GetEvents(ctx context.Context, filter bson.M, limit, skip int) ([]*models.AnalyticsEvent, int64, error) {
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	events := []*models.AnalyticsEvent{}
	if err := cursor.All(ctx, &events); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

func (r *analyticsRepository) GetPageViews(ctx context.Context, startDate, endDate time.Time) ([]map[string]interface{}, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"event_type": "page_view",
				"created_at": bson.M{
					"$gte": startDate,
					"$lte": endDate,
				},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"$dateToString": bson.M{
						"format": "%Y-%m-%d",
						"date":   "$created_at",
					},
				},
				"views": bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	results := []map[string]interface{}{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (r *analyticsRepository) GetConversionRate(ctx context.Context, startDate, endDate time.Time) (float64, error) {
	// Get total page views
	pageViews, err := r.collection.CountDocuments(ctx, bson.M{
		"event_type": "page_view",
		"created_at": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	})
	if err != nil {
		return 0, err
	}

	// Get total purchases
	purchases, err := r.collection.CountDocuments(ctx, bson.M{
		"event_type": "purchase",
		"created_at": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	})
	if err != nil {
		return 0, err
	}

	if pageViews == 0 {
		return 0, nil
	}

	return float64(purchases) / float64(pageViews) * 100, nil
}

