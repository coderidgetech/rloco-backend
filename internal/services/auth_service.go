package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

// ErrAccountNotFound signals that no account exists for the given phone (login OTP).
// Handlers translate it into a structured "USER_NOT_FOUND" code so clients can route to signup.
var ErrAccountNotFound = errors.New("no account found for this phone number")

type AuthService interface {
	Register(ctx context.Context, email, password, name string) (*models.User, string, error)
	Login(ctx context.Context, email, password string) (*models.User, string, error)
	GoogleSignIn(ctx context.Context, idToken string) (*models.User, string, error)
	GenerateToken(user *models.User) (string, error)
	ValidateToken(tokenString string) (*Claims, error)
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, email string) error
	UpdateProfile(ctx context.Context, userID string, phone *string, birthday *time.Time, name *string, email *string, avatar *string, city *string) error
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	DeactivateAccount(ctx context.Context, userID string) error
	DeleteAccount(ctx context.Context, userID string) error
	// SendRegistrationOTP sends an OTP via Twilio Verify (SMS).
	SendRegistrationOTP(ctx context.Context, phoneRaw string) error
	// RegisterWithPhoneOTP verifies the OTP and creates the user (phone must match the sent OTP).
	RegisterWithPhoneOTP(ctx context.Context, phoneRaw, otpCode, email, password, name string) (*models.User, string, error)
	SendLoginOTP(ctx context.Context, phoneRaw string) error
	LoginWithPhoneOTP(ctx context.Context, phoneRaw, otpCode string) (*models.User, string, error)
	// RefreshSession issues a new JWT when the current one is valid or recently expired (signature must verify).
	RefreshSession(ctx context.Context, tokenString string) (*models.User, string, error)
}

type authService struct {
	userRepo              repositories.UserRepository
	passwordResetRepo     repositories.PasswordResetRepository
	emailVerificationRepo repositories.EmailVerificationRepository
	emailService          EmailService
	otpRepo            repositories.PhoneOTPRepository
	twilioVerify       *TwilioVerifyClient
	secret             string
	expiry             time.Duration
	googleClientID string
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(
	userRepo repositories.UserRepository,
	passwordResetRepo repositories.PasswordResetRepository,
	emailVerificationRepo repositories.EmailVerificationRepository,
	emailService EmailService,
	otpRepo repositories.PhoneOTPRepository,
	twilioVerify *TwilioVerifyClient,
	secret string,
	expiryStr string,
	googleClientID string,
) AuthService {
	expiry, _ := time.ParseDuration(expiryStr)
	if expiry == 0 {
		expiry = 24 * time.Hour
	}

	return &authService{
		userRepo:              userRepo,
		passwordResetRepo:     passwordResetRepo,
		emailVerificationRepo: emailVerificationRepo,
		emailService:          emailService,
		otpRepo:            otpRepo,
		twilioVerify:       twilioVerify,
		secret:             secret,
		expiry:             expiry,
		googleClientID:     googleClientID,
	}
}

func (s *authService) Register(ctx context.Context, email, password, name string) (*models.User, string, error) {
	// Check if user exists
	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, "", errors.New("user already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	// Create user
	user := &models.User{
		Email:         email,
		PasswordHash:  string(hashedPassword),
		Name:          name,
		Role:          "customer",
		Active:        true,
		EmailVerified: false,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, "", err
	}

	// Generate token
	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", errors.New("invalid credentials")
	}

	// Check if user is active
	if !user.Active {
		return nil, "", errors.New("account is deactivated")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", errors.New("invalid credentials")
	}

	// Generate token
	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *authService) GoogleSignIn(ctx context.Context, idToken string) (*models.User, string, error) {
	// Verify the ID token with Google's tokeninfo endpoint
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken))
	if err != nil {
		return nil, "", errors.New("failed to verify Google token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.New("invalid Google token")
	}

	var tokenInfo struct {
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		Aud           string `json:"aud"`
		EmailVerified string `json:"email_verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, "", errors.New("failed to parse Google token")
	}

	if s.googleClientID != "" && tokenInfo.Aud != s.googleClientID {
		return nil, "", errors.New("Google token audience mismatch")
	}
	if tokenInfo.Email == "" {
		return nil, "", errors.New("Google token missing email")
	}

	// Find existing user or create new one
	user, err := s.userRepo.GetByEmail(ctx, tokenInfo.Email)
	if err != nil {
		// New user — register via Google
		user = &models.User{
			Email:         tokenInfo.Email,
			Name:          tokenInfo.Name,
			PasswordHash:  "",
			Role:          "customer",
			Active:        true,
			EmailVerified: true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, "", errors.New("failed to create user account")
		}
	}

	if !user.Active {
		return nil, "", errors.New("account is deactivated")
	}

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *authService) GenerateToken(user *models.User) (string, error) {
	claims := &Claims{
		UserID: user.ID.Hex(),
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

func (s *authService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func (s *authService) RefreshSession(ctx context.Context, tokenString string) (*models.User, string, error) {
	if strings.TrimSpace(tokenString) == "" {
		return nil, "", errors.New("missing token")
	}
	keyFunc := func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.secret), nil
	}
	strictParser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	claims := &Claims{}
	token, err := strictParser.ParseWithClaims(tokenString, claims, keyFunc)
	if err != nil {
		if !errors.Is(err, jwt.ErrTokenExpired) {
			return nil, "", err
		}
		claims = &Claims{}
		looseParser := jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithoutClaimsValidation(),
		)
		token, err = looseParser.ParseWithClaims(tokenString, claims, keyFunc)
		if err != nil {
			return nil, "", err
		}
	}
	if token == nil {
		return nil, "", errors.New("invalid token")
	}
	c, ok := token.Claims.(*Claims)
	if !ok {
		return nil, "", errors.New("invalid claims")
	}
	// Reject tokens expired too long ago (limit refresh window)
	if c.ExpiresAt != nil {
		maxPast := time.Now().Add(-7 * 24 * time.Hour)
		if c.ExpiresAt.Time.Before(maxPast) {
			return nil, "", errors.New("session expired; please sign in again")
		}
	}
	userID, err := primitive.ObjectIDFromHex(c.UserID)
	if err != nil {
		return nil, "", errors.New("invalid user in token")
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, "", errors.New("user not found")
	}
	if !user.Active {
		return nil, "", errors.New("account is deactivated")
	}
	newToken, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, newToken, nil
}

func (s *authService) generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *authService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Don't reveal if user exists or not for security
		return nil
	}
	if !s.emailService.Configured() {
		return errors.New("password reset email cannot be sent: SMTP is not configured on the server")
	}

	token, err := s.generateRandomToken()
	if err != nil {
		return err
	}

	resetToken := &models.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour), // Token expires in 1 hour
		Used:      false,
	}

	if err := s.passwordResetRepo.Create(ctx, resetToken); err != nil {
		return err
	}

	// Send email
	return s.emailService.SendPasswordReset(user.Email, token)
}

func (s *authService) ResetPassword(ctx context.Context, token, newPassword string) error {
	resetToken, err := s.passwordResetRepo.GetByToken(ctx, token)
	if err != nil {
		return errors.New("invalid or expired token")
	}

	if resetToken.Used {
		return errors.New("token has already been used")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Update user password
	user, err := s.userRepo.GetByID(ctx, resetToken.UserID)
	if err != nil {
		return errors.New("user not found")
	}

	user.PasswordHash = string(hashedPassword)
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user.ID, user); err != nil {
		return err
	}

	// Mark token as used
	return s.passwordResetRepo.MarkAsUsed(ctx, resetToken.ID)
}

func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	verifyToken, err := s.emailVerificationRepo.GetByToken(ctx, token)
	if err != nil {
		return errors.New("invalid or expired token")
	}

	if verifyToken.Used {
		return errors.New("token has already been used")
	}

	// Update user email_verified
	user, err := s.userRepo.GetByID(ctx, verifyToken.UserID)
	if err != nil {
		return errors.New("user not found")
	}

	user.EmailVerified = true
	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(ctx, user.ID, user); err != nil {
		return err
	}

	// Mark token as used
	return s.emailVerificationRepo.MarkAsUsed(ctx, verifyToken.ID)
}

func (s *authService) ResendVerification(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return errors.New("user not found")
	}

	if user.EmailVerified {
		return errors.New("email already verified")
	}
	if !s.emailService.Configured() {
		return errors.New("verification email cannot be sent: SMTP is not configured on the server")
	}

	token, err := s.generateRandomToken()
	if err != nil {
		return err
	}

	verifyToken := &models.EmailVerificationToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour), // Token expires in 24 hours
		Used:      false,
	}

	if err := s.emailVerificationRepo.Create(ctx, verifyToken); err != nil {
		return err
	}

	// Send verification email
	return s.emailService.SendEmailVerification(user.Email, token)
}

func (s *authService) UpdateProfile(ctx context.Context, userID string, phone *string, birthday *time.Time, name *string, email *string, avatar *string, city *string) error {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("user not found")
	}

	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return errors.New("name cannot be empty")
		}
		user.Name = n
	}

	if email != nil {
		newEmail := strings.TrimSpace(*email)
		if newEmail == "" {
			return errors.New("email cannot be empty")
		}
		if !strings.EqualFold(newEmail, user.Email) {
			other, errLookup := s.userRepo.GetByEmail(ctx, newEmail)
			if errLookup != nil && errLookup != mongo.ErrNoDocuments {
				return errLookup
			}
			if other != nil && other.ID != user.ID {
				return errors.New("email already in use")
			}
			user.Email = newEmail
			user.EmailVerified = false
		}
	}

	if phone != nil {
		raw := strings.TrimSpace(*phone)
		if raw == "" {
			user.Phone = nil
			user.PhoneKey = ""
		} else {
			pk, err := NormalizePhoneKey(raw)
			if err != nil {
				return errors.New("invalid phone number")
			}
			canonical := PhoneKeyToE164Plus(pk)
			user.Phone = &canonical
			user.PhoneKey = pk
		}
	}
	if birthday != nil {
		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		b := time.Date(birthday.Year(), birthday.Month(), birthday.Day(), 0, 0, 0, 0, time.UTC)
		if b.After(today) {
			return errors.New("date of birth cannot be in the future")
		}
		user.Birthday = birthday
	}
	if avatar != nil {
		if a := strings.TrimSpace(*avatar); a != "" {
			user.Avatar = a
		}
	}
	if city != nil {
		user.City = strings.TrimSpace(*city)
	}
	user.UpdatedAt = time.Now()

	return s.userRepo.Update(ctx, user.ID, user)
}

func (s *authService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hashed)
	user.MustResetPassword = false // a chosen password clears the forced-reset gate
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(ctx, user.ID, user)
}

func (s *authService) DeactivateAccount(ctx context.Context, userID string) error {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("user not found")
	}
	user.Active = false
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(ctx, user.ID, user)
}

func (s *authService) DeleteAccount(ctx context.Context, userID string) error {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}
	return s.userRepo.Delete(ctx, id)
}

const (
	registrationOTPTTL       = 10 * time.Minute
	registrationOTPMinResend = 45 * time.Second
	maxRegistrationOTPVerify = 5
)

func (s *authService) SendRegistrationOTP(ctx context.Context, phoneRaw string) error {
	phoneKey, err := NormalizePhoneKey(phoneRaw)
	if err != nil {
		return err
	}
	u, errPhone := s.userRepo.GetByPhoneKey(ctx, phoneKey)
	if errPhone != nil && errPhone != mongo.ErrNoDocuments {
		return errPhone
	}
	if errPhone == nil && u != nil {
		return errors.New("an account with this phone already exists")
	}

	existing, err := s.otpRepo.Get(ctx, phoneKey, repositories.PhoneOTPPurposeRegistration)
	if err == nil && existing != nil {
		if time.Since(existing.LastSentAt) < registrationOTPMinResend {
			return errors.New("please wait before requesting another code")
		}
	}

	expires := time.Now().Add(registrationOTPTTL)
	now := time.Now()

	if s.twilioVerify == nil || !s.twilioVerify.Enabled() {
		return errors.New("Twilio Verify is not configured")
	}
	toE164 := PhoneKeyToE164Plus(phoneKey)
	verificationSid, err := s.twilioVerify.StartVerification(ctx, toE164)
	if err != nil {
		return MapStartVerificationError(err)
	}
	return s.otpRepo.Upsert(ctx, phoneKey, repositories.PhoneOTPPurposeRegistration, verificationSid, expires, now)
}

func (s *authService) RegisterWithPhoneOTP(ctx context.Context, phoneRaw, otpCode, email, password, name string) (*models.User, string, error) {
	phoneKey, err := NormalizePhoneKey(phoneRaw)
	if err != nil {
		return nil, "", err
	}
	ch, err := s.otpRepo.Get(ctx, phoneKey, repositories.PhoneOTPPurposeRegistration)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, "", errors.New("no verification code for this number; request a new code")
		}
		return nil, "", err
	}
	if time.Now().After(ch.ExpiresAt) {
		_ = s.otpRepo.Delete(ctx, ch.ID)
		return nil, "", errors.New("verification code has expired; request a new code")
	}
	if ch.Attempts >= maxRegistrationOTPVerify {
		return nil, "", errors.New("too many incorrect attempts; request a new code")
	}

	if s.twilioVerify == nil || !s.twilioVerify.Enabled() {
		return nil, "", errors.New("Twilio Verify is not configured")
	}
	toE164 := PhoneKeyToE164Plus(phoneKey)
	if err := s.twilioVerify.CheckVerification(ctx, toE164, strings.TrimSpace(otpCode)); err != nil {
		_ = s.otpRepo.IncrementAttempts(ctx, ch.ID)
		return nil, "", errors.New("invalid verification code")
	}

	dup, errDup := s.userRepo.GetByPhoneKey(ctx, phoneKey)
	if errDup != nil && errDup != mongo.ErrNoDocuments {
		return nil, "", errDup
	}
	if errDup == nil && dup != nil {
		return nil, "", errors.New("an account with this phone already exists")
	}
	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, "", errors.New("user already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	phoneStr := toE164
	user := &models.User{
		Email:         email,
		PasswordHash:  string(hashedPassword),
		Name:          name,
		Role:          "customer",
		Active:        true,
		EmailVerified: false,
		Phone:         &phoneStr,
		PhoneKey:      phoneKey,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, "", err
	}
	_ = s.otpRepo.Delete(ctx, ch.ID)

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *authService) SendLoginOTP(ctx context.Context, phoneRaw string) error {
	phoneKey, err := NormalizePhoneKey(phoneRaw)
	if err != nil {
		return err
	}
	u, err := s.userRepo.GetByPhoneKey(ctx, phoneKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrAccountNotFound
		}
		return err
	}
	// Backfill phone_key / canonical phone when user was saved with phone only (e.g. old profile update)
	if u.PhoneKey != phoneKey {
		canonical := PhoneKeyToE164Plus(phoneKey)
		u.PhoneKey = phoneKey
		u.Phone = &canonical
		_ = s.userRepo.Update(ctx, u.ID, u)
	}
	if !u.Active {
		return errors.New("account is deactivated")
	}
	if u.Role != "customer" {
		return errors.New("use email sign-in for this account")
	}

	existing, err := s.otpRepo.Get(ctx, phoneKey, repositories.PhoneOTPPurposeLogin)
	if err == nil && existing != nil {
		if time.Since(existing.LastSentAt) < registrationOTPMinResend {
			return errors.New("please wait before requesting another code")
		}
	}

	expires := time.Now().Add(registrationOTPTTL)
	now := time.Now()

	if s.twilioVerify == nil || !s.twilioVerify.Enabled() {
		return errors.New("Twilio Verify is not configured")
	}
	toE164 := PhoneKeyToE164Plus(phoneKey)
	verificationSid, err := s.twilioVerify.StartVerification(ctx, toE164)
	if err != nil {
		return MapStartVerificationError(err)
	}
	return s.otpRepo.Upsert(ctx, phoneKey, repositories.PhoneOTPPurposeLogin, verificationSid, expires, now)
}

func (s *authService) LoginWithPhoneOTP(ctx context.Context, phoneRaw, otpCode string) (*models.User, string, error) {
	phoneKey, err := NormalizePhoneKey(phoneRaw)
	if err != nil {
		return nil, "", err
	}
	ch, err := s.otpRepo.Get(ctx, phoneKey, repositories.PhoneOTPPurposeLogin)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, "", errors.New("no verification code for this number; request a new code")
		}
		return nil, "", err
	}
	if time.Now().After(ch.ExpiresAt) {
		_ = s.otpRepo.Delete(ctx, ch.ID)
		return nil, "", errors.New("verification code has expired; request a new code")
	}
	if ch.Attempts >= maxRegistrationOTPVerify {
		return nil, "", errors.New("too many incorrect attempts; request a new code")
	}

	if s.twilioVerify == nil || !s.twilioVerify.Enabled() {
		return nil, "", errors.New("Twilio Verify is not configured")
	}
	toE164 := PhoneKeyToE164Plus(phoneKey)
	if err := s.twilioVerify.CheckVerification(ctx, toE164, strings.TrimSpace(otpCode)); err != nil {
		_ = s.otpRepo.IncrementAttempts(ctx, ch.ID)
		return nil, "", errors.New("invalid verification code")
	}

	user, err := s.userRepo.GetByPhoneKey(ctx, phoneKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, "", ErrAccountNotFound
		}
		return nil, "", err
	}
	if !user.Active {
		return nil, "", errors.New("account is deactivated")
	}
	if user.Role != "customer" {
		return nil, "", errors.New("use email sign-in for this account")
	}

	_ = s.otpRepo.Delete(ctx, ch.ID)

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}