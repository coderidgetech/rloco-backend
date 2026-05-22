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

// StripeUSTax calculates US sales tax via Stripe Tax (Tax Calculation API).
type StripeUSTax interface {
	Calculate(ctx context.Context, taxableMerchandiseUSD, shippingUSD float64, shipping models.ShippingInfo) (taxUSD float64, err error)
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

func (s *stripeUSTax) Calculate(ctx context.Context, taxableMerchandiseUSD, shippingUSD float64, shipping models.ShippingInfo) (float64, error) {
	_ = ctx
	if taxableMerchandiseUSD < 0 {
		taxableMerchandiseUSD = 0
	}
	if shippingUSD < 0 {
		shippingUSD = 0
	}
	merchCents := int64(math.Round(taxableMerchandiseUSD * 100))
	shipCents := int64(math.Round(shippingUSD * 100))
	if merchCents <= 0 && shipCents <= 0 {
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
	if merchCents > 0 {
		li := &stripe.TaxCalculationLineItemParams{
			Amount:      stripe.Int64(merchCents),
			Reference:   stripe.String("merchandise"),
			TaxBehavior: stripe.String(string(stripe.TaxCalculationLineItemTaxBehaviorExclusive)),
		}
		if s.productTaxCode != "" {
			li.TaxCode = stripe.String(s.productTaxCode)
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
