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

type VideoRepository interface {
	Create(ctx context.Context, video *models.InspirationVideo) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.InspirationVideo, error)
	Update(ctx context.Context, id primitive.ObjectID, video *models.InspirationVideo) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, filter bson.M, limit, skip int) ([]*models.InspirationVideo, int64, error)
	GetFeatured(ctx context.Context, limit int) ([]*models.InspirationVideo, error)
	GetByCategory(ctx context.Context, category string, limit int) ([]*models.InspirationVideo, error)
}

type videoRepository struct {
	collection *mongo.Collection
}

func NewVideoRepository(db *MongoDB) VideoRepository {
	return &videoRepository{
		collection: db.GetCollection("videos"),
	}
}

func (r *videoRepository) Create(ctx context.Context, video *models.InspirationVideo) error {
	video.ID = primitive.NewObjectID()
	video.CreatedAt = time.Now()
	video.UpdatedAt = time.Now()
	video.IsActive = true

	_, err := r.collection.InsertOne(ctx, video)
	return err
}

func (r *videoRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.InspirationVideo, error) {
	var video models.InspirationVideo
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&video)
	if err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *videoRepository) Update(ctx context.Context, id primitive.ObjectID, video *models.InspirationVideo) error {
	video.UpdatedAt = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": video},
	)
	return err
}

func (r *videoRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *videoRepository) List(ctx context.Context, filter bson.M, limit, skip int) ([]*models.InspirationVideo, int64, error) {
	if filter == nil {
		filter = bson.M{}
	}
	filter["is_active"] = true

	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip)).SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	videos := []*models.InspirationVideo{}
	if err := cursor.All(ctx, &videos); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

func (r *videoRepository) GetFeatured(ctx context.Context, limit int) ([]*models.InspirationVideo, error) {
	filter := bson.M{
		"featured": true,
		"is_active": true,
	}

	cursor, err := r.collection.Find(
		ctx,
		filter,
		options.Find().SetLimit(int64(limit)).SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	videos := []*models.InspirationVideo{}
	if err := cursor.All(ctx, &videos); err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *videoRepository) GetByCategory(ctx context.Context, category string, limit int) ([]*models.InspirationVideo, error) {
	filter := bson.M{
		"category": category,
		"is_active": true,
	}

	cursor, err := r.collection.Find(
		ctx,
		filter,
		options.Find().SetLimit(int64(limit)).SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	videos := []*models.InspirationVideo{}
	if err := cursor.All(ctx, &videos); err != nil {
		return nil, err
	}
	return videos, nil
}
