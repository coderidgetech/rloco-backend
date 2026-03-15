package services

import (
	"context"

	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type NewsletterService interface {
	Subscribe(ctx context.Context, email string, name *string) error
	Unsubscribe(ctx context.Context, email string) error
}

type newsletterService struct {
	newsletterRepo repositories.NewsletterRepository
}

func NewNewsletterService(newsletterRepo repositories.NewsletterRepository) NewsletterService {
	return &newsletterService{
		newsletterRepo: newsletterRepo,
	}
}

func (s *newsletterService) Subscribe(ctx context.Context, email string, name *string) error {
	subscription := &models.NewsletterSubscription{
		Email: email,
		Name:  name,
	}
	return s.newsletterRepo.Create(ctx, subscription)
}

func (s *newsletterService) Unsubscribe(ctx context.Context, email string) error {
	return s.newsletterRepo.Unsubscribe(ctx, email)
}
