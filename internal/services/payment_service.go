package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
	"github.com/stripe/stripe-go/v76/refund"
	"github.com/stripe/stripe-go/v76/webhook"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"rloco-backend/internal/models"
	"rloco-backend/internal/repositories"
)

type PaymentService interface {
	CreatePaymentIntent(ctx context.Context, requesterID primitive.ObjectID, requesterRole string, orderID primitive.ObjectID, amount float64, currency, gateway, paymentMethod string) (*PaymentIntent, error)
	ProcessPayment(ctx context.Context, requesterID primitive.ObjectID, requesterRole, paymentIntentID, paymentMethodID, gateway string) error
	HandleWebhook(ctx context.Context, gateway string, payload []byte, signature string) error
	RefundPayment(ctx context.Context, transactionID primitive.ObjectID, amount *float64) error
	GetTransaction(ctx context.Context, requesterID primitive.ObjectID, requesterRole string, id primitive.ObjectID) (*models.PaymentTransaction, error)
	// SetPaidNotifier registers a callback fired once an order's online payment is
	// confirmed (synchronous /payments/process or Stripe webhook). Used by OrderService
	// to send the "Order Confirmed" notifications at the right time, not at checkout.
	SetPaidNotifier(fn func(ctx context.Context, orderID primitive.ObjectID))
}

type PaymentIntent struct {
	ID              string                 `json:"id"`
	ClientSecret    string                 `json:"client_secret,omitempty"`
	PaymentURL      string                 `json:"payment_url,omitempty"`
	Gateway         string                 `json:"gateway"`
	Amount          float64                `json:"amount"`
	Currency        string                 `json:"currency"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type paymentService struct {
	paymentRepo         repositories.PaymentRepository
	orderRepo           repositories.OrderRepository
	emailService        EmailService
	stripeWebhookEvents repositories.StripeWebhookEventRepository // optional
	stripeKey           string
	stripeWebhookSecret string
	paidNotifier        func(ctx context.Context, orderID primitive.ObjectID) // optional; set via SetPaidNotifier
}

func (s *paymentService) SetPaidNotifier(fn func(ctx context.Context, orderID primitive.ObjectID)) {
	s.paidNotifier = fn
}

func NewPaymentService(
	paymentRepo repositories.PaymentRepository,
	orderRepo repositories.OrderRepository,
	emailService EmailService,
	stripeWebhookEvents repositories.StripeWebhookEventRepository,
	stripeKey, stripeWebhookSecret string,
) PaymentService {
	return &paymentService{
		paymentRepo:         paymentRepo,
		orderRepo:           orderRepo,
		emailService:        emailService,
		stripeWebhookEvents: stripeWebhookEvents,
		stripeKey:           stripeKey,
		stripeWebhookSecret: stripeWebhookSecret,
	}
}

func (s *paymentService) CreatePaymentIntent(ctx context.Context, requesterID primitive.ObjectID, requesterRole string, orderID primitive.ObjectID, amount float64, currency, gateway, paymentMethod string) (*PaymentIntent, error) {
	// Verify order exists
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, errors.New("order not found")
	}
	if requesterRole != "admin" && order.UserID != requesterID {
		return nil, errors.New("access denied")
	}
	if isCODPaymentMethod(order.PaymentMethod) {
		return nil, errors.New("payment intent not applicable for cash on delivery orders")
	}
	if order.PaymentStatus == "paid" {
		return nil, errors.New("order is already paid")
	}
	// Authoritative amount from server order (ignore client-supplied amount to prevent tampering)
	payAmount := order.Total
	if payAmount <= 0 {
		return nil, errors.New("invalid order total for payment")
	}
	expectedCurrency, hasExpected := expectedCurrencyByCountry(order.ShippingInfo.Country)
	currency = strings.ToLower(strings.TrimSpace(currency))
	if hasExpected {
		if currency != "" && currency != expectedCurrency {
			return nil, fmt.Errorf("currency mismatch for shipping country: expected %s", expectedCurrency)
		}
		currency = expectedCurrency
	} else if currency == "" {
		currency = "usd"
	}
	// Optional mismatch warning: tolerate tiny float noise only
	if amount > 0 && math.Abs(amount-payAmount) > 0.02 {
		return nil, errors.New("order total has changed; refresh checkout and try again")
	}

	// Reuse an in-flight Stripe PaymentIntent for this order (double-submit / refresh safe).
	if gateway == "stripe" && s.stripeKey != "" {
		if existing, err := s.paymentRepo.GetByOrderID(ctx, orderID); err == nil && existing != nil &&
			existing.Status == "pending" && strings.HasPrefix(existing.GatewayTransactionID, "pi_") {
			stripe.Key = s.stripeKey
			pi, err := paymentintent.Get(existing.GatewayTransactionID, nil)
			if err == nil {
				switch pi.Status {
				case stripe.PaymentIntentStatusRequiresPaymentMethod,
					stripe.PaymentIntentStatusRequiresConfirmation,
					stripe.PaymentIntentStatusRequiresAction,
					stripe.PaymentIntentStatusProcessing:
					return &PaymentIntent{
						ID:           pi.ID,
						ClientSecret: pi.ClientSecret,
						Gateway:      "stripe",
						Amount:       payAmount,
						Currency:     currency,
						Metadata: map[string]interface{}{
							"order_id":         orderID.Hex(),
							"transaction_id": existing.ID.Hex(),
						},
					}, nil
				}
			}
		}
	}

	// Create payment transaction record
	transaction := &models.PaymentTransaction{
		OrderID:       orderID,
		UserID:        order.UserID,
		Amount:        payAmount,
		Currency:      currency,
		PaymentMethod: paymentMethod,
		Gateway:       gateway,
		Status:        "pending",
		Metadata:      make(map[string]interface{}),
	}
	if transaction.PaymentMethod == "" {
		transaction.PaymentMethod = "card"
	}

	if err := s.paymentRepo.Create(ctx, transaction); err != nil {
		return nil, err
	}

	// Create payment intent based on gateway
	switch gateway {
	case "stripe":
		return s.createStripePaymentIntent(ctx, transaction, payAmount, currency, paymentMethod)
	default:
		return nil, errors.New("unsupported payment gateway")
	}
}

func (s *paymentService) createStripePaymentIntent(ctx context.Context, transaction *models.PaymentTransaction, amount float64, currency, paymentMethod string) (*PaymentIntent, error) {
	if s.stripeKey == "" {
		return nil, errors.New("Stripe is not configured: set STRIPE_SECRET_KEY")
	}
	currency = strings.ToLower(currency)
	amountCents := int64(math.Round(amount * 100))
	stripe.Key = s.stripeKey

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String(currency),
		Metadata: map[string]string{
			"order_id":       transaction.OrderID.Hex(),
			"transaction_id": transaction.ID.Hex(),
		},
	}

	// Restrict Payment Sheet to what the shopper chose (card vs UPI), not both at once.
	pm := strings.ToLower(strings.TrimSpace(paymentMethod))
	if currency == "inr" {
		switch pm {
		case "upi", "wallet":
			params.PaymentMethodTypes = stripe.StringSlice([]string{"upi"})
		default:
			params.PaymentMethodTypes = stripe.StringSlice([]string{"card"})
		}
	} else {
		// Non-INR: card-only unless we explicitly add other rails later.
		params.PaymentMethodTypes = stripe.StringSlice([]string{"card"})
	}

	params.IdempotencyKey = stripe.String(fmt.Sprintf("pi-create-%s", transaction.ID.Hex()))

	_ = ctx
	pi, err := paymentintent.New(params)
	if err != nil {
		return nil, fmt.Errorf("stripe payment intent: %w", err)
	}
	s.paymentRepo.UpdateGatewayTransactionID(ctx, transaction.ID, pi.ID)
	return &PaymentIntent{
		ID:           pi.ID,
		ClientSecret: pi.ClientSecret,
		Gateway:      "stripe",
		Amount:       amount,
		Currency:     currency,
		Metadata: map[string]interface{}{
			"order_id":       transaction.OrderID.Hex(),
			"transaction_id": transaction.ID.Hex(),
		},
	}, nil
}

func (s *paymentService) ProcessPayment(ctx context.Context, requesterID primitive.ObjectID, requesterRole, paymentIntentID, paymentMethodID, gateway string) error {
	// Find transaction by gateway transaction ID
	transaction, err := s.paymentRepo.GetByGatewayTransactionID(ctx, paymentIntentID)
	if err != nil {
		return errors.New("payment transaction not found")
	}
	if requesterRole != "admin" && transaction.UserID != requesterID {
		return errors.New("access denied")
	}

	// Process payment based on gateway
	switch gateway {
	case "stripe":
		if s.stripeKey == "" {
			return errors.New("Stripe is not configured: set STRIPE_SECRET_KEY")
		}
		stripe.Key = s.stripeKey
		_ = ctx
		if strings.HasPrefix(paymentMethodID, "pm_") {
			// Web (Stripe Elements) flow: confirm the intent server-side with the
			// client-tokenized payment method.
			params := &stripe.PaymentIntentConfirmParams{
				PaymentMethod: stripe.String(paymentMethodID),
			}
			if _, err = paymentintent.Confirm(paymentIntentID, params); err != nil {
				reason := err.Error()
				s.paymentRepo.UpdateStatus(ctx, transaction.ID, "failed", &reason)
				return err
			}
		} else {
			// Mobile (PaymentSheet) flow: the intent was already confirmed on-device.
			// Verify with Stripe that it actually succeeded — never trust the client.
			pi, getErr := paymentintent.Get(paymentIntentID, nil)
			if getErr != nil {
				return getErr
			}
			if pi.Status != stripe.PaymentIntentStatusSucceeded {
				reason := fmt.Sprintf("payment not completed (status=%s)", pi.Status)
				s.paymentRepo.UpdateStatus(ctx, transaction.ID, "failed", &reason)
				return errors.New(reason)
			}
		}
		s.paymentRepo.UpdateStatus(ctx, transaction.ID, "success", nil)
		_ = s.markOrderPaidAfterGatewaySuccess(ctx, transaction.OrderID)
		return nil

	default:
		return errors.New("unsupported payment gateway")
	}
}

// markOrderPaidAfterGatewaySuccess sets order payment_status=paid and status=processing after successful charge.
func (s *paymentService) markOrderPaidAfterGatewaySuccess(ctx context.Context, orderID primitive.ObjectID) error {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if isCODPaymentMethod(order.PaymentMethod) {
		return nil
	}
	if order.PaymentStatus == "paid" {
		return nil
	}
	// Don't resurrect an order that was already cancelled (e.g. swept as abandoned).
	// Its stock is released; marking it paid/processing here would ship un-reserved
	// inventory. The webhook caller refunds; this also protects the synchronous path.
	if order.Status == "cancelled" {
		log.Printf("[PAYMENT][CRITICAL] gateway success for already-cancelled order %s — not marking paid; refund required", order.ID.Hex())
		return nil
	}
	order.PaymentStatus = "paid"
	order.Status = "processing"
	if err := s.orderRepo.Update(ctx, order.ID, order); err != nil {
		return err
	}
	// Payment is now confirmed — this is the right moment to tell the customer their
	// order is confirmed (not at checkout, before they paid).
	if s.paidNotifier != nil {
		go s.paidNotifier(context.Background(), order.ID)
	}
	return nil
}

func (s *paymentService) HandleWebhook(ctx context.Context, gateway string, payload []byte, signature string) error {
	switch gateway {
	case "stripe":
		return s.handleStripeWebhook(ctx, payload, signature)
	default:
		return errors.New("unsupported payment gateway")
	}
}

func (s *paymentService) handleStripeWebhook(ctx context.Context, payload []byte, signature string) (err error) {
	if s.stripeWebhookSecret == "" {
		return errors.New("Stripe webhook is not configured: set STRIPE_WEBHOOK_SECRET")
	}
	event, err := webhook.ConstructEvent(payload, signature, s.stripeWebhookSecret)
	if err != nil {
		return fmt.Errorf("stripe webhook verify: %w", err)
	}
	if s.stripeWebhookEvents != nil && event.ID != "" {
		var inserted bool
		inserted, err = s.stripeWebhookEvents.TryInsert(ctx, event.ID)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		defer func() {
			if err != nil {
				_ = s.stripeWebhookEvents.Delete(ctx, event.ID)
			}
		}()
	}
	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err = json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return err
		}
		var tx *models.PaymentTransaction
		tx, err = s.paymentRepo.GetByGatewayTransactionID(ctx, pi.ID)
		if err != nil {
			return err
		}
		if tx.Status == "success" {
			return nil
		}
		// Late-payment race: if the order was already cancelled (e.g. swept as an
		// abandoned unpaid order), its stock is released — don't resurrect/ship it.
		// Record the gateway success and refund the customer instead.
		if ord, gerr := s.orderRepo.GetByID(ctx, tx.OrderID); gerr == nil && ord != nil && ord.Status == "cancelled" {
			// The payment did succeed at the gateway, so record it; then refund (one
			// attempt). On refund failure, log CRITICAL for manual action rather than
			// returning an error — a webhook retry would early-return on tx=success and
			// never refund, silently leaving the customer charged.
			_ = s.paymentRepo.UpdateStatus(ctx, tx.ID, "success", nil)
			if rerr := s.RefundPayment(ctx, tx.ID, nil); rerr != nil {
				log.Printf("[PAYMENT][CRITICAL] refund FAILED for already-cancelled order %s (tx %s): %v — MANUAL REFUND REQUIRED", tx.OrderID.Hex(), tx.ID.Hex(), rerr)
			} else {
				log.Printf("[PAYMENT] refunded late payment for already-cancelled order %s (tx %s)", tx.OrderID.Hex(), tx.ID.Hex())
			}
			return nil
		}
		if err = s.paymentRepo.UpdateStatus(ctx, tx.ID, "success", nil); err != nil {
			return err
		}
		if err = s.markOrderPaidAfterGatewaySuccess(ctx, tx.OrderID); err != nil {
			return err
		}
		oid := tx.OrderID
		amt := tx.Amount
		cur := tx.Currency
		go func() {
			order, err := s.orderRepo.GetByID(context.Background(), oid)
			if err != nil {
				return
			}
			_ = s.emailService.SendPaymentReceived(order.ShippingInfo.Email, order, amt, cur)
		}()
	case "payment_intent.payment_failed":
		var pi stripe.PaymentIntent
		if err = json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return fmt.Errorf("stripe webhook payment_failed payload: %w", err)
		}
		transaction, _ := s.paymentRepo.GetByGatewayTransactionID(ctx, pi.ID)
		if transaction != nil {
			reason := "Payment failed"
			if err = s.paymentRepo.UpdateStatus(ctx, transaction.ID, "failed", &reason); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *paymentService) RefundPayment(ctx context.Context, transactionID primitive.ObjectID, amount *float64) error {
	transaction, err := s.paymentRepo.GetByID(ctx, transactionID)
	if err != nil {
		return errors.New("transaction not found")
	}

	if transaction.Status != "success" {
		return errors.New("can only refund successful transactions")
	}

	refundAmount := transaction.Amount
	if amount != nil {
		refundAmount = *amount
	}
	if refundAmount <= 0 {
		return errors.New("refund amount must be positive")
	}
	if refundAmount > transaction.Amount+1e-6 {
		return errors.New("refund amount cannot exceed original transaction amount")
	}

	switch transaction.Gateway {
	case "stripe":
		if s.stripeKey == "" {
			return errors.New("Stripe is not configured: set STRIPE_SECRET_KEY")
		}
		stripe.Key = s.stripeKey
		params := &stripe.RefundParams{
			PaymentIntent: stripe.String(transaction.GatewayTransactionID),
		}
		_ = ctx
		if amount != nil {
			params.Amount = stripe.Int64(int64(math.Round(refundAmount * 100)))
		}
		if _, err := refund.New(params); err != nil {
			return fmt.Errorf("stripe refund: %w", err)
		}
		s.paymentRepo.UpdateStatus(ctx, transactionID, "refunded", nil)
		return nil

	default:
		return errors.New("unsupported payment gateway")
	}
}

func (s *paymentService) GetTransaction(ctx context.Context, requesterID primitive.ObjectID, requesterRole string, id primitive.ObjectID) (*models.PaymentTransaction, error) {
	transaction, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if requesterRole != "admin" && transaction.UserID != requesterID {
		return nil, errors.New("access denied")
	}
	return transaction, nil
}

func expectedCurrencyByCountry(country string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(country)) {
	case "india", "in":
		return "inr", true
	case "united states", "us", "usa":
		return "usd", true
	default:
		return "", false
	}
}
