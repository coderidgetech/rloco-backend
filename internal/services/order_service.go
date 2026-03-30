package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrderService interface {
	Create(ctx context.Context, userID primitive.ObjectID, items []models.OrderItem, shippingInfo models.ShippingInfo, paymentInfo models.PaymentInfo, paymentMethod string, promotionCode *string, giftPackingCharge float64) (*models.Order, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*models.Order, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Order, int64, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	Cancel(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID, reason string) error
	GetTrackingUpdates(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderTrackingUpdate, error)
	AddTrackingUpdate(ctx context.Context, orderID primitive.ObjectID, status, location, description string, trackingNumber *string) error
	List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.Order, int64, error)
	GetStats(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)
}

type orderService struct {
	orderRepo       repositories.OrderRepository
	trackingRepo    repositories.OrderTrackingRepository
	productRepo     repositories.ProductRepository
	promotionRepo   repositories.PromotionRepository
	emailService    EmailService
	shippingService ShippingService
	taxService      TaxService
}

func NewOrderService(orderRepo repositories.OrderRepository, trackingRepo repositories.OrderTrackingRepository, productRepo repositories.ProductRepository, promotionRepo repositories.PromotionRepository, emailService EmailService, shippingService ShippingService, taxService TaxService) OrderService {
	return &orderService{
		orderRepo:       orderRepo,
		trackingRepo:    trackingRepo,
		productRepo:     productRepo,
		promotionRepo:   promotionRepo,
		emailService:    emailService,
		shippingService: shippingService,
		taxService:      taxService,
	}
}

func (s *orderService) Create(ctx context.Context, userID primitive.ObjectID, items []models.OrderItem, shippingInfo models.ShippingInfo, paymentInfo models.PaymentInfo, paymentMethod string, promotionCode *string, giftPackingCharge float64) (*models.Order, error) {
	// Never persist card-like data in orders; keep only non-sensitive payment hints.
	safePaymentInfo := models.PaymentInfo{
		UPIID:      paymentInfo.UPIID,
		WalletName: paymentInfo.WalletName,
	}

	if _, ok := normalizeSupportedCountry(shippingInfo.Country); !ok {
		return nil, errors.New("unsupported shipping country: only India and United States are allowed")
	}
	orderMarket := orderMarketFromShippingCountry(shippingInfo.Country)

	// Reprice and validate all incoming items from authoritative product catalog.
	validatedItems := make([]models.OrderItem, 0, len(items))
	var subtotal float64
	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, errors.New("invalid quantity")
		}
		product, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			return nil, errors.New("product not found")
		}
		if !productOrderableInMarket(product, orderMarket) {
			return nil, fmt.Errorf("product %s is not available for this shipping region", product.Name)
		}
		stockQty := product.Stock[item.Size]
		if stockQty < item.Quantity {
			return nil, fmt.Errorf("insufficient stock for product %s size %s", product.Name, item.Size)
		}
		price := product.Price
		if orderMarket == "IN" && product.PriceINR != nil {
			price = *product.PriceINR
		}
		sanitized := item
		sanitized.ProductName = product.Name
		sanitized.Image = ""
		if len(product.Images) > 0 {
			sanitized.Image = product.Images[0]
		}
		sanitized.Price = price
		validatedItems = append(validatedItems, sanitized)
		subtotal += price * float64(item.Quantity)
	}

	// Apply promotion if provided
	var discount float64
	if promotionCode != nil && *promotionCode != "" {
		promotion, err := s.promotionRepo.GetByCode(ctx, *promotionCode)
		if err == nil && promotion.IsActive {
			now := time.Now()
			if now.After(promotion.StartDate) && now.Before(promotion.EndDate) {
				if promotion.MinPurchase == nil || subtotal >= *promotion.MinPurchase {
					if promotion.Type == "percentage" {
						discount = subtotal * (promotion.Value / 100)
						if promotion.MaxDiscount != nil && discount > *promotion.MaxDiscount {
							discount = *promotion.MaxDiscount
						}
					} else if promotion.Type == "fixed" {
						discount = promotion.Value
					}
					// Increment usage
					s.promotionRepo.IncrementUsage(ctx, promotion.ID)
				}
			}
		}
	}

	// Calculate shipping using shipping service
	shippingCost := 15.0 // Default
	if s.shippingService != nil {
		methods, err := s.shippingService.CalculateShipping(ctx, ShippingQuoteRequest{
			Country:    shippingInfo.Country,
			State:      shippingInfo.State,
			City:       shippingInfo.City,
			Address:    shippingInfo.Address,
			PostalCode: shippingInfo.ZipCode,
			FirstName:  shippingInfo.FirstName,
			LastName:   shippingInfo.LastName,
			Email:      shippingInfo.Email,
			Phone:      shippingInfo.Phone,
			Subtotal:   subtotal,
		})
		if err == nil && len(methods) > 0 {
			// Orders are still stored in USD internally, so normalize INR quotes.
			shippingCost = normalizeShippingCostUSD(methods[0].BaseCost, methods[0].Currency)
		}
	}

	// Apply free shipping promotions
	if subtotal > 200 {
		shippingCost = 0
	}
	if promotionCode != nil && *promotionCode == "FREESHIP" {
		shippingCost = 0
	}

	// Calculate tax using tax service
	tax := (subtotal - discount) * 0.08 // Default 8%
	if s.taxService != nil {
		calculatedTax, _, err := s.taxService.CalculateTax(ctx, shippingInfo.Country, shippingInfo.State, shippingInfo.City, shippingInfo.ZipCode, subtotal-discount)
		if err == nil {
			tax = calculatedTax
		}
	}

	// Calculate total (include gift packing charge)
	if giftPackingCharge < 0 {
		giftPackingCharge = 0
	}
	total := subtotal - discount + shippingCost + giftPackingCharge + tax

	// Generate unique order number using timestamp + random to prevent collisions
	orderNumber := fmt.Sprintf("RLC%d%06d", time.Now().Unix(), rand.Intn(999999))

	// Create order
	order := &models.Order{
		OrderNumber:        orderNumber,
		UserID:             userID,
		Items:              validatedItems,
		ShippingInfo:       shippingInfo,
		PaymentInfo:        safePaymentInfo,
		Subtotal:           subtotal,
		Discount:           discount,
		ShippingCost:       shippingCost,
		GiftPackingCharge:  giftPackingCharge,
		Tax:                tax,
		Total:              total,
		Status:             "pending",
		PaymentMethod: paymentMethod,
		PaymentStatus: "pending",
		PromotionCode: promotionCode,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Validate and atomically update stock to prevent race conditions
	for _, item := range validatedItems {
		// First verify product exists
		product, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			return nil, errors.New("product not found")
		}
		if !productOrderableInMarket(product, orderMarket) {
			return nil, fmt.Errorf("product %s is not available for this shipping region", product.Name)
		}

		// Atomically update stock - this prevents overselling
		if err := s.productRepo.AtomicStockUpdate(ctx, item.ProductID, item.Size, item.Quantity); err != nil {
			return nil, fmt.Errorf("insufficient stock for product %s size %s", product.Name, item.Size)
		}
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}

	// Update payment status based on payment method
	if paymentMethod == "cod" {
		order.PaymentStatus = "pending"
	} else {
		// For card/upi/wallet, assume payment is processed
		order.PaymentStatus = "paid"
		order.Status = "processing"
		s.orderRepo.Update(ctx, order.ID, order)
	}

	// Send order confirmation email (async)
	go func() {
		orderData := map[string]interface{}{
			"total": order.Total,
		}
		_ = s.emailService.SendOrderConfirmation(order.ShippingInfo.Email, order.OrderNumber, orderData)
		totalDisplay := fmt.Sprintf("$%.2f", order.Total)
		_ = s.emailService.SendNewOrderAlert(order.OrderNumber, totalDisplay, order.ShippingInfo.Email)
	}()

	return order, nil
}

func normalizeSupportedCountry(input string) (string, bool) {
	country := strings.TrimSpace(strings.ToLower(input))
	switch country {
	case "india", "in":
		return "India", true
	case "united states", "us", "usa":
		return "United States", true
	default:
		return "", false
	}
}

func orderMarketFromShippingCountry(country string) string {
	norm, ok := normalizeSupportedCountry(country)
	if !ok {
		return ""
	}
	switch norm {
	case "India":
		return "IN"
	case "United States":
		return "US"
	default:
		return ""
	}
}

func productOrderableInMarket(p *models.Product, market string) bool {
	if market == "" {
		return true
	}
	if len(p.AvailableMarkets) == 0 {
		return true
	}
	for _, m := range p.AvailableMarkets {
		if m == market {
			return true
		}
	}
	return false
}

func normalizeShippingCostUSD(amount float64, currency string) float64 {
	switch currency {
	case "INR", "inr":
		return amount / 75
	default:
		return amount
	}
}

func (s *orderService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	return s.orderRepo.GetByID(ctx, id)
}

func (s *orderService) GetByOrderNumber(ctx context.Context, orderNumber string) (*models.Order, error) {
	return s.orderRepo.GetByOrderNumber(ctx, orderNumber)
}

func (s *orderService) GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Order, int64, error) {
	return s.orderRepo.GetByUserID(ctx, userID, limit, skip)
}

func (s *orderService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	oldStatus := order.Status
	if err := s.orderRepo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}

	// Send email notifications for status changes
	go func() {
		if status == "shipped" && order.TrackingNumber != nil {
			_ = s.emailService.SendShippingNotification(order.ShippingInfo.Email, order.OrderNumber, *order.TrackingNumber)
		} else if status != oldStatus {
			_ = s.emailService.SendOrderStatusUpdate(order.ShippingInfo.Email, order.OrderNumber, status)
		}
	}()

	return nil
}

func (s *orderService) List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.Order, int64, error) {
	bsonFilter := bson.M{}
	if status, ok := filter["status"].(string); ok && status != "" {
		bsonFilter["status"] = status
	}
	return s.orderRepo.List(ctx, bsonFilter, limit, skip)
}

func (s *orderService) GetStats(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
	return s.orderRepo.GetStats(ctx, startDate, endDate)
}

func (s *orderService) Cancel(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID, reason string) error {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("order not found")
	}

	// Check ownership
	if order.UserID != userID {
		return errors.New("unauthorized: you can only cancel your own orders")
	}

	// Check if order can be cancelled
	if order.Status == "cancelled" {
		return errors.New("order is already cancelled")
	}
	if order.Status == "delivered" {
		return errors.New("delivered orders cannot be cancelled")
	}
	if order.Status == "shipped" {
		return errors.New("shipped orders cannot be cancelled. Please request a return instead")
	}

	// Update order status
	if err := s.orderRepo.UpdateStatus(ctx, id, "cancelled"); err != nil {
		return err
	}

	// Add tracking update
	_ = s.AddTrackingUpdate(ctx, id, "cancelled", "System", "Order cancelled by customer. Reason: "+reason, nil)

	// Send email notification
	go func() {
		_ = s.emailService.SendOrderStatusUpdate(order.ShippingInfo.Email, order.OrderNumber, "cancelled")
	}()

	return nil
}

func (s *orderService) GetTrackingUpdates(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderTrackingUpdate, error) {
	return s.trackingRepo.GetByOrderID(ctx, orderID)
}

func (s *orderService) AddTrackingUpdate(ctx context.Context, orderID primitive.ObjectID, status, location, description string, trackingNumber *string) error {
	update := &models.OrderTrackingUpdate{
		OrderID:        orderID,
		Status:         status,
		Date:           time.Now(),
		Location:       location,
		Description:    description,
		TrackingNumber: trackingNumber,
	}
	return s.trackingRepo.Create(ctx, update)
}
