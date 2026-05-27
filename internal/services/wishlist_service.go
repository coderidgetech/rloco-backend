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
	orderRepo    repositories.OrderRepository
}

func NewWishlistService(wishlistRepo repositories.WishlistRepository, productRepo repositories.ProductRepository, orderRepo repositories.OrderRepository) WishlistService {
	return &wishlistService{
		wishlistRepo: wishlistRepo,
		productRepo:  productRepo,
		orderRepo:    orderRepo,
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

	purchaseConversion := 0.0
	if uniqueUsers > 0 {
		wishlistUserIDs, err := s.wishlistRepo.GetUserIDsByProduct(ctx, productID)
		if err == nil && len(wishlistUserIDs) > 0 {
			buyers, err := s.orderRepo.CountBuyersOfProductFromUsers(ctx, productID, wishlistUserIDs)
			if err == nil {
				purchaseConversion = float64(buyers) / float64(uniqueUsers) * 100
			}
		}
	}

	return map[string]interface{}{
		"product_id":          productID.Hex(),
		"wishlist_count":      wishlistCount,
		"unique_users":        uniqueUsers,
		"purchase_conversion": purchaseConversion,
		"trend":               "stable",
		"trend_percent":       0.0,
	}, nil
}

func (s *wishlistService) GetUserAnalytics(ctx context.Context) (map[string]interface{}, error) {
	totalWishlists, err := s.wishlistRepo.GetTotalCount(ctx)
	if err != nil {
		return nil, err
	}
	activeUsers, err := s.wishlistRepo.GetActiveUsersCount(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total_wishlists": totalWishlists,
		"active_users":    activeUsers,
	}, nil
}
