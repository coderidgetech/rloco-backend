package services

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type ReviewService interface {
	Create(ctx context.Context, productID, userID primitive.ObjectID, userName string, rating int, title, comment string, images []string, verified bool) (*models.ProductReview, error)
	GetByProductID(ctx context.Context, productID primitive.ObjectID, limit, skip int) ([]*models.ProductReview, int64, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.ProductReview, int64, error)
	Update(ctx context.Context, id, userID primitive.ObjectID, title, comment string, images []string) (*models.ProductReview, error)
	Delete(ctx context.Context, id, userID primitive.ObjectID) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	IncrementHelpful(ctx context.Context, id primitive.ObjectID) error
	RecalculateProductRating(ctx context.Context, productID primitive.ObjectID) error
}

type reviewService struct {
	reviewRepo  repositories.ReviewRepository
	productRepo repositories.ProductRepository
}

func NewReviewService(reviewRepo repositories.ReviewRepository, productRepo repositories.ProductRepository) ReviewService {
	return &reviewService{
		reviewRepo:  reviewRepo,
		productRepo: productRepo,
	}
}

func (s *reviewService) Create(ctx context.Context, productID, userID primitive.ObjectID, userName string, rating int, title, comment string, images []string, verified bool) (*models.ProductReview, error) {
	// Validate rating
	if rating < 1 || rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}

	// Check if product exists
	_, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		return nil, errors.New("product not found")
	}

	// Check if user already reviewed this product
	existingReviews, _, err := s.reviewRepo.GetByUserID(ctx, userID, 100, 0)
	if err == nil {
		for _, review := range existingReviews {
			if review.ProductID.Hex() == productID.Hex() {
				return nil, errors.New("you have already reviewed this product")
			}
		}
	}

	review := &models.ProductReview{
		ProductID: productID,
		UserID:    userID,
		UserName:  userName,
		Rating:    rating,
		Title:     title,
		Comment:   comment,
		Images:    images,
		Verified:  verified,
		Helpful:   0,
		Status:    "pending",
	}

	if err := s.reviewRepo.Create(ctx, review); err != nil {
		return nil, err
	}

	return review, nil
}

func (s *reviewService) GetByProductID(ctx context.Context, productID primitive.ObjectID, limit, skip int) ([]*models.ProductReview, int64, error) {
	return s.reviewRepo.GetByProductID(ctx, productID, limit, skip)
}

func (s *reviewService) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.ProductReview, int64, error) {
	return s.reviewRepo.GetByUserID(ctx, userID, limit, skip)
}

func (s *reviewService) Update(ctx context.Context, id, userID primitive.ObjectID, title, comment string, images []string) (*models.ProductReview, error) {
	review, err := s.reviewRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("review not found")
	}

	// Check ownership
	if review.UserID.Hex() != userID.Hex() {
		return nil, errors.New("you can only update your own reviews")
	}

	// Check if review is approved (can't edit approved reviews)
	if review.Status == "approved" {
		return nil, errors.New("cannot edit approved reviews")
	}

	review.Title = title
	review.Comment = comment
	review.Images = images

	if err := s.reviewRepo.Update(ctx, id, review); err != nil {
		return nil, err
	}

	return review, nil
}

func (s *reviewService) Delete(ctx context.Context, id, userID primitive.ObjectID) error {
	review, err := s.reviewRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("review not found")
	}

	// Check ownership
	if review.UserID.Hex() != userID.Hex() {
		return errors.New("you can only delete your own reviews")
	}

	productID := review.ProductID
	if err := s.reviewRepo.Delete(ctx, id); err != nil {
		return err
	}

	// Recalculate product rating
	return s.RecalculateProductRating(ctx, productID)
}

func (s *reviewService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	validStatuses := map[string]bool{
		"pending":  true,
		"approved": true,
		"rejected": true,
	}
	if !validStatuses[status] {
		return errors.New("invalid status")
	}

	if err := s.reviewRepo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}

	// If approved, recalculate product rating
	if status == "approved" {
		review, err := s.reviewRepo.GetByID(ctx, id)
		if err == nil {
			return s.RecalculateProductRating(ctx, review.ProductID)
		}
	}

	return nil
}

func (s *reviewService) IncrementHelpful(ctx context.Context, id primitive.ObjectID) error {
	return s.reviewRepo.IncrementHelpful(ctx, id)
}

func (s *reviewService) RecalculateProductRating(ctx context.Context, productID primitive.ObjectID) error {
	rating, count, err := s.reviewRepo.GetProductRating(ctx, productID)
	if err != nil {
		return err
	}

	// Update product rating and review count
	product, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		return err
	}

	product.Rating = rating
	product.Reviews = count

	return s.productRepo.Update(ctx, productID, product)
}
