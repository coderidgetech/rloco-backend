package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/models"
	"rloco-backend/internal/services"
)

// fakeConfigService lets the region handler test drive regionStatusFromConfig
// without a database. A nil cfg with err simulates a store error/absence.
type fakeConfigService struct {
	cfg map[string]interface{}
	err error
}

func (f *fakeConfigService) Get(context.Context) (*models.SiteConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &models.SiteConfig{Config: f.cfg}, nil
}

func (f *fakeConfigService) Update(context.Context, *models.SiteConfig) error { return nil }

func regionsConfig(usEnabled, inEnabled bool, inMsg string) map[string]interface{} {
	return map[string]interface{}{
		"general": map[string]interface{}{
			"regions": map[string]interface{}{
				"US": map[string]interface{}{"enabled": usEnabled},
				"IN": map[string]interface{}{"enabled": inEnabled, "comingSoonMessage": inMsg},
			},
		},
	}
}

// TestRegionHandler_Route exercises the endpoint through a real gin router, as it
// is wired in cmd/server/main.go (public group, no auth), validating routing and
// query handling end to end without a database.
func TestRegionHandler_Route(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewRegionHandler(
		&fakeConfigService{cfg: regionsConfig(true, false, "Soon in India")},
		services.NewRegionService(),
	)
	r := gin.New()
	api := r.Group("/api")
	api.GET("/region/resolve", h.Resolve)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/region/resolve?pincode=560001&city=Bangalore", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["market"] != "IN" || resp["currency"] != "INR" || resp["enabled"] != false ||
		resp["city"] != "Bangalore" || resp["comingSoonMessage"] != "Soon in India" {
		t.Errorf("unexpected response: %v", resp)
	}
}

// TestRegionHandler_CityTruncation verifies the echoed city hint is capped by rune
// count and stays valid UTF-8 even when the input is multi-byte (e.g. Devanagari).
func TestRegionHandler_CityTruncation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewRegionHandler(&fakeConfigService{cfg: regionsConfig(true, true, "")}, services.NewRegionService())

	// 150 Devanagari runes (3 bytes each) — exceeds the 100-rune cap.
	longCity := strings.Repeat("अ", 150)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/region/resolve?pincode=560001&city="+url.QueryEscape(longCity), nil)
	h.Resolve(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var resp struct {
		City string `json:"city"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !utf8.ValidString(resp.City) {
		t.Errorf("echoed city is not valid UTF-8: %q", resp.City)
	}
	if got := utf8.RuneCountInString(resp.City); got != maxCityHintLen {
		t.Errorf("echoed city rune count = %d, want %d", got, maxCityHintLen)
	}
}

func TestRegionHandler_Resolve(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		query       string
		cfg         *fakeConfigService
		wantStatus  int
		wantMarket  string
		wantCurr    string
		wantEnabled bool
		wantMessage string
		wantCity    string
	}{
		{
			name:        "india pincode disabled region",
			query:       "pincode=560001",
			cfg:         &fakeConfigService{cfg: regionsConfig(true, false, "Soon in India")},
			wantStatus:  http.StatusOK,
			wantMarket:  "IN",
			wantCurr:    "INR",
			wantEnabled: false,
			wantMessage: "Soon in India",
		},
		{
			name:        "us zip enabled region",
			query:       "pincode=94105",
			cfg:         &fakeConfigService{cfg: regionsConfig(true, false, "")},
			wantStatus:  http.StatusOK,
			wantMarket:  "US",
			wantCurr:    "USD",
			wantEnabled: true,
		},
		{
			name:        "country fallback when pincode unknown",
			query:       "country=India",
			cfg:         &fakeConfigService{cfg: regionsConfig(true, true, "")},
			wantStatus:  http.StatusOK,
			wantMarket:  "IN",
			wantCurr:    "INR",
			wantEnabled: true,
		},
		{
			// On a store error the helper falls back to getDefaultConfig(), in which
			// US is enabled — so this exercises the default-fallback path, not fail-open.
			name:        "config store error falls back to defaults",
			query:       "pincode=94105",
			cfg:         &fakeConfigService{err: errors.New("db down")},
			wantStatus:  http.StatusOK,
			wantMarket:  "US",
			wantCurr:    "USD",
			wantEnabled: true,
		},
		{
			// Non-empty config with no general block => ensureRegionDefaults leaves it
			// untouched and the traversal fails open (enabled).
			name:        "fail open when general block absent",
			query:       "pincode=560001",
			cfg:         &fakeConfigService{cfg: map[string]interface{}{"design": map[string]interface{}{"x": 1}}},
			wantStatus:  http.StatusOK,
			wantMarket:  "IN",
			wantCurr:    "INR",
			wantEnabled: true,
		},
		{
			name:       "bad request when nothing resolves",
			query:      "pincode=ABC",
			cfg:        &fakeConfigService{cfg: regionsConfig(true, true, "")},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:        "echoes city hint",
			query:       "pincode=560001&city=Bangalore",
			cfg:         &fakeConfigService{cfg: regionsConfig(true, true, "")},
			wantStatus:  http.StatusOK,
			wantMarket:  "IN",
			wantCurr:    "INR",
			wantEnabled: true,
			wantCity:    "Bangalore",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewRegionHandler(tc.cfg, services.NewRegionService())

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/region/resolve?"+tc.query, nil)

			h.Resolve(c)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}

			var resp struct {
				Market            string `json:"market"`
				Currency          string `json:"currency"`
				City              string `json:"city"`
				Enabled           bool   `json:"enabled"`
				ComingSoonMessage string `json:"comingSoonMessage"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.Market != tc.wantMarket {
				t.Errorf("market = %q, want %q", resp.Market, tc.wantMarket)
			}
			if resp.Currency != tc.wantCurr {
				t.Errorf("currency = %q, want %q", resp.Currency, tc.wantCurr)
			}
			if resp.Enabled != tc.wantEnabled {
				t.Errorf("enabled = %v, want %v", resp.Enabled, tc.wantEnabled)
			}
			if resp.ComingSoonMessage != tc.wantMessage {
				t.Errorf("comingSoonMessage = %q, want %q", resp.ComingSoonMessage, tc.wantMessage)
			}
			if resp.City != tc.wantCity {
				t.Errorf("city = %q, want %q", resp.City, tc.wantCity)
			}
		})
	}
}
