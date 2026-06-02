package services

import (
	"context"
	"time"

	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type InventoryService interface {
	CheckLowStock(ctx context.Context, threshold int) ([]models.LowStockItem, error)
	GetStockAlerts(ctx context.Context, threshold int) ([]StockAlert, error)
}

// StockAlert wraps a LowStockItem with a categorised alert_type for the admin UI.
type StockAlert struct {
	ProductID   string    `json:"product_id"`
	ProductName string    `json:"product_name"`
	SKU         string    `json:"sku"`
	Size        string    `json:"size"`
	Stock       int       `json:"stock"`
	AlertType   string    `json:"alert_type"` // "out_of_stock" | "low"
	CreatedAt   time.Time `json:"created_at"`
}

type inventoryService struct {
	productRepo repositories.ProductRepository
}

func NewInventoryService(productRepo repositories.ProductRepository) InventoryService {
	return &inventoryService{productRepo: productRepo}
}

// CheckLowStock returns one row per product×size whose qty ≤ threshold.
// Filtering is done at the DB level via an aggregation pipeline.
func (s *inventoryService) CheckLowStock(ctx context.Context, threshold int) ([]models.LowStockItem, error) {
	items, err := s.productRepo.LowStockItems(ctx, threshold)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.LowStockItem{}
	}
	return items, nil
}

// GetStockAlerts derives alert rows from the low-stock scan so we never do two
// full product scans.  threshold is respected (no more hard-coded 10).
func (s *inventoryService) GetStockAlerts(ctx context.Context, threshold int) ([]StockAlert, error) {
	items, err := s.CheckLowStock(ctx, threshold)
	if err != nil {
		return nil, err
	}

	alerts := make([]StockAlert, 0, len(items))
	now := time.Now()
	for _, item := range items {
		alertType := "low"
		if item.Stock == 0 {
			alertType = "out_of_stock"
		}
		alerts = append(alerts, StockAlert{
			ProductID:   item.ProductID.Hex(),
			ProductName: item.Name,
			SKU:         item.SKU,
			Size:        item.Size,
			Stock:       item.Stock,
			AlertType:   alertType,
			CreatedAt:   now,
		})
	}
	return alerts, nil
}
