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

type SupportRepository interface {
	Create(ctx context.Context, ticket *models.SupportTicket) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.SupportTicket, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.SupportTicket, int64, error)
	List(ctx context.Context, filter bson.M, limit, skip int) ([]*models.SupportTicket, int64, error)
	Update(ctx context.Context, id primitive.ObjectID, ticket *models.SupportTicket) error
	AddMessage(ctx context.Context, ticketID primitive.ObjectID, message *models.TicketMessage) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	Assign(ctx context.Context, id primitive.ObjectID, assignedTo primitive.ObjectID) error
}

type supportRepository struct {
	collection *mongo.Collection
}

func NewSupportRepository(db *MongoDB) SupportRepository {
	return &supportRepository{
		collection: db.GetCollection("support_tickets"),
	}
}

func (r *supportRepository) Create(ctx context.Context, ticket *models.SupportTicket) error {
	ticket.ID = primitive.NewObjectID()
	ticket.CreatedAt = time.Now()
	ticket.UpdatedAt = time.Now()
	ticket.Status = "open"

	_, err := r.collection.InsertOne(ctx, ticket)
	return err
}

func (r *supportRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.SupportTicket, error) {
	var ticket models.SupportTicket
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&ticket)
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *supportRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.SupportTicket, int64, error) {
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

	tickets := []*models.SupportTicket{}
	if err := cursor.All(ctx, &tickets); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

func (r *supportRepository) List(ctx context.Context, filter bson.M, limit, skip int) ([]*models.SupportTicket, int64, error) {
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(skip)).
		SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	tickets := []*models.SupportTicket{}
	if err := cursor.All(ctx, &tickets); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

func (r *supportRepository) Update(ctx context.Context, id primitive.ObjectID, ticket *models.SupportTicket) error {
	ticket.UpdatedAt = time.Now()
	ticket.ID = id // pin _id so $set never changes the immutable field
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": ticket},
	)
	return err
}

func (r *supportRepository) AddMessage(ctx context.Context, ticketID primitive.ObjectID, message *models.TicketMessage) error {
	message.ID = primitive.NewObjectID()
	message.CreatedAt = time.Now()

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": ticketID},
		bson.M{
			"$push": bson.M{"messages": message},
			"$set":  bson.M{"updated_at": time.Now()},
		},
	)
	return err
}

func (r *supportRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
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

func (r *supportRepository) Assign(ctx context.Context, id primitive.ObjectID, assignedTo primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"assigned_to": assignedTo,
			"updated_at":  time.Now(),
		}},
	)
	return err
}
