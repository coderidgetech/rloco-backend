package services

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type CartService interface {
	Get(ctx context.Context, userID primitive.ObjectID) (*models.Cart, error)
	AddItem(ctx context.Context, userID primitive.ObjectID, item models.CartItem) error
	UpdateItem(ctx context.Context, userID primitive.ObjectID, productID primitive.ObjectID, size string, quantity int) error
	UpdateItemGiftOptions(ctx context.Context, userID primitive.ObjectID, productID primitive.ObjectID, size, giftWrapColor, giftMessage string) error
	RemoveItem(ctx context.Context, userID primitive.ObjectID, productID primitive.ObjectID, size string) error
	Clear(ctx context.Context, userID primitive.ObjectID) error
}

type cartService struct {
	cartRepo    repositories.CartRepository
	productRepo repositories.ProductRepository
}

func NewCartService(cartRepo repositories.CartRepository, productRepo repositories.ProductRepository) CartService {
	return &cartService{
		cartRepo:    cartRepo,
		productRepo: productRepo,
	}
}

func (s *cartService) Get(ctx context.Context, userID primitive.ObjectID) (*models.Cart, error) {
	return s.cartRepo.GetByUserID(ctx, userID)
}

func (s *cartService) AddItem(ctx context.Context, userID primitive.ObjectID, item models.CartItem) error {
	product, err := s.productRepo.GetByID(ctx, item.ProductID)
	if err != nil {
		return errors.New("product not found")
	}
	if item.Quantity <= 0 {
		return errors.New("quantity must be at least 1")
	}
	if product.Stock[item.Size] < item.Quantity {
		return errors.New("insufficient stock")
	}
	// Server-side authoritative pricing/details.
	item.ProductName = product.Name
	if len(product.Images) > 0 {
		item.Image = product.Images[0]
	}
	item.Price = product.Price
	item.PriceINR = product.PriceINR

	cart, err := s.cartRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// Check if item already exists
	for i, existingItem := range cart.Items {
		if existingItem.ProductID == item.ProductID && existingItem.Size == item.Size {
			cart.Items[i].Quantity += item.Quantity
			return s.cartRepo.Update(ctx, cart)
		}
	}

	// Add new item
	cart.Items = append(cart.Items, item)
	return s.cartRepo.Update(ctx, cart)
}

func (s *cartService) UpdateItem(ctx context.Context, userID primitive.ObjectID, productID primitive.ObjectID, size string, quantity int) error {
	if quantity > 0 {
		product, err := s.productRepo.GetByID(ctx, productID)
		if err != nil {
			return errors.New("product not found")
		}
		if product.Stock[size] < quantity {
			return errors.New("insufficient stock")
		}
	}

	cart, err := s.cartRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	for i, item := range cart.Items {
		if item.ProductID == productID && item.Size == size {
			if quantity <= 0 {
				// Remove item
				cart.Items = append(cart.Items[:i], cart.Items[i+1:]...)
			} else {
				cart.Items[i].Quantity = quantity
			}
			return s.cartRepo.Update(ctx, cart)
		}
	}

	return nil
}

func (s *cartService) UpdateItemGiftOptions(ctx context.Context, userID primitive.ObjectID, productID primitive.ObjectID, size, giftWrapColor, giftMessage string) error {
	cart, err := s.cartRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for i, item := range cart.Items {
		if item.ProductID == productID && item.Size == size {
			cart.Items[i].IsGift = true
			cart.Items[i].GiftWrapColor = giftWrapColor
			cart.Items[i].GiftMessage = giftMessage
			return s.cartRepo.Update(ctx, cart)
		}
	}
	return nil
}

func (s *cartService) RemoveItem(ctx context.Context, userID primitive.ObjectID, productID primitive.ObjectID, size string) error {
	cart, err := s.cartRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	for i, item := range cart.Items {
		if item.ProductID == productID && item.Size == size {
			cart.Items = append(cart.Items[:i], cart.Items[i+1:]...)
			return s.cartRepo.Update(ctx, cart)
		}
	}

	return nil
}

func (s *cartService) Clear(ctx context.Context, userID primitive.ObjectID) error {
	return s.cartRepo.Clear(ctx, userID)
}

