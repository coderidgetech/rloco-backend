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

type ReturnRepository interface {
	Create(ctx context.Context, returnReq *models.Return) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Return, error)
	GetByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.Return, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Return, int64, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) ([]*models.Return, error)
	List(ctx context.Context, filter bson.M, limit, skip int) ([]*models.Return, int64, error)
	Update(ctx context.Context, id primitive.ObjectID, returnReq *models.Return) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	UpdateRefundStatus(ctx context.Context, id primitive.ObjectID, refundStatus string) error
}

type returnRepository struct {
	collection *mongo.Collection
}

func NewReturnRepository(db *MongoDB) ReturnRepository {
	return &returnRepository{
		collection: db.GetCollection("returns"),
	}
}

func (r *returnRepository) Create(ctx context.Context, returnReq *models.Return) error {
	returnReq.ID = primitive.NewObjectID()
	returnReq.CreatedAt = time.Now()
	returnReq.UpdatedAt = time.Now()
	returnReq.Status = "requested"
	returnReq.RefundStatus = "pending"

	_, err := r.collection.InsertOne(ctx, returnReq)
	return err
}

func (r *returnRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Return, error) {
	var returnReq models.Return
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&returnReq)
	if err != nil {
		return nil, err
	}
	return &returnReq, nil
}

func (r *returnRepository) GetByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.Return, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"order_id": orderID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	returns := []*models.Return{}
	if err := cursor.All(ctx, &returns); err != nil {
		return nil, err
	}
	return returns, nil
}

func (r *returnRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Return, int64, error) {
	filter := bson.M{"user_id": userID}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	returns := []*models.Return{}
	if err := cursor.All(ctx, &returns); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return returns, total, nil
}

func (r *returnRepository) GetByOrderNumber(ctx context.Context, orderNumber string) ([]*models.Return, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"order_number": orderNumber})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	returns := []*models.Return{}
	if err := cursor.All(ctx, &returns); err != nil {
		return nil, err
	}
	return returns, nil
}

func (r *returnRepository) List(ctx context.Context, filter bson.M, limit, skip int) ([]*models.Return, int64, error) {
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	returns := []*models.Return{}
	if err := cursor.All(ctx, &returns); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return returns, total, nil
}

func (r *returnRepository) Update(ctx context.Context, id primitive.ObjectID, returnReq *models.Return) error {
	returnReq.UpdatedAt = time.Now()
	returnReq.ID = id // pin _id so $set never changes the immutable field
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": returnReq},
	)
	return err
}

func (r *returnRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		}},
	)
	return err
}

func (r *returnRepository) UpdateRefundStatus(ctx context.Context, id primitive.ObjectID, refundStatus string) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"refund_status": refundStatus,
			"updated_at":     time.Now(),
		}},
	)
	return err
}
