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

type RewardsRepository interface {
	AddTransaction(ctx context.Context, tx *models.RewardsTransaction) error
	GetUserTransactions(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]models.RewardsTransaction, int64, error)
	GetUserBalance(ctx context.Context, userID primitive.ObjectID) (int64, error)
}

type rewardsRepository struct {
	collection *mongo.Collection
}

func NewRewardsRepository(db *MongoDB) RewardsRepository {
	return &rewardsRepository{
		collection: db.GetCollection("rewards_transactions"),
	}
}

func (r *rewardsRepository) AddTransaction(ctx context.Context, tx *models.RewardsTransaction) error {
	tx.ID = primitive.NewObjectID()
	tx.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, tx)
	return err
}

func (r *rewardsRepository) GetUserTransactions(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]models.RewardsTransaction, int64, error) {
	filter := bson.M{"user_id": userID}
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().
		SetSort(bson.M{"created_at": -1}).
		SetLimit(int64(limit)).
		SetSkip(int64(skip))
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var txs []models.RewardsTransaction
	if err := cursor.All(ctx, &txs); err != nil {
		return nil, 0, err
	}
	return txs, total, nil
}

// GetUserBalance returns earned - redeemed points for a user.
func (r *rewardsRepository) GetUserBalance(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user_id": userID}}},
		{{Key: "$group", Value: bson.M{
			"_id": nil,
			"balance": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{"$type", "earned"}},
				"$points",
				bson.M{"$multiply": bson.A{"$points", -1}},
			}}},
		}}},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var result []struct {
		Balance int64 `bson:"balance"`
	}
	if err := cursor.All(ctx, &result); err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	if result[0].Balance < 0 {
		return 0, nil
	}
	return result[0].Balance, nil
}
