package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrderService interface {
	Create(ctx context.Context, userID primitive.ObjectID, items []models.OrderItem, shippingInfo models.ShippingInfo, paymentInfo models.PaymentInfo, paymentMethod string, promotionCode *string, giftPackingCharge float64, selectedCarrier, selectedService string) (*models.Order, error)
	CreateGuestOrder(ctx context.Context, guestEmail, guestName string, items []models.OrderItem, shippingInfo models.ShippingInfo, paymentMethod string, promotionCode *string, selectedCarrier, selectedService string) (*models.Order, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*models.Order, error)
	GetByUserID(ctx context.Context, userID primitive.ObjectID, limit, skip int) ([]*models.Order, int64, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	// FulfillOrder purchases a shipping label via the appropriate carrier, saves the
	// tracking number on the order, then transitions it to "shipped".
	FulfillOrder(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	Cancel(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID, reason string) error
	// SweepAbandonedOrders cancels pending, unpaid, non-COD orders older than
	// olderThan and releases their reserved stock. Returns the number released.
	SweepAbandonedOrders(ctx context.Context, olderThan time.Duration) (int, error)
	GetTrackingUpdates(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderTrackingUpdate, error)
	AddTrackingUpdate(ctx context.Context, orderID primitive.ObjectID, status, location, description string, trackingNumber *string) error
	List(ctx context.Context, filter map[string]interface{}, limit, skip int) ([]*models.Order, int64, error)
	GetStats(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)
	// AttachRefunder wires the payment collaborators used to refund paid orders on
	// cancellation. Optional: when unset, cancelling a paid order skips the gateway refund.
	AttachRefunder(paymentRepo repositories.PaymentRepository, paymentService PaymentService)
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
	// Optional refund collaborators, attached after construction (see AttachRefunder)
	// to avoid a construction-order cycle with PaymentService.
	paymentRepo    repositories.PaymentRepository
	paymentService PaymentService
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

func (s *orderService) Create(ctx context.Context, userID primitive.ObjectID, items []models.OrderItem, shippingInfo models.ShippingInfo, paymentInfo models.PaymentInfo, paymentMethod string, promotionCode *string, giftPackingCharge float64, selectedCarrier, selectedService string) (*models.Order, error) {
	// Normalize payment_method at the single write point so all downstream exact-match
	// comparisons (COD checks, sweeper filter) are reliable.
	paymentMethod = strings.ToLower(strings.TrimSpace(paymentMethod))
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
	// persistedCarrier/Service record which rate the order was priced/charged for, so
	// fulfillment re-quotes and buys that same rate instead of the cheapest.
	var persistedCarrier, persistedService string
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
			// Honor the customer's chosen rate when supplied; otherwise default to the
			// cheapest (methods are sorted ascending by cost).
			chosen := methods[0]
			if strings.TrimSpace(selectedCarrier) != "" || strings.TrimSpace(selectedService) != "" {
				for _, m := range methods {
					if strings.EqualFold(strings.TrimSpace(m.Carrier), strings.TrimSpace(selectedCarrier)) &&
						strings.EqualFold(strings.TrimSpace(m.Name), strings.TrimSpace(selectedService)) {
						chosen = m
						break
					}
				}
			}
			// Orders are still stored in USD internally, so normalize INR quotes.
			shippingCost = normalizeShippingCostUSD(chosen.BaseCost, chosen.Currency, inrPer)
			persistedCarrier = chosen.Carrier
			persistedService = chosen.Name
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
		ShippingCarrier:  persistedCarrier,
		ShippingService:  persistedService,
		ShippingWeightLb: orderWeightLb,
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
		_ = s.emailService.SendOrderConfirmation(order.ShippingInfo.Email, order)
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

func (s *orderService) CreateGuestOrder(ctx context.Context, guestEmail, guestName string, items []models.OrderItem, shippingInfo models.ShippingInfo, paymentMethod string, promotionCode *string, selectedCarrier, selectedService string) (*models.Order, error) {
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
	order, err := s.Create(ctx, primitive.ObjectID{}, items, shippingInfo, models.PaymentInfo{}, paymentMethod, promotionCode, 0, selectedCarrier, selectedService)
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

// isCODPaymentMethod reports whether a payment method is cash-on-delivery (which has
// no gateway transaction to refund). Covers both stored spellings.
func isCODPaymentMethod(method string) bool {
	m := strings.ToLower(strings.TrimSpace(method))
	return m == "cod" || m == "cash_on_delivery"
}

// fulfillmentBlockReason returns a non-empty reason when an order in the given
// state must NOT be fulfilled (label purchased / shipped). Empty string = OK.
// COD is exempt from the paid check since it settles on delivery.
func fulfillmentBlockReason(status, paymentMethod, paymentStatus string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "shipped", "delivered", "cancelled", "returned":
		return fmt.Sprintf("order cannot be fulfilled in status %q", status)
	}
	if !isCODPaymentMethod(paymentMethod) && strings.ToLower(strings.TrimSpace(paymentStatus)) != "paid" {
		return fmt.Sprintf("order cannot be fulfilled until payment is completed (payment status %q)", paymentStatus)
	}
	return ""
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

// allowedOrderTransitions defines the legal order-status state machine. A status
// may always transition to itself (idempotent webhooks/retries).
var allowedOrderTransitions = map[string][]string{
	"pending":    {"processing", "shipped", "cancelled"},
	"processing": {"shipped", "delivered", "cancelled"},
	"shipped":    {"delivered", "returned"},
	"delivered":  {"returned"},
	"cancelled":  {},
	"returned":   {},
}

// isValidOrderStatusTransition reports whether moving from->to is permitted.
func isValidOrderStatusTransition(from, to string) bool {
	if from == to {
		return true
	}
	allowed, ok := allowedOrderTransitions[from]
	if !ok {
		// Unknown current status: only allow moving to a recognised status.
		_, known := allowedOrderTransitions[to]
		return known
	}
	for _, candidate := range allowed {
		if candidate == to {
			return true
		}
	}
	return false
}

func (s *orderService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	status = strings.ToLower(strings.TrimSpace(status))
	if _, known := allowedOrderTransitions[status]; !known {
		return fmt.Errorf("invalid order status %q", status)
	}
	if !isValidOrderStatusTransition(order.Status, status) {
		return fmt.Errorf("illegal order status transition: %q -> %q", order.Status, status)
	}

	oldStatus := order.Status
	if status == oldStatus {
		return nil
	}
	if err := s.orderRepo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}

	// Send notifications for status changes (async)
	go func() {
		if status == "shipped" && order.TrackingNumber != nil {
			_ = s.emailService.SendShippingNotification(order.ShippingInfo.Email, order)
			if s.smsService != nil && s.smsService.Enabled() && order.ShippingInfo.Phone != "" {
				_ = s.smsService.SendShippingNotification(context.Background(), order.ShippingInfo.Phone, order.OrderNumber, *order.TrackingNumber)
			}
		} else if status != oldStatus {
			_ = s.emailService.SendOrderStatusUpdate(order.ShippingInfo.Email, order, status)
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
	if reason := fulfillmentBlockReason(order.Status, order.PaymentMethod, order.PaymentStatus); reason != "" {
		return nil, errors.New(reason)
	}

	// Re-fulfill must be idempotent and concurrency-safe — buying a shipping label
	// charges money, so two clicks / a retry must never buy two labels.
	existingTracking := ""
	if order.TrackingNumber != nil {
		existingTracking = strings.TrimSpace(*order.TrackingNumber)
	}
	switch {
	case existingTracking == repositories.FulfillmentClaimSentinel:
		// A concurrent fulfill is mid-flight, or a prior attempt bought a label but
		// failed to persist the real number (that path logs for reconciliation).
		// Don't ship with the sentinel and don't buy a second label.
		return nil, errors.New("order fulfillment is already in progress; retry shortly")
	case existingTracking != "":
		// A real label already exists from a prior partial attempt — idempotent:
		// fall through and just (re)advance status.
	default:
		// No label yet: atomically claim the exclusive right to buy exactly one.
		won, cerr := s.orderRepo.ClaimForFulfillment(ctx, id)
		if cerr != nil {
			return nil, cerr
		}
		if !won {
			return nil, errors.New("order fulfillment is already in progress; retry shortly")
		}
		// Fulfill at the order's real captured weight (falls back to carrier default if 0).
		trackingNumber, labelURL, ferr := s.shippingService.FulfillShipment(ctx, order, order.ShippingWeightLb)
		if ferr != nil {
			_ = s.orderRepo.ReleaseFulfillmentClaim(ctx, id) // clear claim so it can be retried
			return nil, fmt.Errorf("carrier label purchase failed: %w", ferr)
		}
		if serr := s.orderRepo.SetTrackingNumber(ctx, id, trackingNumber, labelURL); serr != nil {
			// Label is bought (money spent) but the tracking number didn't persist.
			// Keep the sentinel so we never double-buy, and log loudly for manual
			// reconciliation (the real tracking/label would otherwise be lost).
			log.Printf("[FULFILL][CRITICAL] order %s: label purchased (tracking=%q label=%q) but persisting failed: %v — needs manual reconciliation", id.Hex(), trackingNumber, labelURL, serr)
			return nil, fmt.Errorf("label purchased but could not be saved; flagged for reconciliation: %w", serr)
		}
	}

	// UpdateStatus reads the fresh tracking number and fires email/SMS/FCM notifications
	if err := s.UpdateStatus(ctx, id, "shipped"); err != nil {
		return nil, err
	}

	return s.orderRepo.GetByID(ctx, id)
}

func (s *orderService) AttachRefunder(paymentRepo repositories.PaymentRepository, paymentService PaymentService) {
	s.paymentRepo = paymentRepo
	s.paymentService = paymentService
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

	// Refund a paid (non-COD) order through the gateway so cancelled-after-payment
	// doesn't leave the customer charged. An atomic paid->refunding claim ensures
	// only one concurrent cancel can trigger the refund (no double-refund).
	if order.PaymentStatus == "paid" && !isCODPaymentMethod(order.PaymentMethod) && s.paymentService != nil && s.paymentRepo != nil {
		claimed, claimErr := s.orderRepo.CompareAndSwapPaymentStatus(ctx, id, "paid", "refunding")
		if claimErr != nil {
			return fmt.Errorf("cancellation refund claim failed: %w", claimErr)
		}
		if claimed {
			tx, txErr := s.paymentRepo.GetByOrderID(ctx, id)
			if txErr == nil && tx != nil && tx.Status == "success" {
				if refundErr := s.paymentService.RefundPayment(ctx, tx.ID, nil); refundErr != nil {
					// Release the claim so the cancel can be retried.
					_ = s.orderRepo.UpdatePaymentStatus(ctx, id, "paid")
					return fmt.Errorf("cancellation refund failed: %w", refundErr)
				}
				_ = s.orderRepo.UpdatePaymentStatus(ctx, id, "refunded")
			} else {
				// No gateway transaction to reverse; leave it marked refunded.
				_ = s.orderRepo.UpdatePaymentStatus(ctx, id, "refunded")
			}
		}
	}

	if err := s.orderRepo.UpdateStatus(ctx, id, "cancelled"); err != nil {
		return err
	}

	// Reverse reward points earned at order creation (1 pt per $1), only after the
	// order is confirmed cancelled. Recorded as a "redeemed" transaction so the
	// balance (earned - redeemed) nets back out. Reference matches the earn record.
	if s.rewardsRepo != nil {
		if pts := int64(order.Total); pts > 0 {
			_ = s.rewardsRepo.AddTransaction(ctx, &models.RewardsTransaction{
				UserID:      order.UserID,
				Type:        "redeemed",
				Points:      pts,
				Reference:   order.ID.Hex(),
				Description: "Points reversed for cancelled order " + order.OrderNumber,
				CreatedAt:   time.Now(),
			})
		}
	}

	// Add tracking update
	_ = s.AddTrackingUpdate(ctx, id, "cancelled", "System", "Order cancelled by customer. Reason: "+reason, nil)

	// Send email notification
	go func() {
		_ = s.emailService.SendOrderStatusUpdate(order.ShippingInfo.Email, order, "cancelled")
	}()

	return nil
}

func (s *orderService) SweepAbandonedOrders(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	candidates, err := s.orderRepo.FindAbandonedPending(ctx, cutoff, 200)
	if err != nil {
		return 0, err
	}
	released := 0
	for _, o := range candidates {
		// Authoritative COD guard: the Mongo $nin filter is exact-match, but
		// payment_method may be stored with non-canonical casing/spelling. COD orders
		// are legitimately pending+unpaid (settle on delivery) and must NEVER be swept.
		if isCODPaymentMethod(o.PaymentMethod) {
			continue
		}
		// Atomic claim: only the caller that wins the pending->cancelled transition
		// releases stock, so concurrent sweeps (multiple replicas) can't double-release.
		won, cerr := s.orderRepo.CompareAndSwapStatus(ctx, o.ID, "pending", "cancelled")
		if cerr != nil || !won {
			continue
		}
		for _, item := range o.Items {
			_ = s.productRepo.AtomicStockIncrement(ctx, item.ProductID, item.Size, item.Quantity)
		}
		_ = s.AddTrackingUpdate(ctx, o.ID, "cancelled", "System", "Order automatically cancelled: payment not completed within the allowed window.", nil)
		released++
		log.Printf("[SWEEP] cancelled abandoned unpaid order %s (created %s); released %d item line(s) of stock",
			o.OrderNumber, o.CreatedAt.UTC().Format(time.RFC3339), len(o.Items))
	}
	return released, nil
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
