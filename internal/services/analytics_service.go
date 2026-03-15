package services

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type AnalyticsService interface {
	TrackEvent(ctx context.Context, eventType string, userID *primitive.ObjectID, productID *primitive.ObjectID, orderID *primitive.ObjectID, metadata map[string]interface{}) error
	GetRevenueAnalytics(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)
	GetOrderAnalytics(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)
	GetProductAnalytics(ctx context.Context, startDate, endDate time.Time) ([]map[string]interface{}, error)
	GetCustomerAnalytics(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)
	GetTrafficAnalytics(ctx context.Context, startDate, endDate time.Time) ([]map[string]interface{}, error)
}

type analyticsService struct {
	analyticsRepo repositories.AnalyticsRepository
	orderRepo     repositories.OrderRepository
	productRepo   repositories.ProductRepository
}

func NewAnalyticsService(analyticsRepo repositories.AnalyticsRepository, orderRepo repositories.OrderRepository, productRepo repositories.ProductRepository) AnalyticsService {
	return &analyticsService{
		analyticsRepo: analyticsRepo,
		orderRepo:     orderRepo,
		productRepo:   productRepo,
	}
}

func (s *analyticsService) TrackEvent(ctx context.Context, eventType string, userID *primitive.ObjectID, productID *primitive.ObjectID, orderID *primitive.ObjectID, metadata map[string]interface{}) error {
	event := &models.AnalyticsEvent{
		EventType: eventType,
		UserID:    userID,
		ProductID: productID,
		OrderID:   orderID,
		Metadata:  metadata,
	}
	return s.analyticsRepo.CreateEvent(ctx, event)
}

func (s *analyticsService) GetRevenueAnalytics(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
	return s.orderRepo.GetStats(ctx, startDate, endDate)
}

func (s *analyticsService) GetOrderAnalytics(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
	stats, err := s.orderRepo.GetStats(ctx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Get orders by status
	orders, _, err := s.orderRepo.List(ctx, bson.M{
		"created_at": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	}, 1000, 0)
	if err != nil {
		return nil, err
	}

	statusCounts := make(map[string]int)
	for _, order := range orders {
		statusCounts[order.Status]++
	}

	stats["status_breakdown"] = statusCounts
	return stats, nil
}

func (s *analyticsService) GetProductAnalytics(ctx context.Context, startDate, endDate time.Time) ([]map[string]interface{}, error) {
	// This would require aggregating order items
	// For now, return empty
	return []map[string]interface{}{}, nil
}

func (s *analyticsService) GetCustomerAnalytics(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
	orders, _, err := s.orderRepo.List(ctx, bson.M{
		"created_at": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	}, 1000, 0)
	if err != nil {
		return nil, err
	}

	uniqueCustomers := make(map[string]bool)
	for _, order := range orders {
		uniqueCustomers[order.UserID.Hex()] = true
	}

	return map[string]interface{}{
		"total_customers": len(uniqueCustomers),
		"total_orders":    len(orders),
	}, nil
}

func (s *analyticsService) GetTrafficAnalytics(ctx context.Context, startDate, endDate time.Time) ([]map[string]interface{}, error) {
	return s.analyticsRepo.GetPageViews(ctx, startDate, endDate)
}

