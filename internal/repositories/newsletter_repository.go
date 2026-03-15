package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"rloco-backend/internal/models"
)

type NewsletterRepository interface {
	Create(ctx context.Context, subscription *models.NewsletterSubscription) error
	GetByEmail(ctx context.Context, email string) (*models.NewsletterSubscription, error)
	Update(ctx context.Context, id primitive.ObjectID, subscription *models.NewsletterSubscription) error
	Unsubscribe(ctx context.Context, email string) error
	List(ctx context.Context, limit, skip int) ([]*models.NewsletterSubscription, int64, error)
}

type newsletterRepository struct {
	collection *mongo.Collection
}

func NewNewsletterRepository(db *MongoDB) NewsletterRepository {
	return &newsletterRepository{
		collection: db.GetCollection("newsletter_subscriptions"),
	}
}

func (r *newsletterRepository) Create(ctx context.Context, subscription *models.NewsletterSubscription) error {
	// Check if already exists
	existing, _ := r.GetByEmail(ctx, subscription.Email)
	if existing != nil {
		// Update existing subscription to active
		existing.Active = true
		existing.UpdatedAt = time.Now()
		if subscription.Name != nil {
			existing.Name = subscription.Name
		}
		return r.Update(ctx, existing.ID, existing)
	}

	subscription.ID = primitive.NewObjectID()
	subscription.Active = true
	subscription.CreatedAt = time.Now()
	subscription.UpdatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, subscription)
	return err
}

func (r *newsletterRepository) GetByEmail(ctx context.Context, email string) (*models.NewsletterSubscription, error) {
	var subscription models.NewsletterSubscription
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&subscription)
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (r *newsletterRepository) Update(ctx context.Context, id primitive.ObjectID, subscription *models.NewsletterSubscription) error {
	subscription.UpdatedAt = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": subscription},
	)
	return err
}

func (r *newsletterRepository) Unsubscribe(ctx context.Context, email string) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"email": email},
		bson.M{"$set": bson.M{"active": false, "updated_at": time.Now()}},
	)
	return err
}

func (r *newsletterRepository) List(ctx context.Context, limit, skip int) ([]*models.NewsletterSubscription, int64, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	subscriptions := []*models.NewsletterSubscription{}
	if err := cursor.All(ctx, &subscriptions); err != nil {
		return nil, 0, err
	}

	total := int64(len(subscriptions))

	// Apply pagination
	start := skip
	if start > len(subscriptions) {
		start = len(subscriptions)
	}
	end := start + limit
	if end > len(subscriptions) {
		end = len(subscriptions)
	}

	if start < len(subscriptions) {
		return subscriptions[start:end], total, nil
	}
	return []*models.NewsletterSubscription{}, total, nil
}
