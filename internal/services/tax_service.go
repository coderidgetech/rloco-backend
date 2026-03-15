package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type TaxService interface {
	CalculateTax(ctx context.Context, country, state, city, postalCode string, subtotal float64) (float64, *models.TaxRate, error)
	GetRates(ctx context.Context, activeOnly bool) ([]*models.TaxRate, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.TaxRate, error)
	Create(ctx context.Context, rate *models.TaxRate) error
	Update(ctx context.Context, id primitive.ObjectID, rate *models.TaxRate) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type taxService struct {
	taxRepo repositories.TaxRepository
}

func NewTaxService(taxRepo repositories.TaxRepository) TaxService {
	return &taxService{
		taxRepo: taxRepo,
	}
}

func (s *taxService) CalculateTax(ctx context.Context, country, state, city, postalCode string, subtotal float64) (float64, *models.TaxRate, error) {
	rate, err := s.taxRepo.GetByLocation(ctx, country, state, city, postalCode)
	if err != nil {
		return 0, nil, err
	}

	taxAmount := subtotal * (rate.Rate / 100)
	return taxAmount, rate, nil
}

func (s *taxService) GetRates(ctx context.Context, activeOnly bool) ([]*models.TaxRate, error) {
	return s.taxRepo.List(ctx, activeOnly)
}

func (s *taxService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.TaxRate, error) {
	return s.taxRepo.GetByID(ctx, id)
}

func (s *taxService) Create(ctx context.Context, rate *models.TaxRate) error {
	return s.taxRepo.Create(ctx, rate)
}

func (s *taxService) Update(ctx context.Context, id primitive.ObjectID, rate *models.TaxRate) error {
	return s.taxRepo.Update(ctx, id, rate)
}

func (s *taxService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.taxRepo.Delete(ctx, id)
}
