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

type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*models.Order, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Order, int64, error)
	Update(ctx context.Context, id primitive.ObjectID, order *models.Order) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	UpdatePaymentStatus(ctx context.Context, id primitive.ObjectID, paymentStatus string) error
	// CompareAndSwapPaymentStatus atomically moves payment_status from->to, returning
	// true only for the caller that won the transition (used to guard refunds).
	CompareAndSwapPaymentStatus(ctx context.Context, id primitive.ObjectID, from, to string) (bool, error)
	SetTrackingNumber(ctx context.Context, id primitive.ObjectID, trackingNumber, labelURL string) error
	GetByTrackingNumber(ctx context.Context, trackingNumber string) (*models.Order, error)
	List(ctx context.Context, filter bson.M, limit, skip int) ([]*models.Order, int64, error)
	GetStats(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)
	// GetUserRewardStats: non-cancelled orders count and sum of order totals (lifetime value).
	GetUserRewardStats(ctx context.Context, userID primitive.ObjectID) (orderCount int64, lifetimeSpend float64, err error)
	// CountBuyersOfProductFromUsers counts distinct users from the given set who ordered the product.
	CountBuyersOfProductFromUsers(ctx context.Context, productID primitive.ObjectID, userIDs []primitive.ObjectID) (int64, error)
}

type orderRepository struct {
	collection *mongo.Collection
}

func NewOrderRepository(db *MongoDB) OrderRepository {
	return &orderRepository{
		collection: db.GetCollection("orders"),
	}
}

func (r *orderRepository) Create(ctx context.Context, order *models.Order) error {
	order.ID = primitive.NewObjectID()
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, order)
	return err
}

func (r *orderRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	var order models.Order
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&order)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) GetByOrderNumber(ctx context.Context, orderNumber string) (*models.Order, error) {
	var order models.Order
	err := r.collection.FindOne(ctx, bson.M{"order_number": orderNumber}).Decode(&order)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *orderRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Order, int64, error) {
	filter := bson.M{"user_id": userID}
	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip)).SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	orders := []*models.Order{}
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *orderRepository) Update(ctx context.Context, id primitive.ObjectID, order *models.Order) error {
	order.UpdatedAt = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": order},
	)
	return err
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
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

func (r *orderRepository) UpdatePaymentStatus(ctx context.Context, id primitive.ObjectID, paymentStatus string) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"payment_status": paymentStatus,
			"updated_at":     time.Now(),
		}},
	)
	return err
}

func (r *orderRepository) CompareAndSwapPaymentStatus(ctx context.Context, id primitive.ObjectID, from, to string) (bool, error) {
	res, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id, "payment_status": from},
		bson.M{"$set": bson.M{
			"payment_status": to,
			"updated_at":     time.Now(),
		}},
	)
	if err != nil {
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

func (r *orderRepository) List(ctx context.Context, filter bson.M, limit, skip int) ([]*models.Order, int64, error) {
	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip)).SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	orders := []*models.Order{}
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *orderRepository) GetUserRewardStats(ctx context.Context, userID primitive.ObjectID) (int64, float64, error) {
	match := bson.M{
		"user_id": userID,
		"status":  bson.M{"$ne": "cancelled"},
	}
	pipeline := []bson.M{
		{"$match": match},
		{"$group": bson.M{
			"_id": nil,
			"n":   bson.M{"$sum": 1},
			"sum": bson.M{"$sum": "$total"},
		}},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)
	var out []struct {
		N   int64   `bson:"n"`
		Sum float64 `bson:"sum"`
	}
	if err := cursor.All(ctx, &out); err != nil {
		return 0, 0, err
	}
	if len(out) == 0 {
		return 0, 0, nil
	}
	return out[0].N, out[0].Sum, nil
}

func (r *orderRepository) GetStats(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
	matchStage := bson.M{
		"$match": bson.M{
			"created_at": bson.M{
				"$gte": startDate,
				"$lte": endDate,
			},
		},
	}

	groupStage := bson.M{
		"$group": bson.M{
			"_id": nil,
			"total_revenue": bson.M{"$sum": "$total"},
			"total_orders":  bson.M{"$sum": 1},
			"avg_order_value": bson.M{"$avg": "$total"},
		},
	}

	pipeline := []bson.M{matchStage, groupStage}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := []map[string]interface{}{}
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return map[string]interface{}{
			"total_revenue":   0,
			"total_orders":    0,
			"avg_order_value": 0,
		}, nil
	}

	return result[0], nil
}

func (r *orderRepository) SetTrackingNumber(ctx context.Context, id primitive.ObjectID, trackingNumber, labelURL string) error {
	fields := bson.M{
		"tracking_number": trackingNumber,
		"updated_at":      time.Now(),
	}
	if labelURL != "" {
		fields["label_url"] = labelURL
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": fields})
	return err
}

func (r *orderRepository) GetByTrackingNumber(ctx context.Context, trackingNumber string) (*models.Order, error) {
	var order models.Order
	err := r.collection.FindOne(ctx, bson.M{"tracking_number": trackingNumber}).Decode(&order)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &order, err
}

func (r *orderRepository) CountBuyersOfProductFromUsers(ctx context.Context, productID primitive.ObjectID, userIDs []primitive.ObjectID) (int64, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}
	pipeline := []bson.M{
		{"$match": bson.M{
			"user_id":           bson.M{"$in": userIDs},
			"items.product_id":  productID,
			"status":            bson.M{"$ne": "cancelled"},
		}},
		{"$group": bson.M{"_id": "$user_id"}},
		{"$count": "buyers"},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var result struct {
		Buyers int64 `bson:"buyers"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return 0, err
		}
		return result.Buyers, nil
	}
	return 0, nil
}

