package config

import (
	"fmt"
	"os"
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
	SMTPHost            string
	SMTPPort            string
	SMTPUser            string
	SMTPPassword        string
	SMTPFrom            string
	SMTPFromName        string
	AppBaseURL          string // e.g. https://yoursite.com for verification/reset links
	AdminEmail          string // optional; new order alerts sent here
	StripeSecretKey     string
	StripeWebhookSecret string
	PayPalClientID      string
	PayPalSecret        string
	PayPalMode          string
	GoogleClientID      string
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
	ShippoMassUnit      string
	// Twilio Verify — required for registration OTP (no fallback)
	TwilioAccountSid       string
	TwilioAuthToken        string
	TwilioVerifyServiceSid string // Verify Service SID (VA...)
	// OTPDefaultCountryCode — digits only (e.g. 91); prefixed when user enters a 10-digit local number
	OTPDefaultCountryCode string
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

func Load() (*Config, error) {
	// Load .env from current directory (e.g. when run from backend/) and from backend/.env (when run from repo root)
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")

	env := getEnv("ENV", "development")
	jwtSecret := getEnv("JWT_SECRET", defaultJWTSecret)
	if env == "production" && (jwtSecret == "" || jwtSecret == defaultJWTSecret) {
		return nil, fmt.Errorf("production requires a strong JWT_SECRET (do not use default)")
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
		SMTPHost:            getEnv("SMTP_HOST", ""),
		SMTPPort:            getEnv("SMTP_PORT", "587"),
		SMTPUser:            getEnv("SMTP_USER", ""),
		SMTPPassword:        getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:            getEnv("SMTP_FROM", "noreply@rloco.com"),
		SMTPFromName:        getEnv("SMTP_FROM_NAME", "R-Loko"),
		AppBaseURL:          getEnv("APP_BASE_URL", "https://rloco.com"),
		AdminEmail:          getEnv("ADMIN_EMAIL", ""),
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		PayPalClientID:      getEnv("PAYPAL_CLIENT_ID", ""),
		PayPalSecret:        getEnv("PAYPAL_SECRET", ""),
		PayPalMode:          getEnv("PAYPAL_MODE", "sandbox"),
		GoogleClientID:      getEnv("GOOGLE_CLIENT_ID", ""),
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
		ShippoMassUnit:      getEnv("SHIPPO_MASS_UNIT", "lb"),
		TwilioAccountSid:       getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:        getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioVerifyServiceSid: getEnv("TWILIO_VERIFY_SERVICE_SID", ""),
		OTPDefaultCountryCode:  getEnv("OTP_DEFAULT_COUNTRY_CODE", "91"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
