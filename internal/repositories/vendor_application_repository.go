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

type VendorApplicationRepository interface {
	Create(ctx context.Context, app *models.VendorApplication) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.VendorApplication, error)
	List(ctx context.Context, status string, limit, skip int) ([]*models.VendorApplication, int64, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status, adminNotes string) error
}

type vendorApplicationRepository struct {
	col *mongo.Collection
}

func NewVendorApplicationRepository(db *MongoDB) VendorApplicationRepository {
	return &vendorApplicationRepository{col: db.GetCollection("vendor_applications")}
}

func (r *vendorApplicationRepository) Create(ctx context.Context, app *models.VendorApplication) error {
	app.ID = primitive.NewObjectID()
	app.Status = "pending"
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	_, err := r.col.InsertOne(ctx, app)
	return err
}

func (r *vendorApplicationRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.VendorApplication, error) {
	var app models.VendorApplication
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&app)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *vendorApplicationRepository) List(ctx context.Context, status string, limit, skip int) ([]*models.VendorApplication, int64, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(skip))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var apps []*models.VendorApplication
	if err := cursor.All(ctx, &apps); err != nil {
		return nil, 0, err
	}
	return apps, total, nil
}

func (r *vendorApplicationRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status, adminNotes string) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":      status,
			"admin_notes": adminNotes,
			"updated_at":  time.Now(),
		}},
	)
	return err
}
