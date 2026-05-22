package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type VendorApplicationService interface {
	Submit(ctx context.Context, app *models.VendorApplication) error
	List(ctx context.Context, status string, limit, skip int) ([]*models.VendorApplication, int64, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.VendorApplication, error)
	Approve(ctx context.Context, id primitive.ObjectID, adminNotes string) error
	Reject(ctx context.Context, id primitive.ObjectID, reason string) error
}

type vendorApplicationService struct {
	repo          repositories.VendorApplicationRepository
	vendorService VendorService
	emailService  EmailService
	baseURL       string
}

func NewVendorApplicationService(
	repo repositories.VendorApplicationRepository,
	vendorService VendorService,
	emailService EmailService,
	baseURL string,
) VendorApplicationService {
	return &vendorApplicationService{
		repo:          repo,
		vendorService: vendorService,
		emailService:  emailService,
		baseURL:       baseURL,
	}
}

func (s *vendorApplicationService) Submit(ctx context.Context, app *models.VendorApplication) error {
	if err := s.repo.Create(ctx, app); err != nil {
		return err
	}
	go s.emailService.SendVendorApplicationReceived(app.Email, app.BusinessName)
	return nil
}

func (s *vendorApplicationService) List(ctx context.Context, status string, limit, skip int) ([]*models.VendorApplication, int64, error) {
	return s.repo.List(ctx, status, limit, skip)
}

func (s *vendorApplicationService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.VendorApplication, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *vendorApplicationService) Approve(ctx context.Context, id primitive.ObjectID, adminNotes string) error {
	app, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	tempPassword := generateTempPassword()

	vendor := &models.Vendor{
		Name:   app.BusinessName,
		Email:  app.Email,
		Status: "active",
	}
	result, err := s.vendorService.Create(ctx, vendor, tempPassword)
	if err != nil {
		return err
	}

	if err := s.repo.UpdateStatus(ctx, id, "approved", adminNotes); err != nil {
		return err
	}

	loginURL := fmt.Sprintf("%s/admin/login", strings.TrimRight(s.baseURL, "/"))
	go s.emailService.SendVendorApplicationApproved(app.Email, app.BusinessName, loginURL, result.TemporaryPassword)
	return nil
}

func (s *vendorApplicationService) Reject(ctx context.Context, id primitive.ObjectID, reason string) error {
	app, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateStatus(ctx, id, "rejected", reason); err != nil {
		return err
	}
	go s.emailService.SendVendorApplicationRejected(app.Email, app.BusinessName, reason)
	return nil
}

func generateTempPassword() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%X", b)
}
