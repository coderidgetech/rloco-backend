package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type AddressService interface {
	Create(ctx context.Context, address *models.Address) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Address, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Address, error)
	Update(ctx context.Context, id primitive.ObjectID, address *models.Address) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	SetDefault(ctx context.Context, userID primitive.ObjectID, addressID primitive.ObjectID) error
}

type addressService struct {
	repo repositories.AddressRepository
}

func NewAddressService(repo repositories.AddressRepository) AddressService {
	return &addressService{repo: repo}
}

func (s *addressService) Create(ctx context.Context, address *models.Address) error {
	return s.repo.Create(ctx, address)
}

func (s *addressService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Address, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *addressService) GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Address, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *addressService) Update(ctx context.Context, id primitive.ObjectID, address *models.Address) error {
	return s.repo.Update(ctx, id, address)
}

func (s *addressService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.Delete(ctx, id)
}

func (s *addressService) SetDefault(ctx context.Context, userID primitive.ObjectID, addressID primitive.ObjectID) error {
	return s.repo.SetDefault(ctx, userID, addressID)
}
