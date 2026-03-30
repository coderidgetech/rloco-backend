package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	stripeKey           string
	stripeWebhookSecret string
}

func NewPaymentService(paymentRepo repositories.PaymentRepository, orderRepo repositories.OrderRepository, emailService EmailService, stripeKey, stripeWebhookSecret string) PaymentService {
	return &paymentService{
		paymentRepo:         paymentRepo,
		orderRepo:           orderRepo,
		emailService:        emailService,
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
	if expected, ok := expectedCurrencyByCountry(order.ShippingInfo.Country); ok {
		incoming := strings.ToLower(strings.TrimSpace(currency))
		if incoming != expected {
			return nil, fmt.Errorf("currency mismatch for shipping country: expected %s", expected)
		}
	}

	// Create payment transaction record
	transaction := &models.PaymentTransaction{
		OrderID:       orderID,
		UserID:        order.UserID,
		Amount:        amount,
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
		return s.createStripePaymentIntent(ctx, transaction, amount, currency, paymentMethod)
	default:
		return nil, errors.New("unsupported payment gateway")
	}
}

func (s *paymentService) createStripePaymentIntent(ctx context.Context, transaction *models.PaymentTransaction, amount float64, currency, paymentMethod string) (*PaymentIntent, error) {
	if s.stripeKey == "" {
		return nil, errors.New("Stripe is not configured: set STRIPE_SECRET_KEY")
	}
	currency = strings.ToLower(currency)
	amountCents := int64(amount * 100)
	stripe.Key = s.stripeKey

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amountCents),
		Currency: stripe.String(currency),
		Metadata: map[string]string{
			"order_id":       transaction.OrderID.Hex(),
			"transaction_id": transaction.ID.Hex(),
		},
	}

	// For INR: always set payment_method_types so UPI is available (Stripe only shows card if we use automatic and UPI isn't enabled in Dashboard)
	if currency == "inr" {
		if paymentMethod == "upi" || paymentMethod == "wallet" {
			params.PaymentMethodTypes = stripe.StringSlice([]string{"upi", "card"})
		} else {
			params.PaymentMethodTypes = stripe.StringSlice([]string{"card", "upi"})
		}
	} else {
		// USD or other: use automatic payment methods from Dashboard
		params.AutomaticPaymentMethods = &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		}
	}

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
		if !strings.HasPrefix(paymentMethodID, "pm_") {
			return errors.New("invalid payment method: use Stripe Elements to pay with card (card details must not be sent to the server)")
		}
		stripe.Key = s.stripeKey
		params := &stripe.PaymentIntentConfirmParams{
			PaymentMethod: stripe.String(paymentMethodID),
		}
		_ = ctx
		_, err = paymentintent.Confirm(paymentIntentID, params)
		if err != nil {
			reason := err.Error()
			s.paymentRepo.UpdateStatus(ctx, transaction.ID, "failed", &reason)
			return err
		}
		s.paymentRepo.UpdateStatus(ctx, transaction.ID, "success", nil)
		s.orderRepo.UpdateStatus(ctx, transaction.OrderID, "processing")
		return nil

	default:
		return errors.New("unsupported payment gateway")
	}
}

func (s *paymentService) HandleWebhook(ctx context.Context, gateway string, payload []byte, signature string) error {
	switch gateway {
	case "stripe":
		return s.handleStripeWebhook(ctx, payload, signature)
	default:
		return errors.New("unsupported payment gateway")
	}
}

func (s *paymentService) handleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
	if s.stripeWebhookSecret == "" {
		return errors.New("Stripe webhook is not configured: set STRIPE_WEBHOOK_SECRET")
	}
	event, err := webhook.ConstructEvent(payload, signature, s.stripeWebhookSecret)
	if err != nil {
		return fmt.Errorf("stripe webhook verify: %w", err)
	}
	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
			return err
		}
		transaction, err := s.paymentRepo.GetByGatewayTransactionID(ctx, pi.ID)
		if err != nil {
			return err
		}
		s.paymentRepo.UpdateStatus(ctx, transaction.ID, "success", nil)
		s.orderRepo.UpdateStatus(ctx, transaction.OrderID, "processing")
		// Notify customer that payment was received
		go func() {
			order, err := s.orderRepo.GetByID(context.Background(), transaction.OrderID)
			if err != nil {
				return
			}
			_ = s.emailService.SendPaymentReceived(order.ShippingInfo.Email, order.OrderNumber, transaction.Amount, transaction.Currency)
		}()
	case "payment_intent.payment_failed":
		var pi stripe.PaymentIntent
		_ = json.Unmarshal(event.Data.Raw, &pi)
		transaction, _ := s.paymentRepo.GetByGatewayTransactionID(ctx, pi.ID)
		if transaction != nil {
			reason := "Payment failed"
			s.paymentRepo.UpdateStatus(ctx, transaction.ID, "failed", &reason)
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
			params.Amount = stripe.Int64(int64(refundAmount * 100))
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
