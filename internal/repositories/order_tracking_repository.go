package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"rloco-backend/internal/models"
)

type OrderTrackingRepository interface {
	Create(ctx context.Context, update *models.OrderTrackingUpdate) error
	GetByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderTrackingUpdate, error)
	GetLatestByOrderID(ctx context.Context, orderID primitive.ObjectID) (*models.OrderTrackingUpdate, error)
}

type orderTrackingRepository struct {
	collection *mongo.Collection
}

func NewOrderTrackingRepository(db *MongoDB) OrderTrackingRepository {
	return &orderTrackingRepository{
		collection: db.GetCollection("order_tracking_updates"),
	}
}

func (r *orderTrackingRepository) Create(ctx context.Context, update *models.OrderTrackingUpdate) error {
	update.ID = primitive.NewObjectID()
	update.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, update)
	return err
}

func (r *orderTrackingRepository) GetByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderTrackingUpdate, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"order_id": orderID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	updates := []*models.OrderTrackingUpdate{}
	if err := cursor.All(ctx, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (r *orderTrackingRepository) GetLatestByOrderID(ctx context.Context, orderID primitive.ObjectID) (*models.OrderTrackingUpdate, error) {
	var update models.OrderTrackingUpdate
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	err := r.collection.FindOne(ctx, bson.M{"order_id": orderID}, opts).Decode(&update)
	if err != nil {
		return nil, err
	}
	return &update, nil
}
