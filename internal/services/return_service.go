package services

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type ReturnService interface {
	Create(ctx context.Context, orderID, userID primitive.ObjectID, items []models.ReturnItem, reason, description string) (*models.Return, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Return, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Return, int64, error)
	GetByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.Return, error)
	List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.Return, int64, error)
	Approve(ctx context.Context, id primitive.ObjectID) error
	Reject(ctx context.Context, id primitive.ObjectID, reason string) error
	ProcessRefund(ctx context.Context, id primitive.ObjectID, refundMethod string) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
}

type returnService struct {
	returnRepo     repositories.ReturnRepository
	orderRepo      repositories.OrderRepository
	productRepo    repositories.ProductRepository
	paymentRepo    repositories.PaymentRepository
	paymentService PaymentService
	emailService   EmailService
}

func NewReturnService(returnRepo repositories.ReturnRepository, orderRepo repositories.OrderRepository, productRepo repositories.ProductRepository, paymentRepo repositories.PaymentRepository, paymentService PaymentService, emailService EmailService) ReturnService {
	return &returnService{
		returnRepo:     returnRepo,
		orderRepo:      orderRepo,
		productRepo:    productRepo,
		paymentRepo:    paymentRepo,
		paymentService: paymentService,
		emailService:   emailService,
	}
}

func (s *returnService) Create(ctx context.Context, orderID, userID primitive.ObjectID, items []models.ReturnItem, reason, description string) (*models.Return, error) {
	// Get order to verify ownership and calculate refund
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, errors.New("order not found")
	}

	// Verify order belongs to user
	if order.UserID.Hex() != userID.Hex() {
		return nil, errors.New("order does not belong to user")
	}

	// Check if order is eligible for return (not cancelled, delivered, etc.)
	if order.Status == "cancelled" {
		return nil, errors.New("cancelled orders cannot be returned")
	}

	// Calculate refund amount; validate return quantities do not exceed ordered quantities
	type lineKey struct {
		pid  primitive.ObjectID
		size string
	}
	orderedQty := make(map[lineKey]int)
	for _, oi := range order.Items {
		orderedQty[lineKey{oi.ProductID, oi.Size}] += oi.Quantity
	}
	returnedSoFar := make(map[lineKey]int)

	var refundAmount float64
	for _, returnItem := range items {
		if returnItem.Quantity <= 0 {
			return nil, errors.New("invalid return quantity")
		}
		k := lineKey{returnItem.ProductID, returnItem.Size}
		maxQ := orderedQty[k]
		if maxQ == 0 {
			return nil, errors.New("return item does not match order line")
		}
		returnedSoFar[k] += returnItem.Quantity
		if returnedSoFar[k] > maxQ {
			return nil, fmt.Errorf("return quantity exceeds ordered quantity for product line")
		}
		for _, orderItem := range order.Items {
			if orderItem.ProductID == returnItem.ProductID && orderItem.Size == returnItem.Size {
				refundAmount += orderItem.Price * float64(returnItem.Quantity)
				break
			}
		}
	}

	// Apply proportional discount and tax
	if order.Subtotal <= 0 {
		return nil, errors.New("order subtotal invalid for refund calculation")
	}
	proportionalDiscount := (refundAmount / order.Subtotal) * order.Discount
	proportionalTax := (refundAmount / order.Subtotal) * order.Tax
	refundAmount = refundAmount - proportionalDiscount + proportionalTax

	returnReq := &models.Return{
		OrderID:      orderID,
		OrderNumber:  order.OrderNumber,
		UserID:       userID,
		Items:        items,
		Reason:       reason,
		Description:  description,
		RefundAmount: refundAmount,
		RefundMethod: "original", // Default to original payment method
		Status:       "requested",
		RefundStatus: "pending",
	}

	if err := s.returnRepo.Create(ctx, returnReq); err != nil {
		return nil, err
	}

	// Send return confirmation email (async)
	go func() {
		_ = s.emailService.SendReturnConfirmation(order.ShippingInfo.Email, returnReq.ID.Hex())
	}()

	return returnReq, nil
}

func (s *returnService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Return, error) {
	return s.returnRepo.GetByID(ctx, id)
}

func (s *returnService) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Return, int64, error) {
	return s.returnRepo.GetByUserID(ctx, userID, limit, skip)
}

func (s *returnService) GetByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.Return, error) {
	return s.returnRepo.GetByOrderID(ctx, orderID)
}

func (s *returnService) List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.Return, int64, error) {
	bsonFilter := bson.M{}
	if status, ok := filter["status"].(string); ok && status != "" {
		bsonFilter["status"] = status
	}
	if refundStatus, ok := filter["refund_status"].(string); ok && refundStatus != "" {
		bsonFilter["refund_status"] = refundStatus
	}
	return s.returnRepo.List(ctx, bsonFilter, limit, skip)
}

func (s *returnService) Approve(ctx context.Context, id primitive.ObjectID) error {
	returnReq, err := s.returnRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("return request not found")
	}

	if returnReq.Status != "requested" {
		return errors.New("return request is not in requested status")
	}

	// Restore stock
	for _, item := range returnReq.Items {
		product, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err == nil {
			if product.Stock == nil {
				product.Stock = make(map[string]int)
			}
			product.Stock[item.Size] += item.Quantity
			s.productRepo.Update(ctx, product.ID, product)
		}
	}

	// Update status
	return s.returnRepo.UpdateStatus(ctx, id, "approved")
}

func (s *returnService) Reject(ctx context.Context, id primitive.ObjectID, reason string) error {
	returnReq, err := s.returnRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("return request not found")
	}

	if returnReq.Status != "requested" {
		return errors.New("return request is not in requested status")
	}

	// Update description with rejection reason
	returnReq.Description = fmt.Sprintf("%s\n\nRejection Reason: %s", returnReq.Description, reason)
	returnReq.Status = "rejected"

	return s.returnRepo.Update(ctx, id, returnReq)
}

func (s *returnService) ProcessRefund(ctx context.Context, id primitive.ObjectID, refundMethod string) error {
	returnReq, err := s.returnRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("return request not found")
	}

	if returnReq.Status != "approved" {
		return errors.New("return request must be approved before processing refund")
	}

	orderForRefund, oerr := s.orderRepo.GetByID(ctx, returnReq.OrderID)
	if oerr != nil {
		return errors.New("order not found for refund")
	}

	// COD / manual refunds: no card transaction to reverse.
	if orderForRefund.PaymentMethod != "cod" {
		transaction, txErr := s.paymentRepo.GetByOrderID(ctx, returnReq.OrderID)
		if txErr != nil || transaction == nil {
			return errors.New("no payment transaction found for gateway refund")
		}
		if transaction.Status != "success" {
			return errors.New("payment was not successful; cannot process refund")
		}
		amount := returnReq.RefundAmount
		if err := s.paymentService.RefundPayment(ctx, transaction.ID, &amount); err != nil {
			return fmt.Errorf("gateway refund failed: %w", err)
		}
	}

	// Update refund method and status
	returnReq.RefundMethod = refundMethod
	returnReq.RefundStatus = "processed"
	returnReq.Status = "completed"

	if err := s.returnRepo.Update(ctx, id, returnReq); err != nil {
		return err
	}

	// Send refund notification email (async)
	go func() {
		ret, _ := s.returnRepo.GetByID(context.Background(), id)
		if ret != nil {
			ord, _ := s.orderRepo.GetByID(context.Background(), ret.OrderID)
			if ord != nil {
				_ = s.emailService.SendRefundNotification(ord.ShippingInfo.Email, ret.ID.Hex(), ret.RefundAmount)
			}
		}
	}()

	return nil
}

func (s *returnService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	validStatuses := map[string]bool{
		"requested":  true,
		"approved":   true,
		"rejected":   true,
		"processing": true,
		"completed": true,
	}
	if !validStatuses[status] {
		return errors.New("invalid status")
	}
	return s.returnRepo.UpdateStatus(ctx, id, status)
}
