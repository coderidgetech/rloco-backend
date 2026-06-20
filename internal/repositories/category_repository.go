package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"rloco-backend/internal/models"
)

type CategoryRepository interface {
	Create(ctx context.Context, category *models.Category) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Category, error)
	GetBySlug(ctx context.Context, slug string) (*models.Category, error)
	Update(ctx context.Context, id primitive.ObjectID, category *models.Category) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context) ([]*models.Category, error)
}

type categoryRepository struct {
	collection *mongo.Collection
}

func NewCategoryRepository(db *MongoDB) CategoryRepository {
	return &categoryRepository{
		collection: db.GetCollection("categories"),
	}
}

func (r *categoryRepository) Create(ctx context.Context, category *models.Category) error {
	category.ID = primitive.NewObjectID()
	category.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, category)
	return err
}

func (r *categoryRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Category, error) {
	var category models.Category
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&category)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepository) GetBySlug(ctx context.Context, slug string) (*models.Category, error) {
	var category models.Category
	err := r.collection.FindOne(ctx, bson.M{"slug": slug}).Decode(&category)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepository) Update(ctx context.Context, id primitive.ObjectID, category *models.Category) error {
	category.ID = id // pin _id so $set never changes the immutable field
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": category},
	)
	return err
}

func (r *categoryRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *categoryRepository) List(ctx context.Context) ([]*models.Category, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	categories := []*models.Category{}
	if err := cursor.All(ctx, &categories); err != nil {
		return nil, err
	}
	return categories, nil
}

