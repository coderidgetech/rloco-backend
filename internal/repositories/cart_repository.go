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

type CartRepository interface {
	GetByUserID(ctx context.Context, userID primitive.ObjectID) (*models.Cart, error)
	Create(ctx context.Context, cart *models.Cart) error
	Update(ctx context.Context, cart *models.Cart) error
	Clear(ctx context.Context, userID primitive.ObjectID) error
}

type cartRepository struct {
	collection *mongo.Collection
}

func NewCartRepository(db *MongoDB) CartRepository {
	return &cartRepository{
		collection: db.GetCollection("carts"),
	}
}

func (r *cartRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID) (*models.Cart, error) {
	var cart models.Cart
	err := r.collection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&cart)
	if err == mongo.ErrNoDocuments {
		// Create empty cart if not found
		cart = models.Cart{
			ID:        primitive.NewObjectID(),
			UserID:    userID,
			Items:     []models.CartItem{},
			UpdatedAt: time.Now(),
		}
		_, err = r.collection.InsertOne(ctx, cart)
		if err != nil {
			return nil, err
		}
		return &cart, nil
	}
	if err != nil {
		return nil, err
	}
	return &cart, nil
}

func (r *cartRepository) Create(ctx context.Context, cart *models.Cart) error {
	cart.ID = primitive.NewObjectID()
	cart.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, cart)
	return err
}

func (r *cartRepository) Update(ctx context.Context, cart *models.Cart) error {
	cart.UpdatedAt = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"user_id": cart.UserID},
		bson.M{"$set": cart},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *cartRepository) Clear(ctx context.Context, userID primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"user_id": userID},
		bson.M{"$set": bson.M{
			"items":      []models.CartItem{},
			"updated_at": time.Now(),
		}},
	)
	return err
}

