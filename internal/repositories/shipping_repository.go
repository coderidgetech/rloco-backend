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

type ShippingRepository interface {
	Create(ctx context.Context, method *models.ShippingMethod) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.ShippingMethod, error)
	List(ctx context.Context, activeOnly bool) ([]*models.ShippingMethod, error)
	Update(ctx context.Context, id primitive.ObjectID, method *models.ShippingMethod) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	GetByZone(ctx context.Context, country string) ([]*models.ShippingMethod, error)
}

type shippingRepository struct {
	collection *mongo.Collection
}

func NewShippingRepository(db *MongoDB) ShippingRepository {
	return &shippingRepository{
		collection: db.GetCollection("shipping_methods"),
	}
}

func (r *shippingRepository) Create(ctx context.Context, method *models.ShippingMethod) error {
	method.ID = primitive.NewObjectID()
	method.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, method)
	return err
}

func (r *shippingRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.ShippingMethod, error) {
	var method models.ShippingMethod
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&method)
	if err != nil {
		return nil, err
	}
	return &method, nil
}

func (r *shippingRepository) List(ctx context.Context, activeOnly bool) ([]*models.ShippingMethod, error) {
	filter := bson.M{}
	if activeOnly {
		filter["is_active"] = true
	}

	cursor, err := r.collection.Find(ctx, filter, options.Find().SetSort(bson.M{"created_at": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	methods := []*models.ShippingMethod{}
	if err := cursor.All(ctx, &methods); err != nil {
		return nil, err
	}
	return methods, nil
}

func (r *shippingRepository) Update(ctx context.Context, id primitive.ObjectID, method *models.ShippingMethod) error {
	method.ID = id // pin _id so $set never changes the immutable field
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": method},
	)
	return err
}

func (r *shippingRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *shippingRepository) GetByZone(ctx context.Context, country string) ([]*models.ShippingMethod, error) {
	filter := bson.M{
		"is_active": true,
		"zones.countries": country,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	methods := []*models.ShippingMethod{}
	if err := cursor.All(ctx, &methods); err != nil {
		return nil, err
	}
	return methods, nil
}
