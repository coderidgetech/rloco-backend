package repositories

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"rloco-backend/internal/models"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByPhoneKey(ctx context.Context, phoneKey string) (*models.User, error)
	Update(ctx context.Context, id primitive.ObjectID, user *models.User) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, limit, skip int) ([]*models.User, error)
}

type userRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(db *MongoDB) UserRepository {
	return &userRepository{
		collection: db.GetCollection("users"),
	}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	user.ID = primitive.NewObjectID()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, user)
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// phoneStringLookupVariants returns common stored forms for the same logical number (E.164 digits in phoneKey).
func phoneStringLookupVariants(phoneKey string) []string {
	if phoneKey == "" {
		return nil
	}
	seen := map[string]struct{}{"+" + phoneKey: {}, phoneKey: {}}
	out := []string{"+" + phoneKey, phoneKey}
	// India: 91 + 10-digit national → also match "+91 9xxxxxxxxx" saved from profile UI
	if strings.HasPrefix(phoneKey, "91") && len(phoneKey) == 12 {
		local := phoneKey[2:]
		for _, v := range []string{
			"+91 " + local,
			"+91-" + local,
			local,
		} {
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				out = append(out, v)
			}
		}
	}
	return out
}

func (r *userRepository) GetByPhoneKey(ctx context.Context, phoneKey string) (*models.User, error) {
	if phoneKey == "" {
		return nil, mongo.ErrNoDocuments
	}
	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"phone_key": phoneKey}).Decode(&user)
	if err == nil {
		return &user, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}
	// Legacy / profile-only: phone set without phone_key, or formatted differently than canonical E.164
	or := make([]bson.M, 0, 8)
	for _, p := range phoneStringLookupVariants(phoneKey) {
		or = append(or, bson.M{"phone": p})
	}
	err = r.collection.FindOne(ctx, bson.M{"$or": or}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, id primitive.ObjectID, user *models.User) error {
	user.UpdatedAt = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": user},
	)
	return err
}

func (r *userRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *userRepository) List(ctx context.Context, limit, skip int) ([]*models.User, error) {
	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetLimit(int64(limit)).SetSkip(int64(skip)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	users := []*models.User{}
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

