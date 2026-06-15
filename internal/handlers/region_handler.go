package handlers

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"rloco-backend/internal/services"
)

// maxCityHintLen bounds the user-supplied city hint that the resolver echoes back,
// keeping an arbitrary-length string from being reflected in the response.
const maxCityHintLen = 100

// RegionHandler resolves a postal code to a storefront market, currency, and the
// live availability of that market. Clients use it to derive their market instead
// of relying on a manual region toggle.
type RegionHandler struct {
	configService services.ConfigService
	regionService services.RegionService
}

func NewRegionHandler(configService services.ConfigService, regionService services.RegionService) *RegionHandler {
	return &RegionHandler{configService: configService, regionService: regionService}
}

// Resolve handles GET /api/region/resolve?pincode=&country=&city=.
// It maps the pincode to a market (IN/US) — falling back to the country hint when
// the pincode is empty/unknown — attaches the market's currency, and reads the live
// general.regions site config for the market's availability.
func (h *RegionHandler) Resolve(c *gin.Context) {
	pincode := strings.TrimSpace(c.Query("pincode"))
	country := strings.TrimSpace(c.Query("country"))
	// Truncate by rune count, not bytes, so multi-byte city names (e.g. Devanagari
	// for the IN market) are never sliced mid-character into invalid UTF-8.
	city := strings.TrimSpace(c.Query("city"))
	if utf8.RuneCountInString(city) > maxCityHintLen {
		city = string([]rune(city)[:maxCityHintLen])
	}

	market, ok := h.regionService.MarketFromPincode(pincode)
	if !ok {
		// Fall back to an explicit country hint, reusing the handler-layer mapping.
		market = marketFromCountry(country)
	}
	if market == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not resolve a market from the provided pincode or country"})
		return
	}

	enabled, message := regionStatusFromConfig(c.Request.Context(), h.configService, market)

	c.JSON(http.StatusOK, gin.H{
		"market":            market,
		"currency":          h.regionService.CurrencyForMarket(market),
		"city":              city,
		"enabled":           enabled,
		"comingSoonMessage": message,
	})
}
