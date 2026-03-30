// One-time deterministic backfill: products missing available_markets get US for first half (by _id asc), IN for second half.
// Idempotent: skips documents that already have non-empty available_markets.
//
// Usage: MONGODB_URI=... go run ./cmd/backfill-markets [--dry-run]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	dry := flag.Bool("dry-run", false, "print summary only, no writes")
	flag.Parse()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://admin:password@localhost:27017/rloco?authSource=admin"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	coll := client.Database("rloco").Collection("products")

	filter := bson.M{"$or": []bson.M{
		{"available_markets": bson.M{"$exists": false}},
		{"available_markets": bson.M{"$size": 0}},
	}}

	cur, err := coll.Find(ctx, filter, options.Find().SetSort(bson.M{"_id": 1}))
	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)

	var ids []primitive.ObjectID
	for cur.Next(ctx) {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cur.Decode(&doc); err != nil {
			log.Fatal(err)
		}
		ids = append(ids, doc.ID)
	}
	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	n := len(ids)
	if n == 0 {
		fmt.Println("backfill-markets: no products need assignment")
		return
	}

	half := n / 2
	usCount, inCount := 0, 0
	for i := range ids {
		if i < half {
			usCount++
		} else {
			inCount++
		}
	}

	fmt.Printf("backfill-markets: candidates=%d -> US=%d IN=%d (dry-run=%v)\n", n, usCount, inCount, *dry)

	if *dry {
		return
	}

	var usUpdated, inUpdated int64
	for i, id := range ids {
		markets := []string{"US"}
		if i >= half {
			markets = []string{"IN"}
		}
		res, err := coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
			"available_markets": markets,
			"updated_at":      time.Now(),
		}})
		if err != nil {
			log.Fatalf("update %s: %v", id.Hex(), err)
		}
		if res.ModifiedCount > 0 {
			if i < half {
				usUpdated++
			} else {
				inUpdated++
			}
		}
	}
	fmt.Printf("backfill-markets: modified US=%d IN=%d\n", usUpdated, inUpdated)
}
