package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// StripeWebhookEventRepository records processed Stripe event IDs so webhook retries are idempotent.
type StripeWebhookEventRepository interface {
	// TryInsert returns true if this event was recorded for the first time, false if duplicate (already processed).
	TryInsert(ctx context.Context, eventID string) (inserted bool, err error)
	// Delete removes an event id so Stripe can retry after a transient handler failure.
	Delete(ctx context.Context, eventID string) error
}

type stripeWebhookEventRepository struct {
	collection *mongo.Collection
}

func NewStripeWebhookEventRepository(db *MongoDB) StripeWebhookEventRepository {
	return &stripeWebhookEventRepository{collection: db.GetCollection("stripe_webhook_events")}
}

func (r *stripeWebhookEventRepository) TryInsert(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}
	_, err := r.collection.InsertOne(ctx, bson.M{
		"_id":        eventID,
		"created_at": time.Now(),
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *stripeWebhookEventRepository) Delete(ctx context.Context, eventID string) error {
	if eventID == "" {
		return nil
	}
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": eventID})
	return err
}
