package services

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type SupportService interface {
	CreateTicket(ctx context.Context, userID primitive.ObjectID, orderID *primitive.ObjectID, subject, category, priority, message string) (*models.SupportTicket, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.SupportTicket, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.SupportTicket, int64, error)
	List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.SupportTicket, int64, error)
	AddMessage(ctx context.Context, ticketID, userID primitive.ObjectID, message string, isAdmin bool, attachments []string) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	Assign(ctx context.Context, id, assignedTo primitive.ObjectID) error
}

type supportService struct {
	supportRepo repositories.SupportRepository
}

func NewSupportService(supportRepo repositories.SupportRepository) SupportService {
	return &supportService{
		supportRepo: supportRepo,
	}
}

func (s *supportService) CreateTicket(ctx context.Context, userID primitive.ObjectID, orderID *primitive.ObjectID, subject, category, priority, message string) (*models.SupportTicket, error) {
	validCategories := map[string]bool{
		"order":    true,
		"product":  true,
		"payment":  true,
		"shipping": true,
		"other":    true,
	}
	if !validCategories[category] {
		return nil, errors.New("invalid category")
	}

	validPriorities := map[string]bool{
		"low":    true,
		"medium": true,
		"high":   true,
		"urgent": true,
	}
	if !validPriorities[priority] {
		return nil, errors.New("invalid priority")
	}

	ticket := &models.SupportTicket{
		UserID:    userID,
		OrderID:   orderID,
		Subject:   subject,
		Category:  category,
		Priority:  priority,
		Status:    "open",
		Messages:  []models.TicketMessage{},
	}

	// Add initial message
	initialMessage := models.TicketMessage{
		ID:        primitive.NewObjectID(),
		UserID:    userID,
		IsAdmin:   false,
		Message:   message,
		CreatedAt: ticket.CreatedAt,
	}
	ticket.Messages = append(ticket.Messages, initialMessage)

	if err := s.supportRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

func (s *supportService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.SupportTicket, error) {
	return s.supportRepo.GetByID(ctx, id)
}

func (s *supportService) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.SupportTicket, int64, error) {
	return s.supportRepo.GetByUserID(ctx, userID, limit, skip)
}

func (s *supportService) List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.SupportTicket, int64, error) {
	bsonFilter := bson.M{}
	if status, ok := filter["status"].(string); ok && status != "" {
		bsonFilter["status"] = status
	}
	if category, ok := filter["category"].(string); ok && category != "" {
		bsonFilter["category"] = category
	}
	if priority, ok := filter["priority"].(string); ok && priority != "" {
		bsonFilter["priority"] = priority
	}
	return s.supportRepo.List(ctx, bsonFilter, limit, skip)
}

func (s *supportService) AddMessage(ctx context.Context, ticketID, userID primitive.ObjectID, message string, isAdmin bool, attachments []string) error {
	ticket, err := s.supportRepo.GetByID(ctx, ticketID)
	if err != nil {
		return errors.New("ticket not found")
	}

	// Check ownership (users can only add messages to their own tickets, admins can add to any)
	if !isAdmin && ticket.UserID.Hex() != userID.Hex() {
		return errors.New("access denied")
	}

	ticketMessage := &models.TicketMessage{
		UserID:      userID,
		IsAdmin:     isAdmin,
		Message:     message,
		Attachments: attachments,
	}

	return s.supportRepo.AddMessage(ctx, ticketID, ticketMessage)
}

func (s *supportService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	validStatuses := map[string]bool{
		"open":        true,
		"in_progress": true,
		"resolved":    true,
		"closed":      true,
	}
	if !validStatuses[status] {
		return errors.New("invalid status")
	}
	return s.supportRepo.UpdateStatus(ctx, id, status)
}

func (s *supportService) Assign(ctx context.Context, id, assignedTo primitive.ObjectID) error {
	return s.supportRepo.Assign(ctx, id, assignedTo)
}
