package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const defaultJWTSecret = "your-secret-key-change-in-production"

type Config struct {
	Port                string
	Env                 string
	MongoDBURI          string
	JWTSecret           string
	JWTExpiry           string
	CORSAllowedOrigins  string // comma-separated; empty or "*" = allow all (dev only)
	StorageType         string
	StorageEndpoint     string
	StorageAccessKey    string
	StorageSecretKey    string
	StorageBucket       string
	StoragePublicURL    string // optional; for S3/R2 public bucket URL (e.g. https://pub-xxx.r2.dev)
	ResendAPIKey        string // Resend API key (re_...) for transactional email
	SMTPFrom            string // sender address (SMTP_FROM env); also used as From: in Resend
	SMTPFromName        string // sender display name
	AppBaseURL          string // e.g. https://yoursite.com for verification/reset links
	AdminEmail          string // optional; new order alerts sent here
	StripeSecretKey     string
	StripeWebhookSecret string
	GoogleClientID         string
	GoogleMapsBrowserKey   string // browser Maps+Places key; exposed via GET /api/auth/client-config (referrer-restricted)
	ShippoAPIKey        string
	ShippoBaseURL       string
	ShippoFromName      string
	ShippoFromEmail     string
	ShippoFromPhone     string
	ShippoFromStreet1   string
	ShippoFromStreet2   string
	ShippoFromCity      string
	ShippoFromState     string
	ShippoFromZip       string
	ShippoFromCountry   string
	ShippoParcelLength  string
	ShippoParcelWidth   string
	ShippoParcelHeight  string
	ShippoParcelWeight  string
	ShippoDistanceUnit  string
	ShippoMassUnit          string
	ShippoWebhookSecret     string // HMAC secret for verifying Shippo track_updated webhooks
	// Shiprocket — used for domestic India shipments
	ShiprocketEmail          string
	ShiprocketPassword       string
	ShiprocketBaseURL        string
	ShiprocketPickupLocation string // name of the pickup location configured in Shiprocket dashboard
	ShiprocketParcelLength   string
	ShiprocketParcelWidth    string
	ShiprocketParcelHeight   string
	ShiprocketParcelWeight   string
	ShiprocketWebhookSecret  string // shared token for validating Shiprocket webhook calls
	// Twilio Verify — required for registration OTP (no fallback)
	TwilioAccountSid       string
	TwilioAuthToken        string
	TwilioVerifyServiceSid string // Verify Service SID (VA...)
	// Twilio Programmable Messaging — for transactional SMS (order confirmations etc.)
	TwilioPhoneNumber string
	// FCM — Firebase Cloud Messaging via HTTP v1 API (service account JSON)
	FCMServiceAccountJSON string
	// APIRateLimitRPM is max HTTP API requests per client IP per minute (global limiter, production only).
	APIRateLimitRPM int
	// Order checkout defaults (orders store totals in USD; INR shipping quotes convert using OrderINRPerUSD).
	OrderDefaultShippingUSD        float64
	OrderDefaultTaxRate            float64
	OrderFreeShippingSubtotalUSD   float64
	OrderINRPerUSD                 float64
	// OrderIndiaDefaultGSTPercent is the GST % applied when no Mongo tax_rates row matches India (e.g. 18 for 18%).
	OrderIndiaDefaultGSTPercent float64
	// StripeProductTaxCode optional Stripe Tax Code id (txcd_...) for US merchandise line items; empty uses Dashboard default.
	StripeProductTaxCode string
}

// ValidateTwilioVerify returns an error if Twilio Verify env is incomplete (registration OTP will not work).
func (c *Config) ValidateTwilioVerify() error {
	if strings.TrimSpace(c.TwilioAccountSid) == "" {
		return fmt.Errorf("TWILIO_ACCOUNT_SID is required")
	}
	if strings.TrimSpace(c.TwilioAuthToken) == "" {
		return fmt.Errorf("TWILIO_AUTH_TOKEN is required")
	}
	if strings.TrimSpace(c.TwilioVerifyServiceSid) == "" {
		return fmt.Errorf("TWILIO_VERIFY_SERVICE_SID is required")
	}
	return nil
}

// EmailConfigReady returns true when Resend is configured to send emails.
func (c *Config) EmailConfigReady() bool {
	return strings.TrimSpace(c.ResendAPIKey) != "" && strings.TrimSpace(c.SMTPFrom) != ""
}

func Load() (*Config, error) {
	// In production, only use the process environment (e.g. App Platform). Do not load .env —
	// it can mask missing platform env and leave JWT_SECRET at the dev default.
	if os.Getenv("ENV") != "production" {
		_ = godotenv.Load(".env")
		_ = godotenv.Load("backend/.env")
	}

	env := getEnv("ENV", "development")
	jwtSecret := getEnv("JWT_SECRET", defaultJWTSecret)
	if env == "production" && (jwtSecret == "" || jwtSecret == defaultJWTSecret) {
		return nil, fmt.Errorf("production requires a strong JWT_SECRET (do not use default)")
	}

	apiRL := strings.TrimSpace(os.Getenv("API_RATE_LIMIT_RPM"))
	var apiRateLimitRPM int
	if apiRL != "" {
		n, err := strconv.Atoi(apiRL)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("API_RATE_LIMIT_RPM must be a positive integer")
		}
		apiRateLimitRPM = n
	} else {
		if env == "production" {
			apiRateLimitRPM = 500
		} else {
			apiRateLimitRPM = 1000
		}
	}

	return &Config{
		Port:                getEnv("PORT", "8080"),
		Env:                 env,
		MongoDBURI:          getEnv("MONGODB_URI", "mongodb://admin:password@localhost:27017/rloco?authSource=admin"),
		JWTSecret:           jwtSecret,
		JWTExpiry:           getEnv("JWT_EXPIRY", "24h"),
		CORSAllowedOrigins:  getEnv("CORS_ALLOWED_ORIGINS", ""),
		StorageType:         getEnv("STORAGE_TYPE", "local"),
		StorageEndpoint:     getEnv("STORAGE_ENDPOINT", "localhost:9000"),
		StorageAccessKey:    getEnv("STORAGE_ACCESS_KEY", "minioadmin"),
		StorageSecretKey:    getEnv("STORAGE_SECRET_KEY", "minioadmin"),
		StorageBucket:       getEnv("STORAGE_BUCKET", "rloco-uploads"),
		StoragePublicURL:    getEnv("STORAGE_PUBLIC_URL", ""),
		ResendAPIKey:        getEnv("RESEND_API_KEY", ""),
		SMTPFrom:            firstNonEmptyEnvWithDefault("noreply@rloco.com", "SMTP_FROM", "SMTP_FROM_EMAIL", "EMAIL_FROM"),
		SMTPFromName:        firstNonEmptyEnvWithDefault("R-Loko", "SMTP_FROM_NAME", "EMAIL_FROM_NAME"),
		AppBaseURL:          getEnv("APP_BASE_URL", "https://rloco.com"),
		AdminEmail:          getEnv("ADMIN_EMAIL", ""),
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		GoogleClientID:       getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleMapsBrowserKey: strings.TrimSpace(firstNonEmptyEnv("GOOGLE_MAPS_BROWSER_KEY", "GOOGLE_MAPS_API_KEY")),
		ShippoAPIKey:        getEnv("SHIPPO_API_KEY", ""),
		ShippoBaseURL:       getEnv("SHIPPO_BASE_URL", "https://api.goshippo.com"),
		ShippoFromName:      getEnv("SHIPPO_FROM_NAME", ""),
		ShippoFromEmail:     getEnv("SHIPPO_FROM_EMAIL", ""),
		ShippoFromPhone:     getEnv("SHIPPO_FROM_PHONE", ""),
		ShippoFromStreet1:   getEnv("SHIPPO_FROM_STREET1", ""),
		ShippoFromStreet2:   getEnv("SHIPPO_FROM_STREET2", ""),
		ShippoFromCity:      getEnv("SHIPPO_FROM_CITY", ""),
		ShippoFromState:     getEnv("SHIPPO_FROM_STATE", ""),
		ShippoFromZip:       getEnv("SHIPPO_FROM_ZIP", ""),
		ShippoFromCountry:   getEnv("SHIPPO_FROM_COUNTRY", ""),
		ShippoParcelLength:  getEnv("SHIPPO_PARCEL_LENGTH", "10"),
		ShippoParcelWidth:   getEnv("SHIPPO_PARCEL_WIDTH", "8"),
		ShippoParcelHeight:  getEnv("SHIPPO_PARCEL_HEIGHT", "4"),
		ShippoParcelWeight:  getEnv("SHIPPO_PARCEL_WEIGHT", "0.5"),
		ShippoDistanceUnit:  getEnv("SHIPPO_DISTANCE_UNIT", "in"),
		ShippoMassUnit:          getEnv("SHIPPO_MASS_UNIT", "lb"),
		ShippoWebhookSecret:     getEnv("SHIPPO_WEBHOOK_SECRET", ""),
		ShiprocketEmail:          getEnv("SHIPROCKET_EMAIL", ""),
		ShiprocketPassword:       getEnv("SHIPROCKET_PASSWORD", ""),
		ShiprocketBaseURL:        getEnv("SHIPROCKET_BASE_URL", "https://apiv2.shiprocket.in/v1/external"),
		ShiprocketPickupLocation: getEnv("SHIPROCKET_PICKUP_LOCATION", "Primary"),
		ShiprocketParcelLength:   getEnv("SHIPROCKET_PARCEL_LENGTH", "10"),
		ShiprocketParcelWidth:    getEnv("SHIPROCKET_PARCEL_WIDTH", "8"),
		ShiprocketParcelHeight:   getEnv("SHIPROCKET_PARCEL_HEIGHT", "4"),
		ShiprocketParcelWeight:   getEnv("SHIPROCKET_PARCEL_WEIGHT", "0.5"),
		ShiprocketWebhookSecret:  getEnv("SHIPROCKET_WEBHOOK_SECRET", ""),
		TwilioAccountSid:       getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:        getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioVerifyServiceSid: getEnv("TWILIO_VERIFY_SERVICE_SID", ""),
		TwilioPhoneNumber:      getEnv("TWILIO_PHONE_NUMBER", ""),
		FCMServiceAccountJSON:  getEnv("FCM_SERVICE_ACCOUNT_JSON", ""),
		APIRateLimitRPM:        apiRateLimitRPM,
		OrderDefaultShippingUSD:      float64Env("ORDER_DEFAULT_SHIPPING_USD", 15),
		OrderDefaultTaxRate:          float64Env("ORDER_DEFAULT_TAX_RATE", 0.08),
		OrderFreeShippingSubtotalUSD: float64Env("ORDER_FREE_SHIPPING_SUBTOTAL_USD", 200),
		OrderINRPerUSD:               float64Env("ORDER_INR_PER_USD", 75),
		OrderIndiaDefaultGSTPercent:  float64Env("ORDER_INDIA_DEFAULT_GST_PERCENT", 18),
		StripeProductTaxCode:         strings.TrimSpace(getEnv("STRIPE_TAX_PRODUCT_CODE", "")),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyEnvWithDefault(defaultValue string, keys ...string) string {
	if value := firstNonEmptyEnv(keys...); value != "" {
		return value
	}
	return defaultValue
}

// float64Env parses a float env var; on empty or invalid returns defaultValue.
func float64Env(key string, defaultValue float64) float64 {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return defaultValue
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n < 0 {
		return defaultValue
	}
	return n
}
