package services

import (
	"regexp"
	"strings"
)

// RegionService resolves a postal code to a storefront market and its currency.
// It is stateless: pincode/ZIP parsing is pure logic with no datastore access.
//
// Country-hint fallback intentionally lives in the handler layer (which already
// owns the country->market mapping) so this service holds no duplicate of it.
type RegionService interface {
	// MarketFromPincode resolves a postal code to a market code ("IN" or "US").
	// A 6-digit value resolves to IN; a 5 or 5+4 digit value resolves to US.
	// ok is false when the pincode matches neither format.
	MarketFromPincode(pincode string) (market string, ok bool)

	// CurrencyForMarket returns the ISO currency code for a market ("INR"/"USD").
	CurrencyForMarket(market string) string
}

type regionService struct{}

// NewRegionService constructs the stateless region resolver.
func NewRegionService() RegionService {
	return &regionService{}
}

// These mirror the client-side validation in the Flutter guest delivery cubit
// (mobile-app/lib/core/delivery/presentation/guest_delivery_cubit.dart) so the
// server and clients agree on which pincodes map to which market.
var (
	indiaPincodeRe = regexp.MustCompile(`^\d{6}$`)
	usZipRe        = regexp.MustCompile(`^\d{5}(-\d{4})?$`)
)

func (s *regionService) MarketFromPincode(pincode string) (string, bool) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(pincode), " ", "")
	switch {
	case indiaPincodeRe.MatchString(cleaned):
		return "IN", true
	case usZipRe.MatchString(cleaned):
		return "US", true
	default:
		return "", false
	}
}

func (s *regionService) CurrencyForMarket(market string) string {
	switch strings.ToUpper(strings.TrimSpace(market)) {
	case "IN":
		return "INR"
	case "US":
		return "USD"
	default:
		return ""
	}
}
