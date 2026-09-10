package services

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/repositories"
)

// AccountDeletionRepos groups the repositories that own per-user data not
// otherwise reachable from authService, so DeleteAccount can clean it up.
//
// Orders, payment transactions, and returns are deliberately excluded —
// those are financial/legal records retained even after the account itself
// is deleted, same as any e-commerce platform.
type AccountDeletionRepos struct {
	AddressRepo          repositories.AddressRepository
	CartRepo             repositories.CartRepository
	WishlistRepo         repositories.WishlistRepository
	ReviewRepo           repositories.ReviewRepository
	ProductRepo          repositories.ProductRepository
	RewardsRepo          repositories.RewardsRepository
	SupportRepo          repositories.SupportRepository
	AnalyticsRepo        repositories.AnalyticsRepository
	OrderIdempotencyRepo repositories.OrderIdempotencyRepository
	NewsletterRepo       repositories.NewsletterRepository
}

// cascadeDeleteUserData removes (or anonymizes) everything a user owns
// before the user document itself is deleted. It's best-effort — each step
// logs and continues on failure rather than aborting, since a partial
// cleanup is still far better than none, and the account row is only
// removed by the caller after this returns.
func (d *AccountDeletionRepos) cascadeDeleteUserData(ctx context.Context, userID primitive.ObjectID, email string) {
	if err := d.AddressRepo.DeleteByUserID(ctx, userID); err != nil {
		log.Printf("account deletion: failed to delete addresses for %s: %v", userID.Hex(), err)
	}
	if err := d.CartRepo.DeleteByUserID(ctx, userID); err != nil {
		log.Printf("account deletion: failed to delete cart for %s: %v", userID.Hex(), err)
	}
	if err := d.WishlistRepo.DeleteByUserID(ctx, userID); err != nil {
		log.Printf("account deletion: failed to delete wishlist for %s: %v", userID.Hex(), err)
	}
	if err := d.deleteUserReviews(ctx, userID); err != nil {
		log.Printf("account deletion: failed to delete reviews for %s: %v", userID.Hex(), err)
	}
	if err := d.RewardsRepo.DeleteByUserID(ctx, userID); err != nil {
		log.Printf("account deletion: failed to delete rewards history for %s: %v", userID.Hex(), err)
	}
	if err := d.SupportRepo.DeleteByUserID(ctx, userID); err != nil {
		log.Printf("account deletion: failed to delete support tickets for %s: %v", userID.Hex(), err)
	}
	if err := d.OrderIdempotencyRepo.DeleteByUserID(ctx, userID); err != nil {
		log.Printf("account deletion: failed to delete idempotency keys for %s: %v", userID.Hex(), err)
	}
	if err := d.AnalyticsRepo.AnonymizeByUserID(ctx, userID); err != nil {
		log.Printf("account deletion: failed to anonymize analytics for %s: %v", userID.Hex(), err)
	}
	if email != "" {
		if err := d.NewsletterRepo.Unsubscribe(ctx, email); err != nil {
			log.Printf("account deletion: failed to unsubscribe newsletter for %s: %v", userID.Hex(), err)
		}
	}
}

// deleteUserReviews removes every review the user wrote, going through
// reviewService.Delete (not a raw repo delete) so each product's
// denormalized rating/review-count gets recalculated correctly, the same
// way a normal single-review deletion does. A single generous-limit fetch
// (no user realistically has anywhere near this many reviews) avoids a
// refetch-the-same-page loop hazard if an individual delete keeps failing.
func (d *AccountDeletionRepos) deleteUserReviews(ctx context.Context, userID primitive.ObjectID) error {
	reviews, _, err := d.ReviewRepo.GetByUserID(ctx, userID, 1000, 0)
	if err != nil {
		return err
	}
	reviewSvc := NewReviewService(d.ReviewRepo, d.ProductRepo)
	for _, review := range reviews {
		if err := reviewSvc.Delete(ctx, review.ID, userID); err != nil {
			log.Printf("account deletion: failed to delete review %s: %v", review.ID.Hex(), err)
		}
	}
	return nil
}
