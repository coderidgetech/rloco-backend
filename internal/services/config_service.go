package services

import (
	"context"

	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type ConfigService interface {
	Get(ctx context.Context) (*models.SiteConfig, error)
	Update(ctx context.Context, config *models.SiteConfig) error
}

type configService struct {
	repo repositories.ConfigRepository
}

func NewConfigService(repo repositories.ConfigRepository) ConfigService {
	return &configService{repo: repo}
}

func (s *configService) Get(ctx context.Context) (*models.SiteConfig, error) {
	return s.repo.Get(ctx)
}

func (s *configService) Update(ctx context.Context, config *models.SiteConfig) error {
	return s.repo.Update(ctx, config)
}

