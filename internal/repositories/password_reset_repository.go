package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"rloco-backend/internal/models"
)

type PasswordResetRepository interface {
	Create(ctx context.Context, token *models.PasswordResetToken) error
	GetByToken(ctx context.Context, token string) (*models.PasswordResetToken, error)
	MarkAsUsed(ctx context.Context, tokenID primitive.ObjectID) error
	DeleteExpired(ctx context.Context) error
}

type passwordResetRepository struct {
	collection *mongo.Collection
}

func NewPasswordResetRepository(db *MongoDB) PasswordResetRepository {
	return &passwordResetRepository{
		collection: db.GetCollection("password_reset_tokens"),
	}
}

func (r *passwordResetRepository) Create(ctx context.Context, token *models.PasswordResetToken) error {
	token.ID = primitive.NewObjectID()
	token.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, token)
	return err
}

func (r *passwordResetRepository) GetByToken(ctx context.Context, token string) (*models.PasswordResetToken, error) {
	var resetToken models.PasswordResetToken
	err := r.collection.FindOne(ctx, bson.M{"token": token, "used": false}).Decode(&resetToken)
	if err != nil {
		return nil, err
	}
	// Check if expired
	if time.Now().After(resetToken.ExpiresAt) {
		return nil, mongo.ErrNoDocuments
	}
	return &resetToken, nil
}

func (r *passwordResetRepository) MarkAsUsed(ctx context.Context, tokenID primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": tokenID},
		bson.M{"$set": bson.M{"used": true}},
	)
	return err
}

func (r *passwordResetRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{"expires_at": bson.M{"$lt": time.Now()}})
	return err
}
