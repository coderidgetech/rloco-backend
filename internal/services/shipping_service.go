package services

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type ShippingService interface {
	CalculateShipping(ctx context.Context, req ShippingQuoteRequest) ([]*models.ShippingMethod, error)
	GetMethods(ctx context.Context, activeOnly bool) ([]*models.ShippingMethod, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.ShippingMethod, error)
	Create(ctx context.Context, method *models.ShippingMethod) error
	Update(ctx context.Context, id primitive.ObjectID, method *models.ShippingMethod) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type ShippingQuoteRequest struct {
	Country    string
	State      string
	City       string
	Address    string
	PostalCode string
	FirstName  string
	LastName   string
	Email      string
	Phone      string
	Subtotal   float64
	Weight     *float64
}

type shippingService struct {
	shippingRepo repositories.ShippingRepository
	shippo       *shippoClient
}

func NewShippingService(shippingRepo repositories.ShippingRepository, shippo *shippoClient) ShippingService {
	return &shippingService{
		shippingRepo: shippingRepo,
		shippo:       shippo,
	}
}

func (s *shippingService) CalculateShipping(ctx context.Context, req ShippingQuoteRequest) ([]*models.ShippingMethod, error) {
	if methods, err := s.calculateShippoShipping(ctx, req); err == nil && len(methods) > 0 {
		return methods, nil
	}

	return s.calculateStoredShipping(ctx, req.Country, req.Subtotal, req.Weight)
}

func (s *shippingService) calculateShippoShipping(ctx context.Context, req ShippingQuoteRequest) ([]*models.ShippingMethod, error) {
	if s.shippo == nil {
		return nil, nil
	}

	destination := shippoAddress{
		Name:    firstNonEmpty(strings.TrimSpace(req.FirstName+" "+req.LastName), "Customer"),
		Email:   strings.TrimSpace(req.Email),
		Phone:   strings.TrimSpace(req.Phone),
		Street1: strings.TrimSpace(req.Address),
		City:    strings.TrimSpace(req.City),
		State:   strings.TrimSpace(req.State),
		Zip:     strings.TrimSpace(req.PostalCode),
		Country: normalizeCountryCode(req.Country),
	}

	if !isCompleteShippoAddress(destination) {
		return nil, nil
	}

	return s.shippo.quoteRates(ctx, destination)
}

func (s *shippingService) calculateStoredShipping(ctx context.Context, country string, subtotal float64, weight *float64) ([]*models.ShippingMethod, error) {
	methods, err := s.shippingRepo.GetByZone(ctx, country)
	if err != nil {
		// If no zone-specific methods, get all active methods
		methods, err = s.shippingRepo.List(ctx, true)
		if err != nil {
			return nil, err
		}
	}

	// Calculate costs for each method
	var availableMethods []*models.ShippingMethod
	for _, method := range methods {
		methodCopy := *method

		// Check free shipping threshold
		if methodCopy.FreeShippingThreshold != nil && subtotal >= *methodCopy.FreeShippingThreshold {
			methodCopy.BaseCost = 0
		}

		// Calculate weight-based cost if applicable
		if methodCopy.CostPerKg != nil && weight != nil {
			methodCopy.BaseCost += *methodCopy.CostPerKg * *weight
		}

		// Find zone-specific cost
		for _, zone := range methodCopy.Zones {
			for _, zoneCountry := range zone.Countries {
				if strings.EqualFold(normalizeCountryCode(zoneCountry), normalizeCountryCode(country)) {
					methodCopy.BaseCost = zone.Cost
					break
				}
			}
		}

		if methodCopy.Currency == "" {
			methodCopy.Currency = "USD"
		}

		availableMethods = append(availableMethods, &methodCopy)
	}

	if len(availableMethods) == 0 {
		// Return default shipping method
		return []*models.ShippingMethod{
			{
				Name:          "Standard Shipping",
				Type:          "standard",
				BaseCost:      15.0,
				Currency:      "USD",
				EstimatedDays: 5,
				IsActive:      true,
			},
		}, nil
	}

	return availableMethods, nil
}

func (s *shippingService) GetMethods(ctx context.Context, activeOnly bool) ([]*models.ShippingMethod, error) {
	return s.shippingRepo.List(ctx, activeOnly)
}

func (s *shippingService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.ShippingMethod, error) {
	return s.shippingRepo.GetByID(ctx, id)
}

func (s *shippingService) Create(ctx context.Context, method *models.ShippingMethod) error {
	return s.shippingRepo.Create(ctx, method)
}

func (s *shippingService) Update(ctx context.Context, id primitive.ObjectID, method *models.ShippingMethod) error {
	return s.shippingRepo.Update(ctx, id, method)
}

func (s *shippingService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.shippingRepo.Delete(ctx, id)
}
