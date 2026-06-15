package services

import (
	"html"
	"os"
	"strings"
	"testing"

	"rloco-backend/internal/models"
)

func sampleOrder() *models.Order {
	return &models.Order{
		OrderNumber: "RLK-100245",
		ShippingInfo: models.ShippingInfo{
			FirstName: "Asha", LastName: "Rao", Email: "asha@example.com",
			Address: "12 MG Road, Indiranagar", City: "Bengaluru", State: "KA",
			ZipCode: "560038", Country: "India",
		},
		Items: []models.OrderItem{
			{ProductName: "Silk Wrap Dress", Size: "M", Quantity: 1, Price: 4999, IsGift: true},
			{ProductName: "Leather Tote Bag", Quantity: 2, Price: 2500},
		},
		Subtotal: 9999, Discount: 500, ShippingCost: 0, Tax: 250, Total: 9749,
		TrackingNumber: strPtr("SHIPPO_TRANSIT_99887766"),
	}
}

func strPtr(s string) *string { return &s }

// renderConfirmation mirrors SendOrderConfirmation's body assembly for inspection.
func renderConfirmation(order *models.Order) string {
	on := html.EscapeString(order.OrderNumber)
	inner := emailGreeting() + emailP("Thanks for your order — <strong>"+on+"</strong> is confirmed. Here's a summary:")
	body := emailTextBlockRow(inner) + emailItemsTable(order) + emailTotalsTable(order) +
		emailAddressBlock(order.ShippingInfo) + emailButtonRow("https://dev.rloko.com/orders/x", "View your order") +
		emailMutedNote("We'll email you again as soon as your order ships.")
	return rlokoEmailShell("Order confirmed — R-Loko", "Thank you for your order", body)
}

func renderStatus(order *models.Order, status string) string {
	c := statusEmailCopyFor(status)
	inner := emailGreeting() + emailP("An update on your order:") + emailP(html.EscapeString(c.intro))
	body := emailTextBlockRow(inner)
	if c.showTracking && order.TrackingNumber != nil {
		body += emailHighlightBox("Tracking number", html.EscapeString(*order.TrackingNumber))
	}
	body += emailButtonRow("https://dev.rloko.com/orders/x", "View your order")
	if c.note != "" {
		body += emailMutedNote(c.note)
	}
	return rlokoEmailShell(c.title+" — R-Loko", c.title, body)
}

func TestEmailTemplatesRender(t *testing.T) {
	order := sampleOrder()

	conf := renderConfirmation(order)
	// Content assertions — proves the email is informative, not generic.
	for _, want := range []string{"Silk Wrap Dress", "Leather Tote Bag", "₹", "Total", "Bengaluru", "Gift wrapped", "View your order"} {
		if !strings.Contains(conf, want) {
			t.Errorf("confirmation email missing %q", want)
		}
	}
	_ = os.WriteFile("/tmp/rloko-email-confirmation.html", []byte(conf), 0o644)

	// Per-status copy must differ and be relevant.
	checks := map[string]string{
		"DELIVERED":          "delivered",
		"TRANSIT":            "in transit",
		"DELIVERY EXCEPTION": "issue delivering",
		"CANCELLED":          "cancelled",
		"OUT FOR DELIVERY":   "out for delivery",
	}
	seen := map[string]bool{}
	for status, want := range checks {
		doc := renderStatus(order, status)
		if !strings.Contains(strings.ToLower(doc), want) {
			t.Errorf("status %q email missing relevant copy %q", status, want)
		}
		if seen[doc] {
			t.Errorf("status %q produced identical email to another status (not tailored)", status)
		}
		seen[doc] = true
		_ = os.WriteFile("/tmp/rloko-email-status-"+strings.ReplaceAll(strings.ToLower(status), " ", "_")+".html", []byte(doc), 0o644)
	}
}
