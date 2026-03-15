package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type VideoService interface {
	Create(ctx context.Context, video *models.InspirationVideo) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.InspirationVideo, error)
	Update(ctx context.Context, id primitive.ObjectID, video *models.InspirationVideo) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.InspirationVideo, int64, error)
	GetFeatured(ctx context.Context, limit int) ([]*models.InspirationVideo, error)
	GetByCategory(ctx context.Context, category string, limit int) ([]*models.InspirationVideo, error)
}

type videoService struct {
	repo repositories.VideoRepository
}

func NewVideoService(repo repositories.VideoRepository) VideoService {
	return &videoService{repo: repo}
}

func (s *videoService) Create(ctx context.Context, video *models.InspirationVideo) error {
	return s.repo.Create(ctx, video)
}

func (s *videoService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.InspirationVideo, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *videoService) Update(ctx context.Context, id primitive.ObjectID, video *models.InspirationVideo) error {
	return s.repo.Update(ctx, id, video)
}

func (s *videoService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.Delete(ctx, id)
}

func (s *videoService) List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.InspirationVideo, int64, error) {
	bsonFilter := bson.M{}
	if filter != nil {
		for k, v := range filter {
			bsonFilter[k] = v
		}
	}
	return s.repo.List(ctx, bsonFilter, limit, skip)
}

func (s *videoService) GetFeatured(ctx context.Context, limit int) ([]*models.InspirationVideo, error) {
	return s.repo.GetFeatured(ctx, limit)
}

func (s *videoService) GetByCategory(ctx context.Context, category string, limit int) ([]*models.InspirationVideo, error) {
	return s.repo.GetByCategory(ctx, category, limit)
}
