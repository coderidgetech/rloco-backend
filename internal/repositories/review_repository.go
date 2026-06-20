package repositories

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"rloco-backend/internal/models"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *models.ProductReview) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.ProductReview, error)
	GetByProductID(ctx context.Context, productID primitive.ObjectID, limit, skip int) ([]*models.ProductReview, int64, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.ProductReview, int64, error)
	Update(ctx context.Context, id primitive.ObjectID, review *models.ProductReview) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	IncrementHelpful(ctx context.Context, id primitive.ObjectID) error
	GetProductRating(ctx context.Context, productID primitive.ObjectID) (float64, int, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
	// ListByStatus lists reviews for admin moderation (empty status = all).
	ListByStatus(ctx context.Context, status string, limit, skip int) ([]*models.ProductReview, int64, error)
}

type reviewRepository struct {
	collection *mongo.Collection
}

func NewReviewRepository(db *MongoDB) ReviewRepository {
	return &reviewRepository{
		collection: db.GetCollection("product_reviews"),
	}
}

func (r *reviewRepository) Create(ctx context.Context, review *models.ProductReview) error {
	review.ID = primitive.NewObjectID()
	review.CreatedAt = time.Now()
	review.UpdatedAt = time.Now()
	review.Status = "pending" // Default to pending for moderation

	_, err := r.collection.InsertOne(ctx, review)
	return err
}

func (r *reviewRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.ProductReview, error) {
	var review models.ProductReview
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&review)
	if err != nil {
		return nil, err
	}
	return &review, nil
}

func (r *reviewRepository) GetByProductID(ctx context.Context, productID primitive.ObjectID, limit, skip int) ([]*models.ProductReview, int64, error) {
	filter := bson.M{
		"product_id": productID,
		"status":      "approved", // Only show approved reviews
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.M{"created_at": -1}) // Newest first

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	reviews := []*models.ProductReview{}
	if err := cursor.All(ctx, &reviews); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

func (r *reviewRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.ProductReview, int64, error) {
	filter := bson.M{"user_id": userID}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	reviews := []*models.ProductReview{}
	if err := cursor.All(ctx, &reviews); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

func (r *reviewRepository) Update(ctx context.Context, id primitive.ObjectID, review *models.ProductReview) error {
	review.UpdatedAt = time.Now()
	review.ID = id // pin _id so $set never changes the immutable field
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": review},
	)
	return err
}

func (r *reviewRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		}},
	)
	return err
}

func (r *reviewRepository) IncrementHelpful(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$inc": bson.M{"helpful": 1}},
	)
	return err
}

func (r *reviewRepository) GetProductRating(ctx context.Context, productID primitive.ObjectID) (float64, int, error) {
	pipeline := []bson.M{
		{"$match": bson.M{
			"product_id": productID,
			"status":     "approved",
		}},
		{"$group": bson.M{
			"_id":        nil,
			"avg_rating": bson.M{"$avg": "$rating"},
			"count":      bson.M{"$sum": 1},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)

	var result struct {
		AvgRating float64 `bson:"avg_rating"`
		Count     int     `bson:"count"`
	}

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return 0, 0, err
		}
		return result.AvgRating, result.Count, nil
	}

	return 0, 0, nil // No reviews yet
}

func (r *reviewRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *reviewRepository) ListByStatus(ctx context.Context, status string, limit, skip int) ([]*models.ProductReview, int64, error) {
	filter := bson.M{}
	if strings.TrimSpace(status) != "" {
		filter["status"] = status
	}
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.M{"created_at": -1})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	reviews := []*models.ProductReview{}
	if err := cursor.All(ctx, &reviews); err != nil {
		return nil, 0, err
	}
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return reviews, total, nil
}
