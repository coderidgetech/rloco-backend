package repositories

import (
	"context"
	"time"

	"rloco-backend/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PaymentRepository interface {
	Create(ctx context.Context, transaction *models.PaymentTransaction) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.PaymentTransaction, error)
	GetByOrderID(ctx context.Context, orderID primitive.ObjectID) (*models.PaymentTransaction, error)
	GetByGatewayTransactionID(ctx context.Context, gatewayTransactionID string) (*models.PaymentTransaction, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string, failureReason *string) error
	UpdateGatewayTransactionID(ctx context.Context, id primitive.ObjectID, gatewayTransactionID string) error
}

type paymentRepository struct {
	collection *mongo.Collection
}

func NewPaymentRepository(db *MongoDB) PaymentRepository {
	return &paymentRepository{
		collection: db.GetCollection("payment_transactions"),
	}
}

func (r *paymentRepository) Create(ctx context.Context, transaction *models.PaymentTransaction) error {
	transaction.ID = primitive.NewObjectID()
	transaction.CreatedAt = time.Now()
	transaction.UpdatedAt = time.Now()
	transaction.Status = "pending"
	// Preserve unique index compatibility before gateway returns its transaction ID.
	if transaction.GatewayTransactionID == "" {
		transaction.GatewayTransactionID = transaction.ID.Hex()
	}

	_, err := r.collection.InsertOne(ctx, transaction)
	return err
}

func (r *paymentRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.PaymentTransaction, error) {
	var transaction models.PaymentTransaction
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&transaction)
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *paymentRepository) GetByOrderID(ctx context.Context, orderID primitive.ObjectID) (*models.PaymentTransaction, error) {
	var transaction models.PaymentTransaction
	err := r.collection.FindOne(ctx, bson.M{"order_id": orderID}, options.FindOne().SetSort(bson.M{"created_at": -1})).Decode(&transaction)
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *paymentRepository) GetByGatewayTransactionID(ctx context.Context, gatewayTransactionID string) (*models.PaymentTransaction, error) {
	var transaction models.PaymentTransaction
	err := r.collection.FindOne(ctx, bson.M{"gateway_transaction_id": gatewayTransactionID}).Decode(&transaction)
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (r *paymentRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string, failureReason *string) error {
	update := bson.M{
		"status":     status,
		"updated_at": time.Now(),
	}
	if failureReason != nil {
		update["failure_reason"] = *failureReason
	}

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": update},
	)
	return err
}

func (r *paymentRepository) UpdateGatewayTransactionID(ctx context.Context, id primitive.ObjectID, gatewayTransactionID string) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"gateway_transaction_id": gatewayTransactionID,
			"updated_at":             time.Now(),
		}},
	)
	return err
}
