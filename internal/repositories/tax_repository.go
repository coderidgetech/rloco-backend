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

type TaxRepository interface {
	Create(ctx context.Context, rate *models.TaxRate) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.TaxRate, error)
	GetByLocation(ctx context.Context, country, state, city, postalCode string) (*models.TaxRate, error)
	List(ctx context.Context, activeOnly bool) ([]*models.TaxRate, error)
	Update(ctx context.Context, id primitive.ObjectID, rate *models.TaxRate) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type taxRepository struct {
	collection            *mongo.Collection
	indiaDefaultGSTPercent float64 // when no Mongo row matches India (Rate is percent, e.g. 18 = 18%)
}

func NewTaxRepository(db *MongoDB, indiaDefaultGSTPercent float64) TaxRepository {
	if indiaDefaultGSTPercent < 0 {
		indiaDefaultGSTPercent = 0
	}
	return &taxRepository{
		collection:             db.GetCollection("tax_rates"),
		indiaDefaultGSTPercent: indiaDefaultGSTPercent,
	}
}

func taxRegionFromCountry(country string) string {
	switch strings.TrimSpace(strings.ToLower(country)) {
	case "united states", "us", "usa":
		return "US"
	case "india", "in":
		return "IN"
	default:
		return ""
	}
}

func (r *taxRepository) Create(ctx context.Context, rate *models.TaxRate) error {
	rate.ID = primitive.NewObjectID()
	rate.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, rate)
	return err
}

func (r *taxRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.TaxRate, error) {
	var rate models.TaxRate
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&rate)
	if err != nil {
		return nil, err
	}
	return &rate, nil
}

func (r *taxRepository) GetByLocation(ctx context.Context, country, state, city, postalCode string) (*models.TaxRate, error) {
	// Try to find most specific match first
	filters := []bson.M{
		// Most specific: country + state + city + postal code
		{"country": country, "state": state, "city": city, "postal_code": postalCode, "is_active": true},
		// Country + state + city
		{"country": country, "state": state, "city": city, "is_active": true},
		// Country + state
		{"country": country, "state": state, "is_active": true},
		// Country only
		{"country": country, "is_active": true},
	}

	for _, filter := range filters {
		var rate models.TaxRate
		err := r.collection.FindOne(ctx, filter, options.FindOne().SetSort(bson.M{"created_at": -1})).Decode(&rate)
		if err == nil {
			return &rate, nil
		}
	}

	region := taxRegionFromCountry(country)
	switch region {
	case "US":
		// US sales tax is computed with Stripe Tax in OrderService; do not invent a default % here.
		return nil, mongo.ErrNoDocuments
	case "IN":
		gst := r.indiaDefaultGSTPercent
		if gst <= 0 {
			gst = 18
		}
		return &models.TaxRate{
			Rate:     gst,
			TaxType:  "gst",
			IsActive: true,
		}, nil
	default:
		return nil, mongo.ErrNoDocuments
	}
}

func (r *taxRepository) List(ctx context.Context, activeOnly bool) ([]*models.TaxRate, error) {
	filter := bson.M{}
	if activeOnly {
		filter["is_active"] = true
	}

	cursor, err := r.collection.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "country", Value: 1}, {Key: "state", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	rates := []*models.TaxRate{}
	if err := cursor.All(ctx, &rates); err != nil {
		return nil, err
	}
	return rates, nil
}

func (r *taxRepository) Update(ctx context.Context, id primitive.ObjectID, rate *models.TaxRate) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": rate},
	)
	return err
}

func (r *taxRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
