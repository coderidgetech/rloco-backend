package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

// VendorCreateResult is returned when an admin creates a vendor with a portal login.
type VendorCreateResult struct {
	Vendor                 *models.Vendor `json:"vendor"`
	TemporaryPassword      string         `json:"temporary_password,omitempty"`
	CredentialsEmailSent   bool           `json:"credentials_email_sent"`
	CredentialsEmailError  string         `json:"credentials_email_error,omitempty"`
	LoginURL               string         `json:"login_url"`
}

type VendorService interface {
	Create(ctx context.Context, vendor *models.Vendor, initialPassword string) (*VendorCreateResult, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Vendor, error)
	Update(ctx context.Context, id primitive.ObjectID, vendor *models.Vendor) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, limit, skip int) ([]*models.Vendor, int64, error)
	UpdatePermissions(ctx context.Context, id primitive.ObjectID, permissions map[string]interface{}) error
}

type vendorService struct {
	vendorRepo    repositories.VendorRepository
	userRepo      repositories.UserRepository
	emailService  EmailService
	appBaseURL    string
}

func NewVendorService(
	vendorRepo repositories.VendorRepository,
	userRepo repositories.UserRepository,
	emailService EmailService,
	appBaseURL string,
) VendorService {
	return &vendorService{
		vendorRepo:   vendorRepo,
		userRepo:     userRepo,
		emailService: emailService,
		appBaseURL:   strings.TrimSuffix(strings.TrimSpace(appBaseURL), "/"),
	}
}

func generateTemporaryPassword() (string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b), nil
}

func (s *vendorService) Create(ctx context.Context, vendor *models.Vendor, initialPassword string) (*VendorCreateResult, error) {
	vendor.Email = strings.TrimSpace(strings.ToLower(vendor.Email))
	vendor.Name = strings.TrimSpace(vendor.Name)
	if vendor.Email == "" || !strings.Contains(vendor.Email, "@") {
		return nil, errors.New("valid vendor email is required")
	}
	if vendor.Name == "" {
		return nil, errors.New("vendor name is required")
	}

	existingUser, userErr := s.userRepo.GetByEmail(ctx, vendor.Email)
	if userErr != nil && !errors.Is(userErr, mongo.ErrNoDocuments) {
		return nil, userErr
	}
	if existingUser != nil {
		switch existingUser.Role {
		case "vendor":
			return nil, fmt.Errorf("a vendor with email %s already exists", vendor.Email)
		case "admin", "super_admin":
			return nil, fmt.Errorf("a user with email %s already exists", vendor.Email)
		default:
			// Customer or other non-privileged role — upgrade to vendor
			return s.upgradeToVendor(ctx, existingUser, vendor)
		}
	}

	if initialPassword != "" && len(initialPassword) < 8 {
		return nil, errors.New("initial password must be at least 8 characters")
	}

	if vendor.Status == "" {
		vendor.Status = "active"
	}

	if err := s.vendorRepo.Create(ctx, vendor); err != nil {
		return nil, err
	}

	plain := initialPassword
	if plain == "" {
		var err error
		plain, err = generateTemporaryPassword()
		if err != nil {
			_ = s.vendorRepo.Delete(ctx, vendor.ID)
			return nil, err
		}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		_ = s.vendorRepo.Delete(ctx, vendor.ID)
		return nil, err
	}

	vendorID := vendor.ID
	user := &models.User{
		Email:         vendor.Email,
		PasswordHash:  string(hashed),
		Name:          vendor.Name,
		Role:          "vendor",
		VendorID:      &vendorID,
		Active:        true,
		EmailVerified: false,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		_ = s.vendorRepo.Delete(ctx, vendor.ID)
		return nil, err
	}

	loginURL := s.appBaseURL + "/admin/login"
	result := &VendorCreateResult{
		Vendor:               vendor,
		LoginURL:             loginURL,
		CredentialsEmailSent: false,
	}

	if s.emailService != nil && s.emailService.Configured() {
		if err := s.emailService.SendVendorPortalCredentials(vendor.Email, vendor.Name, loginURL, plain); err != nil {
			log.Printf("[Vendor] portal credentials email failed for %s: %v", vendor.Email, err)
			result.CredentialsEmailError = err.Error()
			result.TemporaryPassword = plain
			return result, nil
		}
		result.CredentialsEmailSent = true
		return result, nil
	}

	result.TemporaryPassword = plain
	return result, nil
}

func (s *vendorService) upgradeToVendor(ctx context.Context, existingUser *models.User, vendor *models.Vendor) (*VendorCreateResult, error) {
	if vendor.Status == "" {
		vendor.Status = "active"
	}
	if err := s.vendorRepo.Create(ctx, vendor); err != nil {
		return nil, err
	}
	vendorID := vendor.ID
	existingUser.Role = "vendor"
	existingUser.VendorID = &vendorID
	if err := s.userRepo.Update(ctx, existingUser.ID, existingUser); err != nil {
		_ = s.vendorRepo.Delete(ctx, vendor.ID)
		return nil, err
	}
	return &VendorCreateResult{
		Vendor:               vendor,
		LoginURL:             s.appBaseURL + "/admin/login",
		TemporaryPassword:    "", // existing user keeps their password
		CredentialsEmailSent: false,
	}, nil
}

func (s *vendorService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Vendor, error) {
	return s.vendorRepo.GetByID(ctx, id)
}

func (s *vendorService) Update(ctx context.Context, id primitive.ObjectID, vendor *models.Vendor) error {
	return s.vendorRepo.Update(ctx, id, vendor)
}

func (s *vendorService) Delete(ctx context.Context, id primitive.ObjectID) error {
	if u, err := s.userRepo.GetByVendorID(ctx, id); err == nil && u != nil {
		_ = s.userRepo.Delete(ctx, u.ID)
	}
	return s.vendorRepo.Delete(ctx, id)
}

func (s *vendorService) List(ctx context.Context, limit, skip int) ([]*models.Vendor, int64, error) {
	return s.vendorRepo.List(ctx, limit, skip)
}

func (s *vendorService) UpdatePermissions(ctx context.Context, id primitive.ObjectID, permissions map[string]interface{}) error {
	vendor, err := s.vendorRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	vendor.Permissions = permissions
	return s.vendorRepo.Update(ctx, id, vendor)
}
