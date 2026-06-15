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

type AddressRepository interface {
	Create(ctx context.Context, address *models.Address) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Address, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Address, error)
	Update(ctx context.Context, id primitive.ObjectID, address *models.Address) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	SetDefault(ctx context.Context, userID primitive.ObjectID, addressID primitive.ObjectID) error
}

type addressRepository struct {
	collection *mongo.Collection
}

func NewAddressRepository(db *MongoDB) AddressRepository {
	return &addressRepository{
		collection: db.GetCollection("addresses"),
	}
}

func (r *addressRepository) Create(ctx context.Context, address *models.Address) error {
	address.ID = primitive.NewObjectID()
	address.CreatedAt = time.Now()
	address.UpdatedAt = time.Now()

	// If this is the first address or marked as default, set it as default
	if address.IsDefault {
		// Unset all other default addresses for this user
		_, _ = r.collection.UpdateMany(
			ctx,
			bson.M{"user_id": address.UserID, "is_default": true},
			bson.M{"$set": bson.M{"is_default": false}},
		)
	} else {
		// Check if user has any addresses
		count, _ := r.collection.CountDocuments(ctx, bson.M{"user_id": address.UserID})
		if count == 0 {
			address.IsDefault = true
		}
	}

	_, err := r.collection.InsertOne(ctx, address)
	return err
}

func (r *addressRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Address, error) {
	var address models.Address
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&address)
	if err != nil {
		return nil, err
	}
	return &address, nil
}

func (r *addressRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Address, error) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{"user_id": userID},
		options.Find().SetSort(bson.D{{Key: "is_default", Value: -1}, {Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	addresses := []*models.Address{}
	if err := cursor.All(ctx, &addresses); err != nil {
		return nil, err
	}
	return addresses, nil
}

func (r *addressRepository) Update(ctx context.Context, id primitive.ObjectID, address *models.Address) error {
	address.UpdatedAt = time.Now()

	// If setting as default, unset all other defaults for this user
	if address.IsDefault {
		// Get the address first to get userID
		existing, err := r.GetByID(ctx, id)
		if err == nil {
			_, _ = r.collection.UpdateMany(
				ctx,
				bson.M{"user_id": existing.UserID, "_id": bson.M{"$ne": id}, "is_default": true},
				bson.M{"$set": bson.M{"is_default": false}},
			)
		}
	}

	// $set only mutable fields. Never $set the whole struct: the bound payload
	// carries _id (json "id"), which Mongo rejects as immutable, and would also
	// clobber created_at/user_id with client-supplied values.
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"name":          address.Name,
			"type":          address.Type,
			"address_line":  address.AddressLine,
			"address_line2": address.AddressLine2,
			"city":          address.City,
			"state":         address.State,
			"pincode":       address.Pincode,
			"mobile":        address.Mobile,
			"country":       address.Country,
			"is_default":    address.IsDefault,
			"updated_at":    address.UpdatedAt,
		}},
	)
	return err
}

func (r *addressRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *addressRepository) SetDefault(ctx context.Context, userID primitive.ObjectID, addressID primitive.ObjectID) error {
	// Unset all other default addresses
	_, err := r.collection.UpdateMany(
		ctx,
		bson.M{"user_id": userID, "is_default": true},
		bson.M{"$set": bson.M{"is_default": false}},
	)
	if err != nil {
		return err
	}

	// Set the specified address as default
	_, err = r.collection.UpdateOne(
		ctx,
		bson.M{"_id": addressID, "user_id": userID},
		bson.M{"$set": bson.M{"is_default": true, "updated_at": time.Now()}},
	)
	return err
}
