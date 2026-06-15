package services

import "testing"

func TestFulfillmentBlockReason(t *testing.T) {
	cases := []struct {
		name          string
		status        string
		paymentMethod string
		paymentStatus string
		wantBlocked   bool
	}{
		{"card paid processing → allowed", "processing", "card", "paid", false},
		{"card paid pending → allowed", "pending", "stripe", "paid", false},
		{"card unpaid pending → blocked", "pending", "card", "pending", true},
		{"card failed → blocked", "processing", "card", "failed", true},
		{"card refunding → blocked", "processing", "card", "refunding", true},
		{"COD unpaid pending → allowed (settles on delivery)", "pending", "cod", "pending", false},
		{"COD (cash_on_delivery) unpaid → allowed", "processing", "cash_on_delivery", "", false},
		{"already shipped → blocked", "shipped", "card", "paid", true},
		{"delivered → blocked", "delivered", "cod", "pending", true},
		{"cancelled → blocked", "cancelled", "card", "paid", true},
		{"returned → blocked", "returned", "card", "paid", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason := fulfillmentBlockReason(tc.status, tc.paymentMethod, tc.paymentStatus)
			if (reason != "") != tc.wantBlocked {
				t.Fatalf("status=%q method=%q payment=%q: got reason=%q, wantBlocked=%v",
					tc.status, tc.paymentMethod, tc.paymentStatus, reason, tc.wantBlocked)
			}
		})
	}
}
