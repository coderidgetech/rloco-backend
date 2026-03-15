package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"rloco-backend/internal/models"
)

type WishlistRepository interface {
	GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Wishlist, error)
	Add(ctx context.Context, wishlist *models.Wishlist) error
	Remove(ctx context.Context, userID, productID primitive.ObjectID) error
	IsInWishlist(ctx context.Context, userID, productID primitive.ObjectID) (bool, error)
	GetProductWishlistCount(ctx context.Context, productID primitive.ObjectID) (int64, error)
	GetUniqueUsersCount(ctx context.Context, productID primitive.ObjectID) (int64, error)
	GetAllByProductID(ctx context.Context, productID primitive.ObjectID) ([]*models.Wishlist, error)
}

type wishlistRepository struct {
	collection *mongo.Collection
}

func NewWishlistRepository(db *MongoDB) WishlistRepository {
	return &wishlistRepository{
		collection: db.GetCollection("wishlists"),
	}
}

func (r *wishlistRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Wishlist, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	wishlists := []*models.Wishlist{}
	if err := cursor.All(ctx, &wishlists); err != nil {
		return nil, err
	}
	return wishlists, nil
}

func (r *wishlistRepository) Add(ctx context.Context, wishlist *models.Wishlist) error {
	// Check if already exists
	existing := r.collection.FindOne(ctx, bson.M{
		"user_id":    wishlist.UserID,
		"product_id": wishlist.ProductID,
	})
	if existing.Err() == nil {
		// Already exists, return nil
		return nil
	}

	wishlist.ID = primitive.NewObjectID()
	wishlist.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, wishlist)
	return err
}

func (r *wishlistRepository) Remove(ctx context.Context, userID, productID primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{
		"user_id":    userID,
		"product_id": productID,
	})
	return err
}

func (r *wishlistRepository) IsInWishlist(ctx context.Context, userID, productID primitive.ObjectID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"user_id":    userID,
		"product_id": productID,
	})
	return count > 0, err
}

func (r *wishlistRepository) GetProductWishlistCount(ctx context.Context, productID primitive.ObjectID) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{"product_id": productID})
}

func (r *wishlistRepository) GetUniqueUsersCount(ctx context.Context, productID primitive.ObjectID) (int64, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"product_id": productID}},
		{"$group": bson.M{"_id": "$user_id"}},
		{"$count": "unique_users"},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	
	var result struct {
		UniqueUsers int64 `bson:"unique_users"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return 0, err
		}
		return result.UniqueUsers, nil
	}
	return 0, nil
}

func (r *wishlistRepository) GetAllByProductID(ctx context.Context, productID primitive.ObjectID) ([]*models.Wishlist, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"product_id": productID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	wishlists := []*models.Wishlist{}
	if err := cursor.All(ctx, &wishlists); err != nil {
		return nil, err
	}
	return wishlists, nil
}
