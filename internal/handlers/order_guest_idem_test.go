package handlers

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGuestIdempotencyNamespace(t *testing.T) {
	a := guestIdempotencyNamespace("Alice@Example.com")
	aNorm := guestIdempotencyNamespace("  alice@example.com ")
	b := guestIdempotencyNamespace("bob@example.com")

	if a != aNorm {
		t.Fatalf("namespace must be case/whitespace-insensitive: %v != %v", a, aNorm)
	}
	if a == b {
		t.Fatal("different guest emails must map to different namespaces (no cross-guest collision)")
	}
	if a == primitive.NilObjectID || b == primitive.NilObjectID {
		t.Fatal("guest namespace must not be the shared Nil namespace")
	}
}
