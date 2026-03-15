package services

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/repositories"
)

type InventoryService interface {
	CheckLowStock(ctx context.Context, threshold int) ([]LowStockItem, error)
	GetStockAlerts(ctx context.Context) ([]StockAlert, error)
}

type LowStockItem struct {
	ProductID   primitive.ObjectID `json:"product_id"`
	ProductName string            `json:"product_name"`
	Size        string            `json:"size"`
	CurrentStock int              `json:"current_stock"`
	Threshold   int               `json:"threshold"`
}

type StockAlert struct {
	ProductID   primitive.ObjectID `json:"product_id"`
	ProductName string            `json:"product_name"`
	Size        string            `json:"size"`
	Stock       int               `json:"stock"`
	AlertType   string            `json:"alert_type"` // "low", "out_of_stock"
	CreatedAt   time.Time         `json:"created_at"`
}

type inventoryService struct {
	productRepo repositories.ProductRepository
}

func NewInventoryService(productRepo repositories.ProductRepository) InventoryService {
	return &inventoryService{
		productRepo: productRepo,
	}
}

func (s *inventoryService) CheckLowStock(ctx context.Context, threshold int) ([]LowStockItem, error) {
	// Get all products
	products, _, err := s.productRepo.List(ctx, bson.M{}, 1000, 0, bson.M{"created_at": -1})
	if err != nil {
		return nil, err
	}

	var lowStockItems []LowStockItem
	for _, product := range products {
		for size, stock := range product.Stock {
			if stock <= threshold {
				lowStockItems = append(lowStockItems, LowStockItem{
					ProductID:   product.ID,
					ProductName: product.Name,
					Size:        size,
					CurrentStock: stock,
					Threshold:   threshold,
				})
			}
		}
	}

	return lowStockItems, nil
}

func (s *inventoryService) GetStockAlerts(ctx context.Context) ([]StockAlert, error) {
	lowStock, err := s.CheckLowStock(ctx, 10) // Default threshold: 10
	if err != nil {
		return nil, err
	}

	alerts := make([]StockAlert, 0, len(lowStock))
	for _, item := range lowStock {
		alertType := "low"
		if item.CurrentStock == 0 {
			alertType = "out_of_stock"
		}

		alerts = append(alerts, StockAlert{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Size:        item.Size,
			Stock:       item.CurrentStock,
			AlertType:   alertType,
			CreatedAt:   time.Now(),
		})
	}

	return alerts, nil
}
