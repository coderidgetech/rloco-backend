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

type ConfigRepository interface {
	Get(ctx context.Context) (*models.SiteConfig, error)
	Update(ctx context.Context, config *models.SiteConfig) error
}

type configRepository struct {
	collection *mongo.Collection
}

func NewConfigRepository(db *MongoDB) ConfigRepository {
	return &configRepository{
		collection: db.GetCollection("site_config"),
	}
}

func (r *configRepository) Get(ctx context.Context) (*models.SiteConfig, error) {
	var config models.SiteConfig
	err := r.collection.FindOne(ctx, bson.M{}).Decode(&config)
	if err == mongo.ErrNoDocuments {
		// Return default config if not found
		return &models.SiteConfig{
			ID:        primitive.NewObjectID(),
			Config:    make(map[string]interface{}),
			UpdatedAt: time.Now(),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *configRepository) Update(ctx context.Context, config *models.SiteConfig) error {
	config.UpdatedAt = time.Now()
	// Only update config and updated_at fields, not the entire object (which includes _id)
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{},
		bson.M{"$set": bson.M{
			"config":     config.Config,
			"updated_at": config.UpdatedAt,
		}},
		options.Update().SetUpsert(true),
	)
	return err
}

