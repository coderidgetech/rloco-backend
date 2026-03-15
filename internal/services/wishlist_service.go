package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type WishlistService interface {
	Get(ctx context.Context, userID primitive.ObjectID) ([]*models.Product, error)
	Add(ctx context.Context, userID, productID primitive.ObjectID) error
	Remove(ctx context.Context, userID, productID primitive.ObjectID) error
	IsInWishlist(ctx context.Context, userID, productID primitive.ObjectID) (bool, error)
	GetProductAnalytics(ctx context.Context, productID primitive.ObjectID) (map[string]interface{}, error)
	GetUserAnalytics(ctx context.Context) (map[string]interface{}, error)
}

type wishlistService struct {
	wishlistRepo repositories.WishlistRepository
	productRepo  repositories.ProductRepository
}

func NewWishlistService(wishlistRepo repositories.WishlistRepository, productRepo repositories.ProductRepository) WishlistService {
	return &wishlistService{
		wishlistRepo: wishlistRepo,
		productRepo:  productRepo,
	}
}

func (s *wishlistService) Get(ctx context.Context, userID primitive.ObjectID) ([]*models.Product, error) {
	wishlists, err := s.wishlistRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var products []*models.Product
	for _, wishlist := range wishlists {
		product, err := s.productRepo.GetByID(ctx, wishlist.ProductID)
		if err != nil {
			continue
		}
		products = append(products, product)
	}

	return products, nil
}

func (s *wishlistService) Add(ctx context.Context, userID, productID primitive.ObjectID) error {
	wishlist := &models.Wishlist{
		UserID:    userID,
		ProductID: productID,
	}
	return s.wishlistRepo.Add(ctx, wishlist)
}

func (s *wishlistService) Remove(ctx context.Context, userID, productID primitive.ObjectID) error {
	return s.wishlistRepo.Remove(ctx, userID, productID)
}

func (s *wishlistService) IsInWishlist(ctx context.Context, userID, productID primitive.ObjectID) (bool, error) {
	return s.wishlistRepo.IsInWishlist(ctx, userID, productID)
}

func (s *wishlistService) GetProductAnalytics(ctx context.Context, productID primitive.ObjectID) (map[string]interface{}, error) {
	wishlistCount, err := s.wishlistRepo.GetProductWishlistCount(ctx, productID)
	if err != nil {
		return nil, err
	}
	uniqueUsers, err := s.wishlistRepo.GetUniqueUsersCount(ctx, productID)
	if err != nil {
		return nil, err
	}

	// Calculate purchase conversion (placeholder - would need order data)
	// For now, return 0
	purchaseConversion := 0.0

	return map[string]interface{}{
		"product_id":         productID.Hex(),
		"wishlist_count":     wishlistCount,
		"unique_users":       uniqueUsers,
		"purchase_conversion": purchaseConversion,
		"trend":              "stable", // Would need historical data
		"trend_percent":      0.0,
	}, nil
}

func (s *wishlistService) GetUserAnalytics(ctx context.Context) (map[string]interface{}, error) {
	// Placeholder - would need to aggregate user wishlist data
	return map[string]interface{}{
		"total_wishlists": 0,
		"active_users":    0,
	}, nil
}
