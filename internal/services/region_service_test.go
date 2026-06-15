package services

import "testing"

func TestMarketFromPincode(t *testing.T) {
	s := NewRegionService()
	cases := []struct {
		name       string
		pincode    string
		wantMarket string
		wantOK     bool
	}{
		{"india 6 digit", "560001", "IN", true},
		{"us 5 digit", "94105", "US", true},
		{"us zip+4", "94105-1234", "US", true},
		{"pincode with spaces", " 560 001 ", "IN", true},
		{"unknown pincode", "ABC", "", false},
		{"empty", "", "", false},
		{"7 digits", "1234567", "", false},
		{"4 digits", "1234", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMarket, gotOK := s.MarketFromPincode(tc.pincode)
			if gotMarket != tc.wantMarket || gotOK != tc.wantOK {
				t.Errorf("MarketFromPincode(%q) = (%q, %v), want (%q, %v)",
					tc.pincode, gotMarket, gotOK, tc.wantMarket, tc.wantOK)
			}
		})
	}
}

func TestCurrencyForMarket(t *testing.T) {
	s := NewRegionService()
	cases := []struct{ market, want string }{
		{"IN", "INR"},
		{"US", "USD"},
		{"in", "INR"},
		{"us", "USD"},
		{"XX", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.market, func(t *testing.T) {
			if got := s.CurrencyForMarket(tc.market); got != tc.want {
				t.Errorf("CurrencyForMarket(%q) = %q, want %q", tc.market, got, tc.want)
			}
		})
	}
}
