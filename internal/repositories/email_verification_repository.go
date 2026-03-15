package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"rloco-backend/internal/models"
)

type EmailVerificationRepository interface {
	Create(ctx context.Context, token *models.EmailVerificationToken) error
	GetByToken(ctx context.Context, token string) (*models.EmailVerificationToken, error)
	MarkAsUsed(ctx context.Context, tokenID primitive.ObjectID) error
	DeleteExpired(ctx context.Context) error
}

type emailVerificationRepository struct {
	collection *mongo.Collection
}

func NewEmailVerificationRepository(db *MongoDB) EmailVerificationRepository {
	return &emailVerificationRepository{
		collection: db.GetCollection("email_verification_tokens"),
	}
}

func (r *emailVerificationRepository) Create(ctx context.Context, token *models.EmailVerificationToken) error {
	token.ID = primitive.NewObjectID()
	token.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, token)
	return err
}

func (r *emailVerificationRepository) GetByToken(ctx context.Context, token string) (*models.EmailVerificationToken, error) {
	var verifyToken models.EmailVerificationToken
	err := r.collection.FindOne(ctx, bson.M{"token": token, "used": false}).Decode(&verifyToken)
	if err != nil {
		return nil, err
	}
	// Check if expired
	if time.Now().After(verifyToken.ExpiresAt) {
		return nil, mongo.ErrNoDocuments
	}
	return &verifyToken, nil
}

func (r *emailVerificationRepository) MarkAsUsed(ctx context.Context, tokenID primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": tokenID},
		bson.M{"$set": bson.M{"used": true}},
	)
	return err
}

func (r *emailVerificationRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{"expires_at": bson.M{"$lt": time.Now()}})
	return err
}
