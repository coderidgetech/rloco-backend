package services

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/stripe/stripe-go/v76"
	taxcalculation "github.com/stripe/stripe-go/v76/tax/calculation"
	"rloco-backend/internal/models"
)

// TaxLine is one taxable line for per-product tax calculation.
type TaxLine struct {
	AmountUSD  float64  // taxable amount (USD, post-discount share)
	TaxCode    string   // Stripe tax code (txcd_…); empty falls back to the global default
	GSTPercent *float64 // India per-product GST %; nil falls back to the location/default rate
}

// StripeUSTax calculates US sales tax via Stripe Tax (Tax Calculation API).
type StripeUSTax interface {
	Calculate(ctx context.Context, lines []TaxLine, shippingUSD float64, shipping models.ShippingInfo) (taxUSD float64, err error)
}

type stripeUSTax struct {
	secretKey      string
	productTaxCode string // optional Stripe tax code id (txcd_...); empty uses Dashboard default
}

// NewStripeUSTax returns nil if STRIPE_SECRET_KEY is not set.
func NewStripeUSTax(secretKey, productTaxCode string) StripeUSTax {
	key := strings.TrimSpace(secretKey)
	if key == "" {
		return nil
	}
	return &stripeUSTax{
		secretKey:      key,
		productTaxCode: strings.TrimSpace(productTaxCode),
	}
}

func (s *stripeUSTax) Calculate(ctx context.Context, lines []TaxLine, shippingUSD float64, shipping models.ShippingInfo) (float64, error) {
	_ = ctx
	if shippingUSD < 0 {
		shippingUSD = 0
	}
	shipCents := int64(math.Round(shippingUSD * 100))
	var totalMerchCents int64
	for _, ln := range lines {
		if ln.AmountUSD > 0 {
			totalMerchCents += int64(math.Round(ln.AmountUSD * 100))
		}
	}
	if totalMerchCents <= 0 && shipCents <= 0 {
		return 0, nil
	}

	line1 := strings.TrimSpace(shipping.Address)
	if line1 == "" {
		line1 = "Shipping address"
	}
	state := strings.TrimSpace(shipping.State)
	city := strings.TrimSpace(shipping.City)
	zip := strings.TrimSpace(shipping.ZipCode)
	if zip == "" {
		return 0, fmt.Errorf("stripe tax: US orders require a postal/zip code")
	}

	stripe.Key = s.secretKey

	params := &stripe.TaxCalculationParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		CustomerDetails: &stripe.TaxCalculationCustomerDetailsParams{
			Address: &stripe.AddressParams{
				Line1:      stripe.String(line1),
				City:       stripe.String(city),
				State:      stripe.String(state),
				PostalCode: stripe.String(zip),
				Country:    stripe.String("US"),
			},
			AddressSource: stripe.String(string(stripe.TaxCalculationCustomerDetailsAddressSourceShipping)),
		},
	}
	// One Stripe line item per product, each with its own tax code (falls back to
	// the global default when a product has none).
	for i, ln := range lines {
		if ln.AmountUSD <= 0 {
			continue
		}
		cents := int64(math.Round(ln.AmountUSD * 100))
		if cents <= 0 {
			continue
		}
		li := &stripe.TaxCalculationLineItemParams{
			Amount:      stripe.Int64(cents),
			Reference:   stripe.String(fmt.Sprintf("merch-%d", i)),
			TaxBehavior: stripe.String(string(stripe.TaxCalculationLineItemTaxBehaviorExclusive)),
		}
		code := strings.TrimSpace(ln.TaxCode)
		if code == "" {
			code = s.productTaxCode
		}
		if code != "" {
			li.TaxCode = stripe.String(code)
		}
		params.LineItems = append(params.LineItems, li)
	}
	if shipCents > 0 {
		params.ShippingCost = &stripe.TaxCalculationShippingCostParams{
			Amount:      stripe.Int64(shipCents),
			TaxBehavior: stripe.String(string(stripe.TaxCalculationShippingCostTaxBehaviorExclusive)),
		}
	}

	calc, err := taxcalculation.New(params)
	if err != nil {
		return 0, fmt.Errorf("stripe tax calculation: %w", err)
	}
	// Exclusive tax is the usual case for tax-exclusive line items + shipping.
	taxCents := calc.TaxAmountExclusive
	if taxCents == 0 && calc.TaxAmountInclusive > 0 {
		taxCents = calc.TaxAmountInclusive
	}
	return float64(taxCents) / 100, nil
}

// CountryLooksUS returns true for common US country strings from checkout/API.
func CountryLooksUS(country string) bool {
	switch strings.TrimSpace(strings.ToLower(country)) {
	case "united states", "us", "usa":
		return true
	default:
		return false
	}
}

// CountryLooksIndia returns true for India country strings.
func CountryLooksIndia(country string) bool {
	switch strings.TrimSpace(strings.ToLower(country)) {
	case "india", "in":
		return true
	default:
		return false
	}
}
