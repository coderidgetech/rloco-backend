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
	CreateGuestOrder(ctx context.Context, guestEmail, guestName string, items []models.OrderItem, shippingInfo models.ShippingInfo, paymentMethod string, promotionCode *string) (*models.Order, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*models.Order, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Order, int64, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	// FulfillOrder purchases a shipping label via the appropriate carrier, saves the
	// tracking number on the order, then transitions it to "shipped".
	FulfillOrder(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	Cancel(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID, reason string) error
	GetTrackingUpdates(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderTrackingUpdate, error)
	AddTrackingUpdate(ctx context.Context, orderID primitive.ObjectID, status, location, description string, trackingNumber *string) error
	List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.Order, int64, error)
	GetStats(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)
}

// OrderCheckoutPricing holds env-driven defaults for order totals (see config.Config ORDER_* vars).
type OrderCheckoutPricing struct {
	DefaultShippingUSD      float64
	DefaultTaxRate          float64
	FreeShippingSubtotalUSD float64
	INRPerUSD               float64
}

type orderService struct {
	orderRepo        repositories.OrderRepository
	trackingRepo     repositories.OrderTrackingRepository
	productRepo      repositories.ProductRepository
	promotionRepo    repositories.PromotionRepository
	promotionService PromotionService
	checkoutPricing  OrderCheckoutPricing
	emailService     EmailService
	shippingService  ShippingService
	taxService       TaxService
	stripeUS         StripeUSTax // optional; US sales tax via Stripe Tax when set
	smsService       *TwilioSMSService
	fcmService       *FCMService
	userRepo         repositories.UserRepository
	rewardsRepo      repositories.RewardsRepository
}

func NewOrderService(orderRepo repositories.OrderRepository, trackingRepo repositories.OrderTrackingRepository, productRepo repositories.ProductRepository, promotionRepo repositories.PromotionRepository, promotionService PromotionService, checkoutPricing OrderCheckoutPricing, emailService EmailService, shippingService ShippingService, taxService TaxService, stripeUS StripeUSTax, smsService *TwilioSMSService, fcmService *FCMService, userRepo repositories.UserRepository, rewardsRepo repositories.RewardsRepository) OrderService {
	return &orderService{
		orderRepo:        orderRepo,
		trackingRepo:     trackingRepo,
		productRepo:      productRepo,
		promotionRepo:    promotionRepo,
		promotionService: promotionService,
		checkoutPricing:  checkoutPricing,
		emailService:     emailService,
		shippingService:  shippingService,
		taxService:       taxService,
		stripeUS:         stripeUS,
		smsService:       smsService,
		fcmService:       fcmService,
		userRepo:         userRepo,
		rewardsRepo:      rewardsRepo,
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

	var orderWeightLb float64
	for _, it := range validatedItems {
		orderWeightLb += DefaultItemWeightLb * float64(it.Quantity)
	}
	if orderWeightLb <= 0 {
		orderWeightLb = DefaultItemWeightLb
	}
	weightPtr := orderWeightLb

	// Apply promotion if provided — same rules as POST /promotions/validate (usage limits, dates, min purchase)
	var discount float64
	var appliedPromotionID *primitive.ObjectID
	var promoFreeShipping bool
	if promotionCode != nil && strings.TrimSpace(*promotionCode) != "" {
		code := strings.TrimSpace(*promotionCode)
		promotion, d, err := s.promotionService.Validate(ctx, code, subtotal)
		if err != nil {
			return nil, err
		}
		discount = d
		if promotion.Type == "free_shipping" {
			promoFreeShipping = true
		}
		pid := promotion.ID
		appliedPromotionID = &pid
	}

	// Calculate shipping using shipping service
	shippingCost := s.checkoutPricing.DefaultShippingUSD
	if shippingCost < 0 {
		shippingCost = 0
	}
	inrPer := s.checkoutPricing.INRPerUSD
	if inrPer <= 0 {
		inrPer = 75
	}
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
			Weight:     &weightPtr,
		})
		if err == nil && len(methods) > 0 {
			// Orders are still stored in USD internally, so normalize INR quotes.
			shippingCost = normalizeShippingCostUSD(methods[0].BaseCost, methods[0].Currency, inrPer)
		}
	}

	// Apply free shipping (subtotal threshold + free_shipping promotion type from Validate)
	freeShipThreshold := s.checkoutPricing.FreeShippingSubtotalUSD
	if freeShipThreshold < 0 {
		freeShipThreshold = 0
	}
	if subtotal > freeShipThreshold {
		shippingCost = 0
	}
	if promoFreeShipping {
		shippingCost = 0
	}

	// Tax: Stripe Tax for US; Mongo-configured GST (or ORDER_INDIA_DEFAULT_GST_PERCENT) for India.
	taxable := subtotal - discount
	if taxable < 0 {
		taxable = 0
	}
	taxRate := s.checkoutPricing.DefaultTaxRate
	if taxRate < 0 {
		taxRate = 0
	}
	tax := taxable * taxRate
	if orderMarket == "US" {
		if s.stripeUS != nil {
			if t, err := s.stripeUS.Calculate(ctx, taxable, shippingCost, shippingInfo); err == nil {
				tax = t
			} else if s.taxService != nil {
				if calculatedTax, _, err := s.taxService.CalculateTax(ctx, shippingInfo.Country, shippingInfo.State, shippingInfo.City, shippingInfo.ZipCode, taxable); err == nil {
					tax = calculatedTax
				}
			}
		} else if s.taxService != nil {
			if calculatedTax, _, err := s.taxService.CalculateTax(ctx, shippingInfo.Country, shippingInfo.State, shippingInfo.City, shippingInfo.ZipCode, taxable); err == nil {
				tax = calculatedTax
			}
		}
	} else if s.taxService != nil {
		if calculatedTax, _, err := s.taxService.CalculateTax(ctx, shippingInfo.Country, shippingInfo.State, shippingInfo.City, shippingInfo.ZipCode, taxable); err == nil {
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

	if appliedPromotionID != nil {
		_ = s.promotionRepo.IncrementUsage(ctx, *appliedPromotionID)
	}

	// Non-COD: order stays status=pending, payment_status=pending until Stripe webhook or /payments/process.
	// COD: same defaults; ops may mark paid on delivery via admin flows.

	// Send order confirmation notifications (async)
	go func() {
		orderData := map[string]interface{}{
			"total": order.Total,
		}
		_ = s.emailService.SendOrderConfirmation(order.ShippingInfo.Email, order.OrderNumber, orderData)
		totalDisplay := fmt.Sprintf("$%.2f", order.Total)
		_ = s.emailService.SendNewOrderAlert(order.OrderNumber, totalDisplay, order.ShippingInfo.Email)
		if s.smsService != nil && s.smsService.Enabled() && order.ShippingInfo.Phone != "" {
			_ = s.smsService.SendOrderConfirmation(context.Background(), order.ShippingInfo.Phone, order.OrderNumber)
		}
		if s.fcmService != nil && s.fcmService.Enabled() && s.userRepo != nil {
			if u, err := s.userRepo.GetByID(context.Background(), order.UserID); err == nil && len(u.FCMTokens) > 0 {
				_ = s.fcmService.SendToTokens(context.Background(), u.FCMTokens, "Order Confirmed", "Your order "+order.OrderNumber+" has been confirmed!", map[string]string{"order_id": order.ID.Hex()})
			}
		}
	}()

	// Credit earned reward points (1 pt per $1 of order total, async)
	go func() {
		if s.rewardsRepo != nil {
			pts := int64(order.Total)
			if pts > 0 {
				tx := &models.RewardsTransaction{
					UserID:      order.UserID,
					Type:        "earned",
					Points:      pts,
					Reference:   order.ID.Hex(),
					Description: "Points earned for order " + order.OrderNumber,
					CreatedAt:   time.Now(),
				}
				_ = s.rewardsRepo.AddTransaction(context.Background(), tx)
			}
		}
	}()

	return order, nil
}

func (s *orderService) CreateGuestOrder(ctx context.Context, guestEmail, guestName string, items []models.OrderItem, shippingInfo models.ShippingInfo, paymentMethod string, promotionCode *string) (*models.Order, error) {
	if strings.TrimSpace(guestEmail) == "" {
		return nil, errors.New("guest email is required")
	}
	if strings.TrimSpace(guestName) == "" {
		return nil, errors.New("guest name is required")
	}
	if strings.ToLower(paymentMethod) != "cod" && strings.ToLower(paymentMethod) != "cash_on_delivery" {
		return nil, errors.New("guest orders only support Cash on Delivery")
	}
	if len(items) == 0 {
		return nil, errors.New("order must contain at least one item")
	}
	// Fill in guest name/email into shipping info if not already provided
	if shippingInfo.Email == "" {
		shippingInfo.Email = guestEmail
	}
	if shippingInfo.FirstName == "" && shippingInfo.LastName == "" {
		parts := strings.SplitN(strings.TrimSpace(guestName), " ", 2)
		shippingInfo.FirstName = parts[0]
		if len(parts) > 1 {
			shippingInfo.LastName = parts[1]
		}
	}
	order, err := s.Create(ctx, primitive.ObjectID{}, items, shippingInfo, models.PaymentInfo{}, paymentMethod, promotionCode, 0)
	if err != nil {
		return nil, err
	}
	// Annotate with guest identity for admin visibility
	order.GuestEmail = &guestEmail
	order.GuestName = &guestName
	_ = s.orderRepo.Update(ctx, order.ID, order)
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

// normalizeShippingCostUSD converts quoted shipping to USD when the carrier returns INR.
func normalizeShippingCostUSD(amount float64, currency string, inrPerUSD float64) float64 {
	if inrPerUSD <= 0 {
		inrPerUSD = 75
	}
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "INR":
		return amount / inrPerUSD
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

	// Send notifications for status changes (async)
	go func() {
		if status == "shipped" && order.TrackingNumber != nil {
			_ = s.emailService.SendShippingNotification(order.ShippingInfo.Email, order.OrderNumber, *order.TrackingNumber)
			if s.smsService != nil && s.smsService.Enabled() && order.ShippingInfo.Phone != "" {
				_ = s.smsService.SendShippingNotification(context.Background(), order.ShippingInfo.Phone, order.OrderNumber, *order.TrackingNumber)
			}
		} else if status != oldStatus {
			_ = s.emailService.SendOrderStatusUpdate(order.ShippingInfo.Email, order.OrderNumber, status)
			if status == "delivered" && s.smsService != nil && s.smsService.Enabled() && order.ShippingInfo.Phone != "" {
				_ = s.smsService.SendOrderDelivered(context.Background(), order.ShippingInfo.Phone, order.OrderNumber)
			}
		}
		if s.fcmService != nil && s.fcmService.Enabled() && s.userRepo != nil && status != oldStatus {
			titleMap := map[string]string{
				"shipped":   "Your Order Has Shipped",
				"delivered": "Your Order Was Delivered",
				"cancelled": "Order Cancelled",
			}
			if title, ok := titleMap[status]; ok {
				if u, err := s.userRepo.GetByID(context.Background(), order.UserID); err == nil && len(u.FCMTokens) > 0 {
					_ = s.fcmService.SendToTokens(context.Background(), u.FCMTokens, title, "Order "+order.OrderNumber, map[string]string{"order_id": order.ID.Hex(), "status": status})
				}
			}
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

func (s *orderService) FulfillOrder(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order.Status == "shipped" || order.Status == "delivered" || order.Status == "cancelled" {
		return nil, fmt.Errorf("order cannot be fulfilled in status %q", order.Status)
	}

	trackingNumber, labelURL, err := s.shippingService.FulfillShipment(ctx, order, 0)
	if err != nil {
		return nil, fmt.Errorf("carrier label purchase failed: %w", err)
	}

	if err := s.orderRepo.SetTrackingNumber(ctx, id, trackingNumber, labelURL); err != nil {
		return nil, err
	}

	// UpdateStatus reads the fresh tracking number and fires email/SMS/FCM notifications
	if err := s.UpdateStatus(ctx, id, "shipped"); err != nil {
		return nil, err
	}

	return s.orderRepo.GetByID(ctx, id)
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

	// Restore inventory when cancelling before fulfillment (stock was reserved at order create)
	if order.Status == "pending" || order.Status == "processing" {
		for _, item := range order.Items {
			_ = s.productRepo.AtomicStockIncrement(ctx, item.ProductID, item.Size, item.Quantity)
		}
	}

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
