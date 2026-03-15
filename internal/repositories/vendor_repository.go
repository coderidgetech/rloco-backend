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

type VendorRepository interface {
	Create(ctx context.Context, vendor *models.Vendor) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Vendor, error)
	Update(ctx context.Context, id primitive.ObjectID, vendor *models.Vendor) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, limit, skip int) ([]*models.Vendor, int64, error)
}

type vendorRepository struct {
	collection *mongo.Collection
}

func NewVendorRepository(db *MongoDB) VendorRepository {
	return &vendorRepository{
		collection: db.GetCollection("vendors"),
	}
}

func (r *vendorRepository) Create(ctx context.Context, vendor *models.Vendor) error {
	vendor.ID = primitive.NewObjectID()
	vendor.CreatedAt = time.Now()
	vendor.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, vendor)
	return err
}

func (r *vendorRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Vendor, error) {
	var vendor models.Vendor
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&vendor)
	if err != nil {
		return nil, err
	}
	return &vendor, nil
}

func (r *vendorRepository) Update(ctx context.Context, id primitive.ObjectID, vendor *models.Vendor) error {
	vendor.UpdatedAt = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": vendor},
	)
	return err
}

func (r *vendorRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *vendorRepository) List(ctx context.Context, limit, skip int) ([]*models.Vendor, int64, error) {
	opts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	vendors := []*models.Vendor{}
	if err := cursor.All(ctx, &vendors); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	return vendors, total, nil
}

