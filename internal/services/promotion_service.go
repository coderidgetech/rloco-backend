package services

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type PromotionService interface {
	Create(ctx context.Context, promotion *models.Promotion) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Promotion, error)
	Update(ctx context.Context, id primitive.ObjectID, promotion *models.Promotion) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, activeOnly bool) ([]*models.Promotion, error)
	Validate(ctx context.Context, code string, subtotal float64) (*models.Promotion, float64, error)
}

type promotionService struct {
	repo repositories.PromotionRepository
}

func NewPromotionService(repo repositories.PromotionRepository) PromotionService {
	return &promotionService{repo: repo}
}

func (s *promotionService) Create(ctx context.Context, promotion *models.Promotion) error {
	return s.repo.Create(ctx, promotion)
}

func (s *promotionService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Promotion, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *promotionService) Update(ctx context.Context, id primitive.ObjectID, promotion *models.Promotion) error {
	return s.repo.Update(ctx, id, promotion)
}

func (s *promotionService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.Delete(ctx, id)
}

func (s *promotionService) List(ctx context.Context, activeOnly bool) ([]*models.Promotion, error) {
	return s.repo.List(ctx, activeOnly)
}

func (s *promotionService) Validate(ctx context.Context, code string, subtotal float64) (*models.Promotion, float64, error) {
	promotion, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, 0, errors.New("invalid promotion code")
	}

	now := time.Now()
	if !promotion.IsActive {
		return nil, 0, errors.New("promotion is not active")
	}

	if now.Before(promotion.StartDate) || now.After(promotion.EndDate) {
		return nil, 0, errors.New("promotion is not valid at this time")
	}

	if promotion.MinPurchase != nil && subtotal < *promotion.MinPurchase {
		return nil, 0, errors.New("minimum purchase amount not met")
	}

	if promotion.UsageLimit != nil && promotion.UsageCount >= *promotion.UsageLimit {
		return nil, 0, errors.New("promotion usage limit reached")
	}

	var discount float64
	if promotion.Type == "percentage" {
		discount = subtotal * (promotion.Value / 100)
		if promotion.MaxDiscount != nil && discount > *promotion.MaxDiscount {
			discount = *promotion.MaxDiscount
		}
	} else if promotion.Type == "fixed" {
		// Cap fixed discount at subtotal so the order total never goes negative
		if promotion.Value > subtotal {
			discount = subtotal
		} else {
			discount = promotion.Value
		}
	} else if promotion.Type == "free_shipping" {
		discount = 0 // Free shipping is handled separately
	}

	return promotion, discount, nil
}

