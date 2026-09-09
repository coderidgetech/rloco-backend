package services

import (
	"context"
	"errors"
	"time"

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
	IncrementHelpful(ctx context.Context, id, userID primitive.ObjectID) (bool, error)
	// ReportReview records userID's report against a review; once
	// reportThreshold distinct reports land, an approved review is pulled
	// back to "pending" for admin re-review. Returns false (no error) if
	// userID already reported this review.
	ReportReview(ctx context.Context, id, userID primitive.ObjectID) (bool, error)
	RecalculateProductRating(ctx context.Context, productID primitive.ObjectID) error
	ListByStatus(ctx context.Context, status string, limit, skip int) ([]*models.ProductReview, int64, error)
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

func (s *reviewService) Create(ctx context.Context, productID, userID primitive.ObjectID, userName string, rating int, title, comment string, images []string, _verifiedIgnored bool) (*models.ProductReview, error) {
	// Validate rating
	if rating < 1 || rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}

	// Check if product exists
	_, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		return nil, errors.New("product not found")
	}

	// Check if user already reviewed this product, and count this user's
	// very recent reviews for the fraud-velocity check below.
	existingReviews, _, err := s.reviewRepo.GetByUserID(ctx, userID, 100, 0)
	recentCount := 0
	if err == nil {
		cutoff := time.Now().Add(-fraudVelocityWindow)
		for _, review := range existingReviews {
			if review.ProductID.Hex() == productID.Hex() {
				return nil, errors.New("you have already reviewed this product")
			}
			if review.CreatedAt.After(cutoff) {
				recentCount++
			}
		}
	}

	// Obvious spam/profanity never gets published — reject at submission
	// rather than queuing it for a human to look at.
	if isFlaggedContent(title, comment) {
		return nil, errors.New("your review contains content that isn't allowed; please revise and resubmit")
	}

	// Auto-approve by default (matches Myntra/Amazon/Flipkart-style instant
	// publish); a burst of reviews from the same account in a short window
	// looks bot-like, so hold those for manual approval instead.
	status := "approved"
	if recentCount >= fraudVelocityLimit-1 {
		status = "pending"
	}

	review := &models.ProductReview{
		ProductID: productID,
		UserID:    userID,
		UserName:  userName,
		Rating:    rating,
		Title:     title,
		Comment:   comment,
		Images:    images,
		Verified:  false, // set only server-side when tied to a verified purchase
		Helpful:   0,
		Status:    status,
	}

	if err := s.reviewRepo.Create(ctx, review); err != nil {
		return nil, err
	}

	if status == "approved" {
		if err := s.RecalculateProductRating(ctx, productID); err != nil {
			return nil, err
		}
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

	// Re-run the content filter on the edit — an approved review stays
	// approved, but editing spam/profanity past the filter isn't allowed.
	if isFlaggedContent(title, comment) {
		return nil, errors.New("your review contains content that isn't allowed; please revise and resubmit")
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

func (s *reviewService) IncrementHelpful(ctx context.Context, id, userID primitive.ObjectID) (bool, error) {
	return s.reviewRepo.IncrementHelpful(ctx, id, userID)
}

func (s *reviewService) ReportReview(ctx context.Context, id, userID primitive.ObjectID) (bool, error) {
	reported, newCount, err := s.reviewRepo.ReportReview(ctx, id, userID)
	if err != nil || !reported {
		return reported, err
	}

	if newCount >= reportThreshold {
		review, err := s.reviewRepo.GetByID(ctx, id)
		if err == nil && review.Status == "approved" {
			if err := s.reviewRepo.UpdateStatus(ctx, id, "pending"); err != nil {
				return true, err
			}
			return true, s.RecalculateProductRating(ctx, review.ProductID)
		}
	}

	return true, nil
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

func (s *reviewService) ListByStatus(ctx context.Context, status string, limit, skip int) ([]*models.ProductReview, int64, error) {
	return s.reviewRepo.ListByStatus(ctx, status, limit, skip)
}
