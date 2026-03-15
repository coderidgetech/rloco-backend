package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

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
	UpdateProfile(ctx context.Context, userID string, phone *string, birthday *time.Time) error
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	DeactivateAccount(ctx context.Context, userID string) error
	DeleteAccount(ctx context.Context, userID string) error
}

type authService struct {
	userRepo              repositories.UserRepository
	passwordResetRepo     repositories.PasswordResetRepository
	emailVerificationRepo repositories.EmailVerificationRepository
	emailService          EmailService
	secret                string
	expiry                time.Duration
	googleClientID        string
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(userRepo repositories.UserRepository, passwordResetRepo repositories.PasswordResetRepository, emailVerificationRepo repositories.EmailVerificationRepository, emailService EmailService, secret string, expiryStr string, googleClientID string) AuthService {
	expiry, _ := time.ParseDuration(expiryStr)
	if expiry == 0 {
		expiry = 24 * time.Hour
	}

	return &authService{
		userRepo:              userRepo,
		passwordResetRepo:     passwordResetRepo,
		emailVerificationRepo: emailVerificationRepo,
		emailService:          emailService,
		secret:                secret,
		expiry:                expiry,
		googleClientID:        googleClientID,
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

func (s *authService) UpdateProfile(ctx context.Context, userID string, phone *string, birthday *time.Time) error {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("user not found")
	}

	if phone != nil {
		user.Phone = phone
	}
	if birthday != nil {
		user.Birthday = birthday
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