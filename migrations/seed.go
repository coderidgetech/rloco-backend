package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

type Product struct {
	ID               primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name             string              `bson:"name" json:"name"`
	SKU              string              `bson:"sku" json:"sku"`
	Price            float64             `bson:"price" json:"price"`
	OriginalPrice    *float64            `bson:"original_price,omitempty" json:"original_price,omitempty"`
	PriceINR         *float64            `bson:"price_inr,omitempty" json:"price_inr,omitempty"`
	OriginalPriceINR *float64            `bson:"original_price_inr,omitempty" json:"original_price_inr,omitempty"`
	Images           []string            `bson:"images" json:"images"`
	Category         string              `bson:"category" json:"category"`
	Subcategory      string              `bson:"subcategory" json:"subcategory"`
	Gender           string              `bson:"gender" json:"gender"`
	Colors           []string            `bson:"colors" json:"colors"`
	Sizes            []string            `bson:"sizes" json:"sizes"`
	Description      string              `bson:"description" json:"description"`
	Details          []string            `bson:"details" json:"details"`
	Material         string              `bson:"material" json:"material"`
	Featured         bool                `bson:"featured" json:"featured"`
	NewArrival       bool                `bson:"new_arrival" json:"new_arrival"`
	OnSale           bool                `bson:"on_sale" json:"on_sale"`
	Rating           float64             `bson:"rating" json:"rating"`
	Reviews          int                 `bson:"reviews" json:"reviews"`
	Badge            *string             `bson:"badge,omitempty" json:"badge,omitempty"`
	VideoURL         *string             `bson:"video_url,omitempty" json:"video_url,omitempty"`
	Stock            map[string]int      `bson:"stock" json:"stock"`
	VendorID         *primitive.ObjectID `bson:"vendor_id,omitempty" json:"vendor_id,omitempty"`
	CreatedAt        time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time           `bson:"updated_at" json:"updated_at"`
}

type User struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email          string             `bson:"email" json:"email"`
	PasswordHash   string             `bson:"password_hash" json:"-"`
	Name           string             `bson:"name" json:"name"`
	Role           string             `bson:"role" json:"role"`
	Active         bool               `bson:"active" json:"active"`
	EmailVerified  bool               `bson:"email_verified" json:"email_verified"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}

// InspirationVideo matches backend/internal/models.InspirationVideo for seeding
type InspirationVideo struct {
	ID            primitive.ObjectID `bson:"_id" json:"id"`
	Title         string             `bson:"title" json:"title"`
	VideoURL      string             `bson:"video_url" json:"video_url"`
	ThumbnailURL  string             `bson:"thumbnail_url" json:"thumbnail_url"`
	Category      string             `bson:"category" json:"category"`
	Featured      bool               `bson:"featured" json:"featured"`
	UploadedByName string            `bson:"uploaded_by_name,omitempty" json:"uploaded_by_name,omitempty"`
	IsActive      bool               `bson:"is_active" json:"is_active"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}

func main() {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://admin:password@localhost:27017/rloco?authSource=admin"
	}

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer connectCancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	// Do not use a 10s deadline for the whole run — slow Atlas/remote DB could skip work mid-seed.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	db := client.Database("rloco")

	// Create admin user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	adminUser := User{
		ID:            primitive.NewObjectID(),
		Email:         "admin@rloko.com",
		PasswordHash:  string(hashedPassword),
		Name:          "Admin User",
		Role:          "admin",
		Active:        true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	usersCollection := db.Collection("users")
	_, err = usersCollection.InsertOne(ctx, adminUser)
	if err != nil {
		log.Printf("Admin user may already exist: %v", err)
		// Old seeds omitted active; login requires active=true.
		if _, perr := usersCollection.UpdateOne(ctx, bson.M{"email": "admin@rloko.com"}, bson.M{"$set": bson.M{
			"active":         true,
			"email_verified": true,
			"updated_at":     time.Now(),
		}}); perr == nil {
			log.Println("Patched admin@rloko.com: active=true, email_verified=true (if document existed)")
		}
	} else {
		log.Println("Admin user created: admin@rloko.com / admin123")
	}

	// Upsert default categories by slug so re-running seed gives all four and sets images
	categoriesCollection := db.Collection("categories")
	defaultCategories := []struct {
		name          string
		slug          string
		gender        string
		image         string
		subcategories []string
		order         int
	}{
		{"Women", "women", "women", "https://images.unsplash.com/photo-1483985988355-763728e1935b?w=600&q=80", []string{"Dresses", "Tops", "Bottoms", "Shoes"}, 1},
		{"Men", "men", "men", "https://images.unsplash.com/photo-1617127365659-c47fa864d8bc?w=600&q=80", []string{"Shirts", "Trousers", "Outerwear", "Shoes"}, 2},
		{"Dresses", "dresses", "women", "https://images.unsplash.com/photo-1595777457583-95e059d581b8?w=600&q=80", []string{"Casual Dresses", "Formal Dresses"}, 3},
		{"Accessories", "accessories", "unisex", "https://images.unsplash.com/photo-1584917865442-de89df76afd3?w=600&q=80", []string{"Bags", "Watches", "Jewelry"}, 4},
	}
	now := time.Now()
	for _, c := range defaultCategories {
		update := bson.M{
			"$set": bson.M{
				"name":          c.name,
				"slug":          c.slug,
				"gender":        c.gender,
				"image":         c.image,
				"subcategories": c.subcategories,
				"order":         c.order,
				"updated_at":    now,
			},
			"$setOnInsert": bson.M{
				"_id":        primitive.NewObjectID(),
				"created_at": now,
			},
		}
		_, err = categoriesCollection.UpdateOne(ctx, bson.M{"slug": c.slug}, update, options.Update().SetUpsert(true))
		if err != nil {
			log.Printf("Category %s upsert failed: %v", c.slug, err)
		}
	}
	log.Println("Default categories upserted (Women, Men, Dresses, Accessories)")

	// Seed inspiration videos for homepage style/lookbook section (only if empty)
	videosCollection := db.Collection("videos")
	count, _ := videosCollection.CountDocuments(ctx, bson.M{})
	if count == 0 {
		now := time.Now()
		inspirationVideos := []InspirationVideo{
			{ID: primitive.NewObjectID(), Title: "Luxury Fashion Collection", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4", ThumbnailURL: "https://images.unsplash.com/photo-1483985988355-763728e1935b?w=800&q=80", Category: "Luxury", Featured: true, IsActive: true, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Title: "Street Style Essentials", VideoURL: "https://www.w3schools.com/html/movie.mp4", ThumbnailURL: "https://images.unsplash.com/photo-1515886657613-9f3515b0c78f?w=800&q=80", Category: "Street Style", Featured: false, IsActive: true, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Title: "Summer Elegance", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4", ThumbnailURL: "https://images.unsplash.com/photo-1509631179647-0177331693ae?w=800&q=80", Category: "Summer Collection", Featured: false, IsActive: true, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Title: "Urban Fashion", VideoURL: "https://www.w3schools.com/html/movie.mp4", ThumbnailURL: "https://images.unsplash.com/photo-1487222477894-8943e31ef7b2?w=800&q=80", Category: "Urban", Featured: false, IsActive: true, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Title: "Elegant Evening Wear", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4", ThumbnailURL: "https://images.unsplash.com/photo-1496747611176-843222e1e57c?w=800&q=80", Category: "Evening Wear", Featured: true, IsActive: true, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Title: "Casual Chic", VideoURL: "https://www.w3schools.com/html/movie.mp4", ThumbnailURL: "https://images.unsplash.com/photo-1515372039744-b8f02a3ae446?w=800&q=80", Category: "Casual Wear", Featured: false, IsActive: true, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Title: "Accessories Showcase", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4", ThumbnailURL: "https://images.unsplash.com/photo-1492707892479-7bc8d5a4ee93?w=800&q=80", Category: "Accessories", Featured: false, IsActive: true, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Title: "Modern Minimalist", VideoURL: "https://www.w3schools.com/html/movie.mp4", ThumbnailURL: "https://images.unsplash.com/photo-1490481651871-ab68de25d43d?w=800&q=80", Category: "Minimalist", Featured: false, IsActive: true, CreatedAt: now, UpdatedAt: now},
		}
		videoDocs := make([]interface{}, len(inspirationVideos))
		for i := range inspirationVideos {
			videoDocs[i] = inspirationVideos[i]
		}
		_, err = videosCollection.InsertMany(ctx, videoDocs)
		if err != nil {
			log.Printf("Failed to seed inspiration videos: %v", err)
		} else {
			log.Println("Inspiration videos seeded (8 videos)")
		}
	} else {
		log.Printf("Videos collection already has %d documents, skipping seed", count)
	}

	// Seed products for Top Collection, New Arrivals, On Sale (only if empty)
	productsCollection := db.Collection("products")
	productCount, _ := productsCollection.CountDocuments(ctx, bson.M{})
	if productCount == 0 {
		now := time.Now()
		orig399 := 399.99
		orig249 := 249.99
		orig79 := 79.99
		seedProducts := []Product{
			{ID: primitive.NewObjectID(), Name: "Elegant Evening Dress", SKU: "SEED-DRESS-001", Price: 299.99, OriginalPrice: &orig399, Images: []string{"https://images.unsplash.com/photo-1595777457583-95e059d581b8?w=800&q=80"}, Category: "Dresses", Subcategory: "Evening", Gender: "women", Colors: []string{"Black", "Navy"}, Sizes: []string{"S", "M", "L", "XL"}, Description: "A stunning evening dress for special occasions.", Material: "Silk Blend", Featured: true, NewArrival: true, OnSale: true, Rating: 4.8, Reviews: 24, Stock: map[string]int{"S": 10, "M": 15, "L": 12, "XL": 8}, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Name: "Classic White Blouse", SKU: "SEED-TOP-001", Price: 79.99, Images: []string{"https://images.unsplash.com/photo-1594633312681-425c7b97ccd1?w=800&q=80"}, Category: "Tops", Subcategory: "Blouses", Gender: "women", Colors: []string{"White", "Ivory"}, Sizes: []string{"S", "M", "L", "XL"}, Description: "Timeless white blouse for office or casual wear.", Material: "Cotton", Featured: true, NewArrival: false, OnSale: false, Rating: 4.6, Reviews: 18, Stock: map[string]int{"S": 10, "M": 15, "L": 12, "XL": 8}, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Name: "High-Waisted Trousers", SKU: "SEED-PANTS-001", Price: 129.99, Images: []string{"https://images.unsplash.com/photo-1542272604-787c3835535d?w=800&q=80"}, Category: "Bottoms", Subcategory: "Pants", Gender: "women", Colors: []string{"Black", "Navy"}, Sizes: []string{"S", "M", "L", "XL"}, Description: "Professional high-waisted trousers with perfect fit.", Material: "Wool Blend", Featured: true, NewArrival: true, OnSale: false, Rating: 4.7, Reviews: 32, Stock: map[string]int{"S": 10, "M": 15, "L": 12, "XL": 8}, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Name: "Leather Ankle Boots", SKU: "SEED-SHOE-001", Price: 179.99, Images: []string{"https://images.unsplash.com/photo-1543163521-1bf539c55dd2?w=800&q=80"}, Category: "Shoes", Subcategory: "Boots", Gender: "women", Colors: []string{"Black", "Brown"}, Sizes: []string{"6", "7", "8", "9", "10"}, Description: "Stylish leather ankle boots with comfortable heel.", Material: "Leather", Featured: true, NewArrival: false, OnSale: true, OriginalPrice: &orig249, Rating: 4.9, Reviews: 41, Stock: map[string]int{"6": 5, "7": 8, "8": 10, "9": 8, "10": 5}, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Name: "Classic Dress Shirt", SKU: "SEED-SHIRT-001", Price: 89.99, Images: []string{"https://images.unsplash.com/photo-1596755094514-f87e34085b2c?w=800&q=80"}, Category: "Shirts", Subcategory: "Dress Shirts", Gender: "men", Colors: []string{"White", "Blue", "Gray"}, Sizes: []string{"S", "M", "L", "XL", "XXL"}, Description: "Premium cotton dress shirt for office or formal occasions.", Material: "Cotton", Featured: true, NewArrival: true, OnSale: false, Rating: 4.5, Reviews: 28, Stock: map[string]int{"S": 10, "M": 15, "L": 12, "XL": 8, "XXL": 5}, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Name: "Wool Overcoat", SKU: "SEED-COAT-001", Price: 399.99, Images: []string{"https://images.unsplash.com/photo-1539533018447-63fcce2678e3?w=800&q=80"}, Category: "Outerwear", Subcategory: "Coats", Gender: "men", Colors: []string{"Black", "Navy", "Gray"}, Sizes: []string{"S", "M", "L", "XL", "XXL"}, Description: "Premium wool overcoat for winter.", Material: "Wool", Featured: true, NewArrival: false, OnSale: true, OriginalPrice: &orig399, Rating: 4.8, Reviews: 19, Stock: map[string]int{"S": 5, "M": 8, "L": 10, "XL": 6, "XXL": 4}, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Name: "Silk Camisole Top", SKU: "SEED-CAMI-001", Price: 59.99, OriginalPrice: &orig79, Images: []string{"https://images.unsplash.com/photo-1594633313593-bab3825d0caf?w=800&q=80"}, Category: "Tops", Subcategory: "Camisoles", Gender: "women", Colors: []string{"Black", "Nude", "White"}, Sizes: []string{"S", "M", "L"}, Description: "Luxurious silk camisole for layering or standalone wear.", Material: "Silk", Featured: false, NewArrival: true, OnSale: true, Rating: 4.4, Reviews: 15, Stock: map[string]int{"S": 8, "M": 12, "L": 10}, CreatedAt: now, UpdatedAt: now},
			{ID: primitive.NewObjectID(), Name: "Leather Dress Shoes", SKU: "SEED-SHOE-002", Price: 199.99, Images: []string{"https://images.unsplash.com/photo-1549298916-b41d501d3772?w=800&q=80"}, Category: "Shoes", Subcategory: "Dress Shoes", Gender: "men", Colors: []string{"Black", "Brown"}, Sizes: []string{"8", "9", "10", "11", "12"}, Description: "Premium leather dress shoes for formal occasions.", Material: "Leather", Featured: false, NewArrival: true, OnSale: false, Rating: 4.7, Reviews: 36, Stock: map[string]int{"8": 6, "9": 10, "10": 12, "11": 8, "12": 5}, CreatedAt: now, UpdatedAt: now},
		}
		productDocs := make([]interface{}, len(seedProducts))
		for i := range seedProducts {
			productDocs[i] = seedProducts[i]
		}
		_, err = productsCollection.InsertMany(ctx, productDocs)
		if err != nil {
			log.Printf("Failed to seed products: %v", err)
		} else {
			log.Println("Products seeded (8 products, 6 featured for Top Collection)")
		}
	} else {
		log.Printf("Products collection already has %d documents, skipping seed", productCount)
	}

	log.Println("Seed script completed!")
}