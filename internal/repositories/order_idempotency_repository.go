package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// OrderIdempotencyRepository maps (user, client idempotency key) → order for safe checkout retries.
type OrderIdempotencyRepository interface {
	TryReserve(ctx context.Context, userID primitive.ObjectID, clientKey string) (leased bool, err error)
	LookupOrderID(ctx context.Context, userID primitive.ObjectID, clientKey string) (*primitive.ObjectID, error)
	Commit(ctx context.Context, userID primitive.ObjectID, clientKey string, orderID primitive.ObjectID) error
	Release(ctx context.Context, userID primitive.ObjectID, clientKey string) error
}

type orderIdempotencyRepository struct {
	collection *mongo.Collection
}

func NewOrderIdempotencyRepository(db *MongoDB) OrderIdempotencyRepository {
	return &orderIdempotencyRepository{collection: db.GetCollection("order_idempotency")}
}

func idempotencyDocID(userID primitive.ObjectID, clientKey string) string {
	h := sha256.Sum256(append([]byte(userID.Hex()+":"), []byte(clientKey)...))
	return hex.EncodeToString(h[:])
}

func (r *orderIdempotencyRepository) TryReserve(ctx context.Context, userID primitive.ObjectID, clientKey string) (bool, error) {
	if clientKey == "" {
		return true, nil
	}
	id := idempotencyDocID(userID, clientKey)
	now := time.Now()
	_, err := r.collection.InsertOne(ctx, bson.M{
		"_id":         id,
		"user_id":     userID,
		"order_id":    nil,
		"created_at":  now,
		"reserved_at": now,
	})
	if err == nil {
		return true, nil
	}
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	return false, err
}

func (r *orderIdempotencyRepository) LookupOrderID(ctx context.Context, userID primitive.ObjectID, clientKey string) (*primitive.ObjectID, error) {
	if clientKey == "" {
		return nil, nil
	}
	id := idempotencyDocID(userID, clientKey)
	var doc struct {
		UserID  primitive.ObjectID  `bson:"user_id"`
		OrderID *primitive.ObjectID `bson:"order_id"`
	}
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	if doc.UserID != userID {
		return nil, nil
	}
	return doc.OrderID, nil
}

func (r *orderIdempotencyRepository) Commit(ctx context.Context, userID primitive.ObjectID, clientKey string, orderID primitive.ObjectID) error {
	if clientKey == "" {
		return nil
	}
	id := idempotencyDocID(userID, clientKey)
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"_id": id, "user_id": userID},
		bson.M{"$set": bson.M{"order_id": orderID, "updated_at": time.Now()}},
	)
	return err
}

func (r *orderIdempotencyRepository) Release(ctx context.Context, userID primitive.ObjectID, clientKey string) error {
	if clientKey == "" {
		return nil
	}
	id := idempotencyDocID(userID, clientKey)
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id, "user_id": userID, "order_id": nil})
	return err
}
