package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"rloco-backend/internal/models"
)

type PromotionRepository interface {
	Create(ctx context.Context, promotion *models.Promotion) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Promotion, error)
	GetByCode(ctx context.Context, code string) (*models.Promotion, error)
	Update(ctx context.Context, id primitive.ObjectID, promotion *models.Promotion) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, activeOnly bool) ([]*models.Promotion, error)
	IncrementUsage(ctx context.Context, id primitive.ObjectID) error
}

type promotionRepository struct {
	collection *mongo.Collection
}

func NewPromotionRepository(db *MongoDB) PromotionRepository {
	return &promotionRepository{
		collection: db.GetCollection("promotions"),
	}
}

func (r *promotionRepository) Create(ctx context.Context, promotion *models.Promotion) error {
	promotion.ID = primitive.NewObjectID()
	promotion.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, promotion)
	return err
}

func (r *promotionRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Promotion, error) {
	var promotion models.Promotion
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&promotion)
	if err != nil {
		return nil, err
	}
	return &promotion, nil
}

func (r *promotionRepository) GetByCode(ctx context.Context, code string) (*models.Promotion, error) {
	var promotion models.Promotion
	err := r.collection.FindOne(ctx, bson.M{"code": code}).Decode(&promotion)
	if err != nil {
		return nil, err
	}
	return &promotion, nil
}

func (r *promotionRepository) Update(ctx context.Context, id primitive.ObjectID, promotion *models.Promotion) error {
	promotion.ID = id // pin _id so $set never changes the immutable field
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": promotion},
	)
	return err
}

func (r *promotionRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *promotionRepository) List(ctx context.Context, activeOnly bool) ([]*models.Promotion, error) {
	filter := bson.M{}
	if activeOnly {
		now := time.Now()
		filter = bson.M{
			"is_active": true,
			"start_date": bson.M{"$lte": now},
			"end_date":   bson.M{"$gte": now},
		}
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	promotions := []*models.Promotion{}
	if err := cursor.All(ctx, &promotions); err != nil {
		return nil, err
	}
	return promotions, nil
}

func (r *promotionRepository) IncrementUsage(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$inc": bson.M{"usage_count": 1}},
	)
	return err
}

