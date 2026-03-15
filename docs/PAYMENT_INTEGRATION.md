# Payment Gateway Integration Guide

## Overview

The payment flow is **production-ready with no mock implementations**. **Stripe** is fully integrated: the backend requires `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET` for card payments; the frontend requires `VITE_STRIPE_PUBLISHABLE_KEY` and uses Stripe.js/Elements so card data never touches your server. **PayPal** is not implemented; selecting it returns a clear error. Card and wallet payments require Stripe to be configured; otherwise the UI asks the user to use Cash on Delivery.

## Environment Variables

### Backend (`.env`)

```env
# Stripe (required for card/wallet payments; missing = API returns an error)
STRIPE_SECRET_KEY=sk_test_...   # From https://dashboard.stripe.com/apikeys
STRIPE_WEBHOOK_SECRET=whsec_... # From Developers → Webhooks (e.g. stripe listen --forward-to http://localhost:8080/api/webhooks/stripe)

# PayPal (not implemented; leave unset)
# PAYPAL_CLIENT_ID=...
# PAYPAL_SECRET=...
# PAYPAL_MODE=sandbox
```

### Frontend

Copy `frontend/.env.example` to `frontend/.env` and set (never commit `.env`):

```env
VITE_API_URL=http://localhost:8080/api
VITE_STRIPE_PUBLISHABLE_KEY=pk_test_...
```

## Secure setup (never commit keys)

1. **Backend:** `cp backend/.env.example backend/.env` — add your `STRIPE_SECRET_KEY` and (for local webhook) `STRIPE_WEBHOOK_SECRET`.  
2. **Frontend:** `cp frontend/.env.example frontend/.env` — add your `VITE_STRIPE_PUBLISHABLE_KEY`.  
3. Ensure `.env` and `backend/.env`, `frontend/.env` are in `.gitignore` (they are). Never commit files containing `sk_` or `pk_` keys.

## Testing Stripe (sandbox)

1. **Start backend** (from repo root):  
   `cd backend && go run ./cmd/server`  
   (Uses `backend/.env` if present; ensure `STRIPE_SECRET_KEY` is set.)

2. **Optional – webhook for “payment received” and order status:**  
   Install [Stripe CLI](https://stripe.com/docs/stripe-cli), then:  
   `stripe listen --forward-to http://localhost:8080/api/webhooks/stripe`  
   Put the printed `whsec_...` into `backend/.env` as `STRIPE_WEBHOOK_SECRET` and restart the backend.

3. **Start frontend:**  
   `cd frontend && pnpm run dev`  
   (Ensure `VITE_STRIPE_PUBLISHABLE_KEY` is in `frontend/.env`.)

4. **Test card:** In checkout choose “Credit/Debit Card”, place order, then use Stripe test card **4242 4242 4242 4242**, any future expiry (e.g. 12/34), any CVC, any billing details. Payment should succeed and you’ll see the order confirmation.

5. **Other test cards:** Decline `4000 0000 0000 0002`; 3D Secure `4000 0025 0000 3155`.

## Stripe integration (production)

The backend uses `github.com/stripe/stripe-go/v76` with **no fallbacks**:

- **Create Payment Intent** – Requires `STRIPE_SECRET_KEY`; returns error if unset. Creates a real PaymentIntent and returns `client_secret`.
- **Confirm Payment** – Requires Stripe key and a Stripe PaymentMethod id (`pm_...`). Rejects raw card data.
- **Webhook** – Requires `STRIPE_WEBHOOK_SECRET`; returns error if unset. Handles `payment_intent.succeeded` and `payment_intent.payment_failed`.
- **Refund** – Requires Stripe key; performs real refund via Stripe API.

## Installation (Stripe SDK – already added)

```bash
go get github.com/stripe/stripe-go/v76
```

### PayPal SDK

```bash
go get github.com/plutov/paypal/v4
```

## Implementation Steps

### 1. Update Payment Service

Replace the mock implementations in `internal/services/payment_service.go`:

#### Stripe Payment Intent Creation

```go
import "github.com/stripe/stripe-go/v76"
import "github.com/stripe/stripe-go/v76/paymentintent"

func (s *paymentService) createStripePaymentIntent(ctx context.Context, transaction *models.PaymentTransaction, amount float64, currency string) (*PaymentIntent, error) {
    stripe.Key = s.stripeKey
    
    params := &stripe.PaymentIntentParams{
        Amount:   stripe.Int64(int64(amount * 100)), // Convert to cents
        Currency: stripe.String(currency),
        Metadata: map[string]string{
            "order_id": transaction.OrderID.Hex(),
        },
    }
    
    pi, err := paymentintent.New(params)
    if err != nil {
        return nil, err
    }
    
    s.paymentRepo.UpdateGatewayTransactionID(ctx, transaction.ID, pi.ID)
    
    return &PaymentIntent{
        ID:           pi.ID,
        ClientSecret: pi.ClientSecret,
        Gateway:      "stripe",
        Amount:       amount,
        Currency:     currency,
    }, nil
}
```

#### PayPal Payment Creation

```go
import "github.com/plutov/paypal/v4"

func (s *paymentService) createPayPalPayment(ctx context.Context, transaction *models.PaymentTransaction, amount float64, currency string) (*PaymentIntent, error) {
    client, err := paypal.NewClient(s.paypalClientID, s.paypalSecret, paypal.APIBaseSandBox)
    if s.paypalMode == "live" {
        client, err = paypal.NewClient(s.paypalClientID, s.paypalSecret, paypal.APIBaseLive)
    }
    if err != nil {
        return nil, err
    }
    
    accessToken, err := client.GetAccessToken()
    if err != nil {
        return nil, err
    }
    
    payment := paypal.Payment{
        Intent: "sale",
        Payer: &paypal.Payer{
            PaymentMethod: "paypal",
        },
        Transactions: []paypal.Transaction{
            {
                Amount: &paypal.Amount{
                    Total:    fmt.Sprintf("%.2f", amount),
                    Currency: currency,
                },
            },
        },
        RedirectURLs: &paypal.RedirectURLs{
            ReturnURL: "https://yourdomain.com/payment/success",
            CancelURL: "https://yourdomain.com/payment/cancel",
        },
    }
    
    createdPayment, err := client.CreatePayment(payment, accessToken)
    if err != nil {
        return nil, err
    }
    
    approvalURL := ""
    for _, link := range createdPayment.Links {
        if link.Rel == "approval_url" {
            approvalURL = link.Href
            break
        }
    }
    
    s.paymentRepo.UpdateGatewayTransactionID(ctx, transaction.ID, createdPayment.ID)
    
    return &PaymentIntent{
        ID:        createdPayment.ID,
        PaymentURL: approvalURL,
        Gateway:   "paypal",
        Amount:    amount,
        Currency: currency,
    }, nil
}
```

### 2. Webhook Handling

#### Stripe Webhook

```go
import "github.com/stripe/stripe-go/v76/webhook"

func (s *paymentService) handleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
    event, err := webhook.ConstructEvent(payload, signature, s.stripeWebhookSecret)
    if err != nil {
        return err
    }
    
    switch event.Type {
    case "payment_intent.succeeded":
        var pi stripe.PaymentIntent
        err := json.Unmarshal(event.Data.Raw, &pi)
        if err != nil {
            return err
        }
        
        transaction, err := s.paymentRepo.GetByGatewayTransactionID(ctx, pi.ID)
        if err != nil {
            return err
        }
        
        s.paymentRepo.UpdateStatus(ctx, transaction.ID, "success", nil)
        s.orderRepo.UpdateStatus(ctx, transaction.OrderID, "processing")
        
    case "payment_intent.payment_failed":
        var pi stripe.PaymentIntent
        json.Unmarshal(event.Data.Raw, &pi)
        
        transaction, _ := s.paymentRepo.GetByGatewayTransactionID(ctx, pi.ID)
        if transaction != nil {
            reason := "Payment failed"
            s.paymentRepo.UpdateStatus(ctx, transaction.ID, "failed", &reason)
        }
    }
    
    return nil
}
```

#### PayPal Webhook

```go
func (s *paymentService) handlePayPalWebhook(ctx context.Context, payload []byte, signature string) error {
    // Verify webhook signature
    // Parse webhook event
    // Update transaction status accordingly
    
    // Example for payment completed:
    // transaction, _ := s.paymentRepo.GetByGatewayTransactionID(ctx, paymentID)
    // s.paymentRepo.UpdateStatus(ctx, transaction.ID, "success", nil)
    // s.orderRepo.UpdateStatus(ctx, transaction.OrderID, "processing")
    
    return nil
}
```

### 3. Payment Processing

#### Stripe Payment Confirmation

```go
func (s *paymentService) ProcessPayment(ctx context.Context, paymentIntentID, paymentMethodID string, gateway string) error {
    if gateway != "stripe" {
        return errors.New("invalid gateway")
    }
    
    stripe.Key = s.stripeKey
    
    params := &stripe.PaymentIntentConfirmParams{
        PaymentMethod: stripe.String(paymentMethodID),
    }
    
    pi, err := paymentintent.Confirm(paymentIntentID, params)
    if err != nil {
        transaction, _ := s.paymentRepo.GetByGatewayTransactionID(ctx, paymentIntentID)
        if transaction != nil {
            reason := err.Error()
            s.paymentRepo.UpdateStatus(ctx, transaction.ID, "failed", &reason)
        }
        return err
    }
    
    transaction, _ := s.paymentRepo.GetByGatewayTransactionID(ctx, paymentIntentID)
    if transaction != nil {
        s.paymentRepo.UpdateStatus(ctx, transaction.ID, "success", nil)
        s.orderRepo.UpdateStatus(ctx, transaction.OrderID, "processing")
    }
    
    return nil
}
```

### 4. Refund Processing

#### Stripe Refund

```go
import "github.com/stripe/stripe-go/v76/refund"

func (s *paymentService) RefundPayment(ctx context.Context, transactionID primitive.ObjectID, amount *float64) error {
    transaction, err := s.paymentRepo.GetByID(ctx, transactionID)
    if err != nil {
        return err
    }
    
    if transaction.Gateway != "stripe" {
        return errors.New("invalid gateway")
    }
    
    stripe.Key = s.stripeKey
    
    params := &stripe.RefundParams{
        PaymentIntent: stripe.String(transaction.GatewayTransactionID),
    }
    
    if amount != nil {
        params.Amount = stripe.Int64(int64(*amount * 100))
    }
    
    _, err = refund.New(params)
    if err != nil {
        return err
    }
    
    s.paymentRepo.UpdateStatus(ctx, transactionID, "refunded", nil)
    return nil
}
```

## API Endpoints

### Create Payment Intent

```http
POST /api/payments/intent
Authorization: Bearer <token>
Content-Type: application/json

{
  "order_id": "order_id_here",
  "amount": 100.00,
  "currency": "usd",
  "gateway": "stripe"  // or "paypal"
}
```

Response:
```json
{
  "id": "pi_...",
  "client_secret": "pi_..._secret_...",
  "gateway": "stripe",
  "amount": 100.00,
  "currency": "usd"
}
```

### Process Payment (Stripe)

```http
POST /api/payments/process
Authorization: Bearer <token>
Content-Type: application/json

{
  "payment_intent_id": "pi_...",
  "payment_method_id": "pm_...",
  "gateway": "stripe"
}
```

### Webhook Endpoints

- Stripe: `POST /api/webhooks/stripe`
- PayPal: `POST /api/webhooks/paypal`

Configure these URLs in your Stripe/PayPal dashboards.

### Refund Payment

```http
POST /api/payments/refund/:transaction_id
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "amount": 50.00  // Optional: partial refund
}
```

## Testing

### Stripe Test Cards

- Success: `4242 4242 4242 4242`
- Decline: `4000 0000 0000 0002`
- Requires 3D Secure: `4000 0025 0000 3155`

### PayPal Sandbox

Use PayPal sandbox accounts for testing. Create test accounts in PayPal Developer Dashboard.

## Security Notes

1. **Never expose secret keys** in frontend code
2. **Always verify webhook signatures** before processing
3. **Use HTTPS** in production
4. **Store sensitive data** securely
5. **Implement rate limiting** on payment endpoints
6. **Log all payment transactions** for audit

## Next Steps

1. Install the SDKs: `go get github.com/stripe/stripe-go/v76` and `go get github.com/plutov/paypal/v4`
2. Replace mock implementations with real API calls
3. Test with sandbox/test credentials
4. Configure webhook endpoints in Stripe/PayPal dashboards
5. Update frontend to handle payment flows
6. Test end-to-end payment processing
7. Deploy to production with live credentials
