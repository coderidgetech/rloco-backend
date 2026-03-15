package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type VendorService interface {
	Create(ctx context.Context, vendor *models.Vendor) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Vendor, error)
	Update(ctx context.Context, id primitive.ObjectID, vendor *models.Vendor) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, limit, skip int) ([]*models.Vendor, int64, error)
	UpdatePermissions(ctx context.Context, id primitive.ObjectID, permissions map[string]interface{}) error
}

type vendorService struct {
	vendorRepo repositories.VendorRepository
	userRepo   repositories.UserRepository
}

func NewVendorService(vendorRepo repositories.VendorRepository, userRepo repositories.UserRepository) VendorService {
	return &vendorService{
		vendorRepo: vendorRepo,
		userRepo:   userRepo,
	}
}

func (s *vendorService) Create(ctx context.Context, vendor *models.Vendor) error {
	return s.vendorRepo.Create(ctx, vendor)
}

func (s *vendorService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Vendor, error) {
	return s.vendorRepo.GetByID(ctx, id)
}

func (s *vendorService) Update(ctx context.Context, id primitive.ObjectID, vendor *models.Vendor) error {
	return s.vendorRepo.Update(ctx, id, vendor)
}

func (s *vendorService) Delete(ctx context.Context, id primitive.ObjectID) error {
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

