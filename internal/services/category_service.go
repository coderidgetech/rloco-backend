package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type CategoryService interface {
	Create(ctx context.Context, category *models.Category) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Category, error)
	Update(ctx context.Context, id primitive.ObjectID, category *models.Category) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	List(ctx context.Context) ([]*models.Category, error)
}

type categoryService struct {
	repo repositories.CategoryRepository
}

func NewCategoryService(repo repositories.CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) Create(ctx context.Context, category *models.Category) error {
	return s.repo.Create(ctx, category)
}

func (s *categoryService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Category, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *categoryService) Update(ctx context.Context, id primitive.ObjectID, category *models.Category) error {
	return s.repo.Update(ctx, id, category)
}

func (s *categoryService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.Delete(ctx, id)
}

func (s *categoryService) List(ctx context.Context) ([]*models.Category, error) {
	return s.repo.List(ctx)
}

