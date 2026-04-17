package repositories

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewMongoDB(uri string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Configure connection pooling for scalability
	clientOptions := options.Client().ApplyURI(uri).
		SetMaxPoolSize(100). // Max connections in pool
		SetMinPoolSize(10).  // Min connections in pool
		SetMaxConnIdleTime(30 * time.Second).
		SetRetryWrites(true).
		SetRetryReads(true)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the database
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := client.Database("rloco")

	mongoDB := &MongoDB{
		Client:   client,
		Database: db,
	}

	// Create indexes for performance
	if err := mongoDB.CreateIndexes(ctx); err != nil {
		// Log error but don't fail - indexes can be created later
		// In production, you might want to fail here
	}

	return mongoDB, nil
}

func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.Client.Disconnect(ctx)
}

func (m *MongoDB) GetCollection(name string) *mongo.Collection {
	return m.Database.Collection(name)
}

// CreateIndexes creates database indexes for optimal query performance
func (m *MongoDB) CreateIndexes(ctx context.Context) error {
	// Products indexes
	products := m.GetCollection("products")
	products.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "name", Value: "text"}, {Key: "description", Value: "text"}},
	})
	products.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "category", Value: 1}, {Key: "gender", Value: 1}},
	})
	products.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "featured", Value: 1}, {Key: "created_at", Value: -1}},
	})
	products.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "new_arrival", Value: 1}, {Key: "created_at", Value: -1}},
	})
	products.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "on_sale", Value: 1}, {Key: "created_at", Value: -1}},
	})
	products.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "is_gift", Value: 1}, {Key: "gender", Value: 1}},
	})
	products.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "vendor_id", Value: 1}},
	})

	// Orders indexes
	orders := m.GetCollection("orders")
	orders.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "order_number", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	orders.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
	})
	orders.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}},
	})

	// Users indexes
	users := m.GetCollection("users")
	users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "role", Value: 1}},
	})
	users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "phone_key", Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true),
	})

	// Phone OTP challenges (registration, etc.)
	phoneOTP := m.GetCollection("phone_otp_challenges")
	phoneOTP.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "phone_key", Value: 1}, {Key: "purpose", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	// Carts indexes
	carts := m.GetCollection("carts")
	carts.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	// Wishlists indexes
	wishlists := m.GetCollection("wishlists")
	wishlists.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "product_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	wishlists.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
	})

	// Promotions indexes
	promotions := m.GetCollection("promotions")
	promotions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	promotions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "is_active", Value: 1}, {Key: "start_date", Value: 1}, {Key: "end_date", Value: 1}},
	})

	// Categories indexes
	categories := m.GetCollection("categories")
	categories.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	categories.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "gender", Value: 1}, {Key: "order", Value: 1}},
	})

	// Product Reviews indexes
	reviews := m.GetCollection("product_reviews")
	reviews.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "product_id", Value: 1}, {Key: "status", Value: 1}, {Key: "created_at", Value: -1}},
	})
	reviews.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "product_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	reviews.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}},
	})

	// Returns indexes
	returns := m.GetCollection("returns")
	returns.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "order_id", Value: 1}, {Key: "created_at", Value: -1}},
	})
	returns.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
	})
	returns.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "order_number", Value: 1}},
	})
	returns.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}},
	})

	// Shipping Methods indexes
	shippingMethods := m.GetCollection("shipping_methods")
	shippingMethods.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "is_active", Value: 1}},
	})
	shippingMethods.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "zones.countries", Value: 1}},
	})

	// Tax Rates indexes
	taxRates := m.GetCollection("tax_rates")
	taxRates.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "country", Value: 1}, {Key: "state", Value: 1}, {Key: "is_active", Value: 1}},
	})
	taxRates.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "country", Value: 1}, {Key: "is_active", Value: 1}},
	})

	// Support Tickets indexes
	supportTickets := m.GetCollection("support_tickets")
	supportTickets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
	})
	supportTickets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "priority", Value: 1}, {Key: "created_at", Value: -1}},
	})
	supportTickets.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "assigned_to", Value: 1}},
	})

	// Payment Transactions indexes
	paymentTransactions := m.GetCollection("payment_transactions")
	paymentTransactions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "order_id", Value: 1}, {Key: "created_at", Value: -1}},
	})
	paymentTransactions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
	})
	paymentTransactions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "gateway_transaction_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	paymentTransactions.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}},
	})

	// Stripe webhook idempotency (event IDs); TTL avoids unbounded growth
	stripeWebhookEvents := m.GetCollection("stripe_webhook_events")
	stripeWebhookEvents.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(7 * 24 * 3600),
	})

	// Order idempotency keys (checkout retries)
	orderIdem := m.GetCollection("order_idempotency")
	orderIdem.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(48 * 3600),
	})

	return nil
}
