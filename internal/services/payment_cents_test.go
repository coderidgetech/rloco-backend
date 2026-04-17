package services

import (
	"math"
	"testing"
)

func TestStripeCentsRounding(t *testing.T) {
	// Mirrors createStripePaymentIntent amount → cents conversion.
	got := int64(math.Round(19.99 * 100))
	if got != 1999 {
		t.Fatalf("expected 1999 cents, got %d", got)
	}
}
