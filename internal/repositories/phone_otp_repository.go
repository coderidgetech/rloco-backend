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

const (
	PhoneOTPPurposeRegistration = "registration"
	PhoneOTPPurposeLogin        = "login"
)

type PhoneOTPRepository interface {
	Upsert(ctx context.Context, phoneKey, purpose, verifyID string, expiresAt, lastSentAt time.Time) error
	Get(ctx context.Context, phoneKey, purpose string) (*models.PhoneOTPChallenge, error)
	IncrementAttempts(ctx context.Context, id primitive.ObjectID) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type phoneOTPRepository struct {
	collection *mongo.Collection
}

func NewPhoneOTPRepository(db *MongoDB) PhoneOTPRepository {
	return &phoneOTPRepository{
		collection: db.GetCollection("phone_otp_challenges"),
	}
}

func (r *phoneOTPRepository) Upsert(ctx context.Context, phoneKey, purpose, verifyID string, expiresAt, lastSentAt time.Time) error {
	now := time.Now()
	filter := bson.M{"phone_key": phoneKey, "purpose": purpose}
	update := bson.M{
		"$set": bson.M{
			"verify_id":    verifyID,
			"expires_at":   expiresAt,
			"attempts":     0,
			"last_sent_at": lastSentAt,
			"updated_at":   now,
		},
		"$unset": bson.M{"code_hash": "", "otp_code_hash": ""},
	}
	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *phoneOTPRepository) Get(ctx context.Context, phoneKey, purpose string) (*models.PhoneOTPChallenge, error) {
	var doc models.PhoneOTPChallenge
	err := r.collection.FindOne(ctx, bson.M{"phone_key": phoneKey, "purpose": purpose}).Decode(&doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *phoneOTPRepository) IncrementAttempts(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"attempts": 1}, "$set": bson.M{"updated_at": time.Now()}})
	return err
}

func (r *phoneOTPRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
