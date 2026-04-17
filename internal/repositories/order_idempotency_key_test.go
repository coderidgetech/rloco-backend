package repositories

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestIdempotencyDocID_Deterministic(t *testing.T) {
	u := primitive.NewObjectID()
	k := "checkout-attempt-1"
	a := idempotencyDocID(u, k)
	b := idempotencyDocID(u, k)
	if a != b {
		t.Fatalf("expected stable idempotency key, got %q vs %q", a, b)
	}
	if idempotencyDocID(u, "other") == a {
		t.Fatal("different client keys should not collide")
	}
}
