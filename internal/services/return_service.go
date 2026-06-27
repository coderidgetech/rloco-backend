package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

// returnWindowDays is how long after delivery a customer may request a return.
// Measured from the order's UpdatedAt, which is set when the order transitions to
// "delivered" (no dedicated delivered-at timestamp exists on the order yet).
const returnWindowDays = 30

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

	// Returns are only valid for delivered orders, within the return window. (The UI
	// already gates on "delivered"; enforce it server-side so a direct API call can't
	// return a pending/shipped/cancelled order.)
	if order.Status != "delivered" {
		return nil, errors.New("only delivered orders can be returned")
	}
	if time.Since(order.UpdatedAt) > returnWindowDays*24*time.Hour {
		return nil, fmt.Errorf("the %d-day return window for this order has passed", returnWindowDays)
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
		_ = s.emailService.SendReturnConfirmation(order.ShippingInfo.Email, order, returnReq)
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

	// Note: stock is NOT restored here. Approval only authorizes the return; inventory
	// is credited when the refund is processed (i.e. once the item has been received),
	// so an approved-but-never-shipped-back return doesn't inflate available stock.
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
	if !isCODPaymentMethod(orderForRefund.PaymentMethod) {
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

	// Restore stock now that the return is finalized (item received & refund issued).
	for _, item := range returnReq.Items {
		_ = s.productRepo.AtomicStockIncrement(ctx, item.ProductID, item.Size, item.Quantity)
	}

	// Determine whether the order is now fully or partially refunded. A partial return
	// (e.g. 1 of 3 items) must NOT mark the whole order "refunded"/"returned". Sum the
	// refund amounts of all completed returns for this order plus this one.
	refundedTotal := returnReq.RefundAmount
	if existing, gerr := s.returnRepo.GetByOrderID(ctx, returnReq.OrderID); gerr == nil {
		for _, r := range existing {
			if r.ID != returnReq.ID && r.Status == "completed" {
				refundedTotal += r.RefundAmount
			}
		}
	}
	fullyRefunded := orderForRefund.Total > 0 && refundedTotal >= orderForRefund.Total-0.01

	if !isCODPaymentMethod(orderForRefund.PaymentMethod) {
		if fullyRefunded {
			_ = s.orderRepo.UpdatePaymentStatus(ctx, returnReq.OrderID, "refunded")
		} else {
			_ = s.orderRepo.UpdatePaymentStatus(ctx, returnReq.OrderID, "partially_refunded")
		}
	}

	// Update refund method and status
	returnReq.RefundMethod = refundMethod
	returnReq.RefundStatus = "processed"
	returnReq.Status = "completed"

	if err := s.returnRepo.Update(ctx, id, returnReq); err != nil {
		return err
	}

	// Reflect a FULL refund back onto the parent order so it reads as "returned". A
	// partial return leaves the order in its delivered/shipped state. Only the
	// post-fulfillment states can transition to "returned"; guard here because this
	// uses the raw repository (the return service has no OrderService dependency).
	if fullyRefunded && (orderForRefund.Status == "shipped" || orderForRefund.Status == "delivered") {
		_ = s.orderRepo.UpdateStatus(ctx, returnReq.OrderID, "returned")
	}

	// Send refund notification email (async)
	go func() {
		ret, _ := s.returnRepo.GetByID(context.Background(), id)
		if ret != nil {
			ord, _ := s.orderRepo.GetByID(context.Background(), ret.OrderID)
			if ord != nil {
				_ = s.emailService.SendRefundNotification(ord.ShippingInfo.Email, ord, ret)
			}
		}
	}()

	return nil
}

// allowedReturnTransitions is the legal return-status state machine for the generic
// admin status setter. The richer Approve/Reject/ProcessRefund paths carry their own
// side effects; this guards manual overrides from skipping straight to "completed"
// (which would mark a return done without a refund ever being issued).
var allowedReturnTransitions = map[string][]string{
	"requested":  {"approved", "rejected"},
	"approved":   {"processing", "completed"},
	"processing": {"completed"},
	"rejected":   {},
	"completed":  {},
}

func (s *returnService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	if _, known := allowedReturnTransitions[status]; !known {
		return errors.New("invalid status")
	}
	current, err := s.returnRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("return request not found")
	}
	if status == current.Status {
		return nil
	}
	valid := false
	for _, next := range allowedReturnTransitions[current.Status] {
		if next == status {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("illegal return status transition: %q -> %q", current.Status, status)
	}
	return s.returnRepo.UpdateStatus(ctx, id, status)
}
