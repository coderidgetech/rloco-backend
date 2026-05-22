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
)

// DemoProduct mirrors models.Product fields needed for demo seeding.
type DemoProduct struct {
	ID               primitive.ObjectID  `bson:"_id"`
	Name             string              `bson:"name"`
	SKU              string              `bson:"sku"`
	Price            float64             `bson:"price"`
	OriginalPrice    *float64            `bson:"original_price,omitempty"`
	Images           []string            `bson:"images"`
	Category         string              `bson:"category"`
	Subcategory      string              `bson:"subcategory,omitempty"`
	Gender           string              `bson:"gender"`
	Colors           []string            `bson:"colors"`
	Color            string              `bson:"color,omitempty"`
	Sizes            []string            `bson:"sizes"`
	Description      string              `bson:"description"`
	Details          []string            `bson:"details"`
	Material         string              `bson:"material"`
	Care             string              `bson:"care,omitempty"`
	Featured         bool                `bson:"featured"`
	NewArrival       bool                `bson:"new_arrival"`
	OnSale           bool                `bson:"on_sale"`
	IsGift           bool                `bson:"is_gift"`
	Rating           float64             `bson:"rating"`
	Reviews          int                 `bson:"reviews"`
	Badge            *string             `bson:"badge,omitempty"`
	Stock            map[string]int      `bson:"stock"`
	VariantGroupID   *primitive.ObjectID `bson:"variant_group_id,omitempty"`
	IsMainVariant    bool                `bson:"is_main_variant,omitempty"`
	AvailableMarkets []string            `bson:"available_markets,omitempty"`
	CreatedAt        time.Time           `bson:"created_at"`
	UpdatedAt        time.Time           `bson:"updated_at"`
}

func badgePtr(s string) *string  { return &s }
func origPrice(f float64) *float64 { return &f }

func main() {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://admin:password@localhost:27017/rloco?authSource=admin"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("connect:", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	col := client.Database("rloco").Collection("products")

	if _, err := col.DeleteMany(ctx, bson.M{}); err != nil {
		log.Fatal("clear products:", err)
	}
	log.Println("Cleared products collection")

	now := time.Now()
	markets := []string{"IN", "US"}

	womenSz := []string{"XS", "S", "M", "L", "XL"}
	menSz   := []string{"S", "M", "L", "XL", "XXL"}
	oneSz   := []string{"One Size"}
	shoeSz  := []string{"36", "37", "38", "39", "40", "41"}

	womenStk := map[string]int{"XS": 8, "S": 15, "M": 20, "L": 15, "XL": 10}
	menStk   := map[string]int{"S": 10, "M": 18, "L": 18, "XL": 12, "XXL": 6}
	oneStk   := map[string]int{"One Size": 30}
	shoeStk  := map[string]int{"36": 4, "37": 6, "38": 8, "39": 8, "40": 5, "41": 3}
	lowStk   := map[string]int{"XS": 2, "S": 4, "M": 6, "L": 4, "XL": 2}
	soldOut  := map[string]int{"XS": 0, "S": 0, "M": 3, "L": 5, "XL": 4}

	// Shared variant group IDs
	gDress      := primitive.NewObjectID()
	gSweater    := primitive.NewObjectID()
	gTrousers   := primitive.NewObjectID()
	gBag        := primitive.NewObjectID()
	gShirt      := primitive.NewObjectID()
	gLinenPants := primitive.NewObjectID()

	slipDressDesc := "A fluid bias-cut slip dress in luxurious charmeuse. Elegant drape, adjustable spaghetti straps, and a subtle side slit that hints at the leg without revealing too much."
	slipDressDetails := []string{"Adjustable spaghetti straps", "Side slit hem", "Concealed zip closure", "Fully lined"}

	cashmereDesc := "Cloud-soft cashmere crewneck in a relaxed oversized silhouette. Drop shoulders, ribbed trims, and a cozy feel that carries effortlessly from morning to evening."
	cashmereDetails := []string{"Relaxed oversized fit", "Drop shoulders", "Ribbed neck, cuffs and hem", "Ethically sourced Grade A cashmere"}

	trouserDesc := "Impeccably tailored wide-leg trousers with a high rise and fluid drape. A wardrobe anchor that pairs with everything from a silk blouse to a simple tee."
	trouserDetails := []string{"High-rise waist", "Wide leg silhouette", "Side welt pockets", "Back zip closure", "Fully lined"}

	bagDesc := "A classic structured top-handle bag in pebbled leather. Spacious interior, gold-toned hardware, and a detachable crossbody strap for all-day versatility."
	bagDetails := []string{"Pebbled full-grain leather", "Gold-toned hardware", "Detachable shoulder strap", "Interior zip pocket + two slip pockets", "Dust bag included"}

	shirtDesc := "A refined Oxford shirt in crisp Supima cotton. Classic button-down collar and a versatile fit that works tucked or untucked, office or weekend."
	shirtDetails := []string{"Button-down collar", "Single chest pocket", "Regular fit", "Machine washable"}

	linenDesc := "Breezy linen wide-leg pants with a relaxed high-rise waist and side pockets. The easy warm-weather trouser."
	linenDetails := []string{"High-rise waist with elastic back", "Wide leg", "Side pockets", "Raw hem finish"}

	products := []DemoProduct{
		// ──────────────────────────────────────────────────────────────────────
		// GROUP 1 — Bias-Cut Slip Dress (3 colors)
		// ──────────────────────────────────────────────────────────────────────
		{
			ID: primitive.NewObjectID(), Name: "Bias-Cut Slip Dress", SKU: "DEMO-DRESS-BLK",
			Price: 189.00,
			Images: []string{
				"https://images.unsplash.com/photo-1595777457583-95e059d581b8?w=800&q=80",
				"https://images.unsplash.com/photo-1509631179647-0177331693ae?w=800&q=80",
			},
			Category: "Dresses", Subcategory: "Midi Dresses", Gender: "women",
			Colors: []string{"Black"}, Color: "Black",
			Sizes: womenSz, Description: slipDressDesc, Details: slipDressDetails,
			Material: "100% Charmeuse Silk", Care: "Dry clean only",
			Featured: true, NewArrival: true, OnSale: false, IsGift: true,
			Rating: 4.8, Reviews: 47, Badge: badgePtr("Trending"),
			Stock: womenStk, VariantGroupID: &gDress, IsMainVariant: true,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Bias-Cut Slip Dress", SKU: "DEMO-DRESS-CHP",
			Price: 189.00,
			Images: []string{"https://images.unsplash.com/photo-1572804013309-59a88b7e92f1?w=800&q=80"},
			Category: "Dresses", Subcategory: "Midi Dresses", Gender: "women",
			Colors: []string{"Champagne"}, Color: "Champagne",
			Sizes: womenSz, Description: slipDressDesc, Details: slipDressDetails,
			Material: "100% Charmeuse Silk", Care: "Dry clean only",
			Featured: false, NewArrival: true, IsGift: true,
			Rating: 4.7, Reviews: 23,
			Stock: womenStk, VariantGroupID: &gDress, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Bias-Cut Slip Dress", SKU: "DEMO-DRESS-RSE",
			Price: 189.00,
			Images: []string{"https://images.unsplash.com/photo-1566479179817-c0a5b2a9e9d4?w=800&q=80"},
			Category: "Dresses", Subcategory: "Midi Dresses", Gender: "women",
			Colors: []string{"Dusty Rose"}, Color: "Dusty Rose",
			Sizes: womenSz, Description: slipDressDesc, Details: slipDressDetails,
			Material: "100% Charmeuse Silk", Care: "Dry clean only",
			Featured: false, IsGift: true,
			Rating: 4.6, Reviews: 18,
			Stock: lowStk, VariantGroupID: &gDress, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},

		// ──────────────────────────────────────────────────────────────────────
		// GROUP 2 — Oversized Cashmere Crewneck (4 colors, one sold-out in XS/S)
		// ──────────────────────────────────────────────────────────────────────
		{
			ID: primitive.NewObjectID(), Name: "Oversized Cashmere Crewneck", SKU: "DEMO-SWT-OAT",
			Price: 165.00,
			Images: []string{
				"https://images.unsplash.com/photo-1576566588028-4147f3842f27?w=800&q=80",
				"https://images.unsplash.com/photo-1620799140408-edc6dcb6d633?w=800&q=80",
			},
			Category: "Tops", Subcategory: "Sweaters", Gender: "women",
			Colors: []string{"Oatmeal"}, Color: "Oatmeal",
			Sizes: womenSz, Description: cashmereDesc, Details: cashmereDetails,
			Material: "100% Grade A Cashmere", Care: "Hand wash cold, lay flat to dry",
			Featured: true, OnSale: false, IsGift: true,
			Rating: 4.9, Reviews: 83, Badge: badgePtr("Best Seller"),
			Stock: womenStk, VariantGroupID: &gSweater, IsMainVariant: true,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Oversized Cashmere Crewneck", SKU: "DEMO-SWT-MID",
			Price: 165.00,
			Images: []string{"https://images.unsplash.com/photo-1578587018452-892bacefd3f2?w=800&q=80"},
			Category: "Tops", Subcategory: "Sweaters", Gender: "women",
			Colors: []string{"Midnight"}, Color: "Midnight",
			Sizes: womenSz, Description: cashmereDesc, Details: cashmereDetails,
			Material: "100% Grade A Cashmere", Care: "Hand wash cold, lay flat to dry",
			Featured: false, Rating: 4.8, Reviews: 41,
			Stock: womenStk, VariantGroupID: &gSweater, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Oversized Cashmere Crewneck", SKU: "DEMO-SWT-MVE",
			Price: 165.00,
			Images: []string{"https://images.unsplash.com/photo-1558769132-cb1aea458c5e?w=800&q=80"},
			Category: "Tops", Subcategory: "Sweaters", Gender: "women",
			Colors: []string{"Dusty Mauve"}, Color: "Dusty Mauve",
			Sizes: womenSz, Description: cashmereDesc, Details: cashmereDetails,
			Material: "100% Grade A Cashmere", Care: "Hand wash cold, lay flat to dry",
			NewArrival: true, Rating: 4.7, Reviews: 29,
			Stock: lowStk, VariantGroupID: &gSweater, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			// XS + S are sold out — demonstrates the slash swatch on product page
			ID: primitive.NewObjectID(), Name: "Oversized Cashmere Crewneck", SKU: "DEMO-SWT-SAG",
			Price: 165.00,
			Images: []string{"https://images.unsplash.com/photo-1491336477066-31156b5e4f35?w=800&q=80"},
			Category: "Tops", Subcategory: "Sweaters", Gender: "women",
			Colors: []string{"Sage"}, Color: "Sage",
			Sizes: womenSz, Description: cashmereDesc, Details: cashmereDetails,
			Material: "100% Grade A Cashmere", Care: "Hand wash cold, lay flat to dry",
			Rating: 4.6, Reviews: 17,
			Stock: soldOut, VariantGroupID: &gSweater, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},

		// ──────────────────────────────────────────────────────────────────────
		// GROUP 3 — Tailored Wide-Leg Trousers (3 colors)
		// ──────────────────────────────────────────────────────────────────────
		{
			ID: primitive.NewObjectID(), Name: "Tailored Wide-Leg Trousers", SKU: "DEMO-TROU-CHR",
			Price: 129.00,
			Images: []string{
				"https://images.unsplash.com/photo-1542272604-787c3835535d?w=800&q=80",
				"https://images.unsplash.com/photo-1594938298603-c8148c4b4545?w=800&q=80",
			},
			Category: "Bottoms", Subcategory: "Trousers", Gender: "women",
			Colors: []string{"Charcoal"}, Color: "Charcoal",
			Sizes: womenSz, Description: trouserDesc, Details: trouserDetails,
			Material: "72% Polyester, 28% Viscose", Care: "Dry clean recommended",
			Featured: true, NewArrival: true,
			Rating: 4.7, Reviews: 62,
			Stock: womenStk, VariantGroupID: &gTrousers, IsMainVariant: true,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Tailored Wide-Leg Trousers", SKU: "DEMO-TROU-IVY",
			Price: 129.00,
			Images: []string{"https://images.unsplash.com/photo-1624378439575-d8705ad7ae80?w=800&q=80"},
			Category: "Bottoms", Subcategory: "Trousers", Gender: "women",
			Colors: []string{"Ivory"}, Color: "Ivory",
			Sizes: womenSz, Description: trouserDesc, Details: trouserDetails,
			Material: "72% Polyester, 28% Viscose", Care: "Dry clean recommended",
			NewArrival: true, Rating: 4.5, Reviews: 34,
			Stock: womenStk, VariantGroupID: &gTrousers, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Tailored Wide-Leg Trousers", SKU: "DEMO-TROU-CML",
			Price: 129.00,
			Images: []string{"https://images.unsplash.com/photo-1552902865-b72c031ac5ea?w=800&q=80"},
			Category: "Bottoms", Subcategory: "Trousers", Gender: "women",
			Colors: []string{"Camel"}, Color: "Camel",
			Sizes: womenSz, Description: trouserDesc, Details: trouserDetails,
			Material: "72% Polyester, 28% Viscose", Care: "Dry clean recommended",
			Rating: 4.6, Reviews: 28,
			Stock: map[string]int{"XS": 5, "S": 8, "M": 10, "L": 8, "XL": 5},
			VariantGroupID: &gTrousers, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},

		// ──────────────────────────────────────────────────────────────────────
		// GROUP 4 — Structured Top-Handle Bag (3 colors)
		// ──────────────────────────────────────────────────────────────────────
		{
			ID: primitive.NewObjectID(), Name: "Structured Top-Handle Bag", SKU: "DEMO-BAG-BLK",
			Price: 285.00,
			Images: []string{
				"https://images.unsplash.com/photo-1584917865442-de89df76afd3?w=800&q=80",
				"https://images.unsplash.com/photo-1548036328-c9fa89d128fa?w=800&q=80",
			},
			Category: "Bags", Subcategory: "Top-Handle", Gender: "women",
			Colors: []string{"Black"}, Color: "Black",
			Sizes: oneSz, Description: bagDesc, Details: bagDetails,
			Material: "Full-Grain Leather", Care: "Wipe with damp cloth; leather conditioner monthly",
			Featured: true, IsGift: true,
			Rating: 4.9, Reviews: 118, Badge: badgePtr("Best Seller"),
			Stock: oneStk, VariantGroupID: &gBag, IsMainVariant: true,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Structured Top-Handle Bag", SKU: "DEMO-BAG-TAN",
			Price: 285.00,
			Images: []string{"https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=800&q=80"},
			Category: "Bags", Subcategory: "Top-Handle", Gender: "women",
			Colors: []string{"Tan"}, Color: "Tan",
			Sizes: oneSz, Description: bagDesc, Details: bagDetails,
			Material: "Full-Grain Leather", Care: "Wipe with damp cloth; leather conditioner monthly",
			Featured: false, IsGift: true,
			Rating: 4.8, Reviews: 72,
			Stock: oneStk, VariantGroupID: &gBag, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Structured Top-Handle Bag", SKU: "DEMO-BAG-CGN",
			Price: 285.00,
			Images: []string{"https://images.unsplash.com/photo-1491637639811-60e2756cc1c7?w=800&q=80"},
			Category: "Bags", Subcategory: "Top-Handle", Gender: "women",
			Colors: []string{"Cognac"}, Color: "Cognac",
			Sizes: oneSz, Description: bagDesc, Details: bagDetails,
			Material: "Full-Grain Leather", Care: "Wipe with damp cloth; leather conditioner monthly",
			NewArrival: true, IsGift: true,
			Rating: 4.8, Reviews: 39,
			Stock: map[string]int{"One Size": 12},
			VariantGroupID: &gBag, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},

		// ──────────────────────────────────────────────────────────────────────
		// GROUP 5 — Oxford Button-Down Shirt (3 colors)
		// ──────────────────────────────────────────────────────────────────────
		{
			ID: primitive.NewObjectID(), Name: "Oxford Button-Down Shirt", SKU: "DEMO-SHRT-WHT",
			Price: 89.00,
			Images: []string{
				"https://images.unsplash.com/photo-1596755094514-f87e34085b2c?w=800&q=80",
				"https://images.unsplash.com/photo-1620012253295-c15cc3e65df4?w=800&q=80",
			},
			Category: "Tops", Subcategory: "Shirts", Gender: "men",
			Colors: []string{"White"}, Color: "White",
			Sizes: menSz, Description: shirtDesc, Details: shirtDetails,
			Material: "100% Supima Cotton", Care: "Machine wash cold, tumble dry low",
			Featured: true, NewArrival: true,
			Rating: 4.6, Reviews: 54, Badge: badgePtr("New"),
			Stock: menStk, VariantGroupID: &gShirt, IsMainVariant: true,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Oxford Button-Down Shirt", SKU: "DEMO-SHRT-NVY",
			Price: 89.00,
			Images: []string{"https://images.unsplash.com/photo-1602810319428-019690571b5b?w=800&q=80"},
			Category: "Tops", Subcategory: "Shirts", Gender: "men",
			Colors: []string{"Navy"}, Color: "Navy",
			Sizes: menSz, Description: shirtDesc, Details: shirtDetails,
			Material: "100% Supima Cotton", Care: "Machine wash cold, tumble dry low",
			NewArrival: true, Rating: 4.5, Reviews: 38,
			Stock: menStk, VariantGroupID: &gShirt, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Oxford Button-Down Shirt", SKU: "DEMO-SHRT-SAG",
			Price: 89.00,
			Images: []string{"https://images.unsplash.com/photo-1598961942613-ba897716405b?w=800&q=80"},
			Category: "Tops", Subcategory: "Shirts", Gender: "men",
			Colors: []string{"Sage"}, Color: "Sage",
			Sizes: menSz, Description: shirtDesc, Details: shirtDetails,
			Material: "100% Supima Cotton", Care: "Machine wash cold, tumble dry low",
			Rating: 4.4, Reviews: 21,
			Stock: map[string]int{"S": 6, "M": 10, "L": 12, "XL": 8, "XXL": 3},
			VariantGroupID: &gShirt, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},

		// ──────────────────────────────────────────────────────────────────────
		// GROUP 6 — Linen Wide-Leg Pants (3 colors, on sale)
		// ──────────────────────────────────────────────────────────────────────
		{
			ID: primitive.NewObjectID(), Name: "Linen Wide-Leg Pants", SKU: "DEMO-LIN-NAT",
			Price: 79.00, OriginalPrice: origPrice(129.00),
			Images: []string{"https://images.unsplash.com/photo-1624378439575-d8705ad7ae80?w=800&q=80"},
			Category: "Bottoms", Subcategory: "Pants", Gender: "women",
			Colors: []string{"Natural"}, Color: "Natural",
			Sizes: womenSz, Description: linenDesc, Details: linenDetails,
			Material: "100% Belgian Linen", Care: "Machine wash cold, tumble dry low",
			OnSale: true, Rating: 4.5, Reviews: 44, Badge: badgePtr("Hot"),
			Stock: womenStk, VariantGroupID: &gLinenPants, IsMainVariant: true,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Linen Wide-Leg Pants", SKU: "DEMO-LIN-BLK",
			Price: 79.00, OriginalPrice: origPrice(129.00),
			Images: []string{"https://images.unsplash.com/photo-1542272604-787c3835535d?w=800&q=80"},
			Category: "Bottoms", Subcategory: "Pants", Gender: "women",
			Colors: []string{"Black"}, Color: "Black",
			Sizes: womenSz, Description: linenDesc, Details: linenDetails,
			Material: "100% Belgian Linen", Care: "Machine wash cold, tumble dry low",
			OnSale: true, Rating: 4.6, Reviews: 58,
			Stock: womenStk, VariantGroupID: &gLinenPants, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Linen Wide-Leg Pants", SKU: "DEMO-LIN-TER",
			Price: 79.00, OriginalPrice: origPrice(129.00),
			Images: []string{"https://images.unsplash.com/photo-1475180098004-ca77a66827be?w=800&q=80"},
			Category: "Bottoms", Subcategory: "Pants", Gender: "women",
			Colors: []string{"Terracotta"}, Color: "Terracotta",
			Sizes: womenSz, Description: linenDesc, Details: linenDetails,
			Material: "100% Belgian Linen", Care: "Machine wash cold, tumble dry low",
			OnSale: true, Rating: 4.4, Reviews: 31,
			Stock: map[string]int{"XS": 4, "S": 7, "M": 9, "L": 7, "XL": 4},
			VariantGroupID: &gLinenPants, IsMainVariant: false,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},

		// ──────────────────────────────────────────────────────────────────────
		// STANDALONE — No variant group
		// ──────────────────────────────────────────────────────────────────────
		{
			ID: primitive.NewObjectID(), Name: "Gold Link Chain Necklace", SKU: "DEMO-JWL-001",
			Price: 75.00,
			Images: []string{"https://images.unsplash.com/photo-1515562141207-7a88fb7ce338?w=800&q=80"},
			Category: "Jewelry", Subcategory: "Necklaces", Gender: "women",
			Colors: []string{"Gold"}, Sizes: oneSz,
			Description: "A refined 18k gold-plated link chain necklace at a versatile length that layers beautifully.",
			Details: []string{"18k gold plated", "Adjustable 40–45 cm", "Lobster claw clasp", "Tarnish resistant"},
			Material: "Gold-Plated Brass",
			Featured: true, NewArrival: true, IsGift: true,
			Rating: 4.7, Reviews: 93, Badge: badgePtr("Limited Edition"),
			Stock: oneStk,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Suede Pointed-Toe Mules", SKU: "DEMO-SHOE-001",
			Price: 195.00, OriginalPrice: origPrice(249.00),
			Images: []string{"https://images.unsplash.com/photo-1543163521-1bf539c55dd2?w=800&q=80"},
			Category: "Footwear", Subcategory: "Mules", Gender: "women",
			Colors: []string{"Nude", "Black"},
			Sizes: shoeSz,
			Description: "Effortlessly chic pointed-toe mules in buttery suede. A wardrobe staple that elevates any outfit.",
			Details: []string{"Genuine suede upper", "Leather insole", "3 cm block heel", "Pointed toe"},
			Material: "Genuine Suede",
			Featured: true, OnSale: true, IsGift: true,
			Rating: 4.8, Reviews: 76,
			Stock: shoeStk,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Merino Ribbed Beanie", SKU: "DEMO-ACC-001",
			Price: 45.00,
			Images: []string{"https://images.unsplash.com/photo-1520903920243-00d872a2d1c9?w=800&q=80"},
			Category: "Accessories", Subcategory: "Hats", Gender: "unisex",
			Colors: []string{"Charcoal", "Camel", "Ivory", "Burgundy"},
			Sizes: oneSz,
			Description: "Fine-knit merino wool beanie with a classic ribbed texture. Warm, soft, and sustainably made.",
			Details: []string{"Fine gauge ribbed knit", "No-itch merino", "Sustainably sourced wool"},
			Material: "100% Merino Wool",
			NewArrival: true, IsGift: true,
			Rating: 4.6, Reviews: 38,
			Stock: oneStk,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Double-Breasted Wool Blazer", SKU: "DEMO-BLZ-001",
			Price: 235.00,
			Images: []string{"https://images.unsplash.com/photo-1539533018447-63fcce2678e3?w=800&q=80"},
			Category: "Outerwear", Subcategory: "Blazers", Gender: "men",
			Colors: []string{"Navy", "Charcoal"},
			Sizes: menSz,
			Description: "A sharp double-breasted blazer in a refined wool blend. Structured shoulders and a tailored silhouette.",
			Details: []string{"Fully lined", "Structured shoulders", "Flap pockets", "Interior breast pocket"},
			Material: "70% Wool, 30% Polyester",
			Featured: true,
			Rating: 4.7, Reviews: 29,
			Stock: menStk,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Floral Satin Midi Skirt", SKU: "DEMO-SKT-001",
			Price: 115.00,
			Images: []string{"https://images.unsplash.com/photo-1515886657613-9f3515b0c78f?w=800&q=80"},
			Category: "Bottoms", Subcategory: "Skirts", Gender: "women",
			Colors: []string{"Blush Multi", "Navy Multi"},
			Sizes: womenSz,
			Description: "A feminine floral midi skirt in lustrous satin. Bias-cut for movement, elasticated waist for comfort.",
			Details: []string{"Elasticated waist", "Bias cut", "Midi length", "Fully lined"},
			Material: "100% Polyester Satin",
			Featured: true, NewArrival: true, IsGift: true,
			Rating: 4.5, Reviews: 47,
			Stock: womenStk,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Name: "Slim-Fit Chino Trousers", SKU: "DEMO-CHN-001",
			Price: 95.00,
			Images: []string{"https://images.unsplash.com/photo-1552902865-b72c031ac5ea?w=800&q=80"},
			Category: "Bottoms", Subcategory: "Chinos", Gender: "men",
			Colors: []string{"Khaki", "Navy", "Olive"},
			Sizes: menSz,
			Description: "Classic slim-fit chinos in a durable stretch cotton blend. Smart-casual versatility from desk to weekend.",
			Details: []string{"Slim fit", "Stretch cotton", "Belt loops", "Side and back pockets"},
			Material: "97% Cotton, 3% Elastane",
			NewArrival: true,
			Rating: 4.5, Reviews: 64,
			Stock: menStk,
			AvailableMarkets: markets, CreatedAt: now, UpdatedAt: now,
		},
	}

	docs := make([]interface{}, len(products))
	for i := range products {
		docs[i] = products[i]
	}

	res, err := col.InsertMany(ctx, docs)
	if err != nil {
		log.Fatal("insert:", err)
	}

	// Print a summary grouped by variant group
	log.Printf("Inserted %d products:", len(res.InsertedIDs))
	log.Println("  6 variant groups:")
	log.Println("    • Bias-Cut Slip Dress        — Black | Champagne | Dusty Rose")
	log.Println("    • Oversized Cashmere Crewneck — Oatmeal | Midnight | Dusty Mauve | Sage (XS+S sold out)")
	log.Println("    • Tailored Wide-Leg Trousers  — Charcoal | Ivory | Camel")
	log.Println("    • Structured Top-Handle Bag   — Black | Tan | Cognac")
	log.Println("    • Oxford Button-Down Shirt    — White | Navy | Sage")
	log.Println("    • Linen Wide-Leg Pants        — Natural | Black | Terracotta (on sale)")
	log.Println("  6 standalone products (no variants):")
	log.Println("    • Gold Link Chain Necklace, Suede Mules, Merino Beanie,")
	log.Println("      Wool Blazer, Floral Satin Skirt, Slim-Fit Chinos")

	// Ensure indexes for variant queries
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "variant_group_id", Value: 1},
			{Key: "is_main_variant", Value: 1},
		},
	}
	if _, err := col.Indexes().CreateOne(ctx, indexModel); err != nil {
		log.Printf("warn: index creation: %v", err)
	}

	log.Println("Done. Run the app and browse http://localhost:3000 to see variants in action.")
}
