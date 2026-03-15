# Plan Enhancements: Production-Ready Features

This document extends the base backend implementation plan with additional features for a complete production-ready apparel e-commerce solution.

## Additional Database Collections

### 10. Product Reviews Collection
```go
type ProductReview struct {
    ID          primitive.ObjectID `bson:"_id" json:"id"`
    ProductID   primitive.ObjectID `bson:"product_id" json:"product_id"`
    UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
    UserName    string             `bson:"user_name" json:"user_name"`
    Rating      int                `bson:"rating" json:"rating"` // 1-5
    Title       string             `bson:"title" json:"title"`
    Comment     string             `bson:"comment" json:"comment"`
    Images      []string           `bson:"images,omitempty" json:"images,omitempty"`
    Verified    bool               `bson:"verified" json:"verified"` // Verified purchase
    Helpful     int                `bson:"helpful" json:"helpful"` // Helpful votes
    Status      string             `bson:"status" json:"status"` // "pending", "approved", "rejected"
    CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
```

### 11. Returns & Refunds Collection
```go
type Return struct {
    ID              primitive.ObjectID `bson:"_id" json:"id"`
    OrderID         primitive.ObjectID `bson:"order_id" json:"order_id"`
    OrderNumber     string             `bson:"order_number" json:"order_number"`
    UserID          primitive.ObjectID `bson:"user_id" json:"user_id"`
    Items           []ReturnItem       `bson:"items" json:"items"`
    Reason          string             `bson:"reason" json:"reason"`
    Description     string             `bson:"description" json:"description"`
    Status          string             `bson:"status" json:"status"` // "requested", "approved", "rejected", "processing", "completed"
    RefundAmount    float64            `bson:"refund_amount" json:"refund_amount"`
    RefundMethod    string             `bson:"refund_method" json:"refund_method"` // "original", "store_credit"
    RefundStatus    string             `bson:"refund_status" json:"refund_status"` // "pending", "processed", "failed"
    TrackingNumber  *string            `bson:"tracking_number,omitempty" json:"tracking_number,omitempty"`
    CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}

type ReturnItem struct {
    OrderItemID primitive.ObjectID `bson:"order_item_id" json:"order_item_id"`
    ProductID   primitive.ObjectID `bson:"product_id" json:"product_id"`
    ProductName string             `bson:"product_name" json:"product_name"`
    Size        string             `bson:"size" json:"size"`
    Quantity    int                `bson:"quantity" json:"quantity"`
    Price       float64            `bson:"price" json:"price"`
}
```

### 12. Shipping Methods Collection
```go
type ShippingMethod struct {
    ID              primitive.ObjectID `bson:"_id" json:"id"`
    Name            string             `bson:"name" json:"name"`
    Carrier         string             `bson:"carrier" json:"carrier"` // "fedex", "ups", "dhl", "custom"
    Type            string             `bson:"type" json:"type"` // "standard", "express", "overnight"
    BaseCost        float64            `bson:"base_cost" json:"base_cost"`
    CostPerKg       *float64           `bson:"cost_per_kg,omitempty" json:"cost_per_kg,omitempty"`
    FreeShippingThreshold *float64     `bson:"free_shipping_threshold,omitempty" json:"free_shipping_threshold,omitempty"`
    EstimatedDays   int                `bson:"estimated_days" json:"estimated_days"`
    Zones           []ShippingZone     `bson:"zones" json:"zones"`
    IsActive        bool               `bson:"is_active" json:"is_active"`
    CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
}

type ShippingZone struct {
    Countries       []string           `bson:"countries" json:"countries"`
    Cost            float64            `bson:"cost" json:"cost"`
    EstimatedDays   int                `bson:"estimated_days" json:"estimated_days"`
}
```

### 13. Tax Rates Collection
```go
type TaxRate struct {
    ID              primitive.ObjectID `bson:"_id" json:"id"`
    Country         string             `bson:"country" json:"country"`
    State           *string            `bson:"state,omitempty" json:"state,omitempty"`
    City            *string            `bson:"city,omitempty" json:"city,omitempty"`
    PostalCode      *string            `bson:"postal_code,omitempty" json:"postal_code,omitempty"`
    Rate            float64            `bson:"rate" json:"rate"` // Percentage (e.g., 8.0 for 8%)
    TaxType         string             `bson:"tax_type" json:"tax_type"` // "gst", "vat", "sales_tax"
    IsActive        bool               `bson:"is_active" json:"is_active"`
    CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
}
```

### 14. Payment Transactions Collection
```go
type PaymentTransaction struct {
    ID              primitive.ObjectID `bson:"_id" json:"id"`
    OrderID         primitive.ObjectID `bson:"order_id" json:"order_id"`
    UserID          primitive.ObjectID `bson:"user_id" json:"user_id"`
    Amount          float64            `bson:"amount" json:"amount"`
    Currency        string             `bson:"currency" json:"currency"`
    PaymentMethod   string             `bson:"payment_method" json:"payment_method"`
    Gateway         string             `bson:"gateway" json:"gateway"` // "stripe", "razorpay", "payu"
    GatewayTransactionID string        `bson:"gateway_transaction_id" json:"gateway_transaction_id"`
    Status          string             `bson:"status" json:"status"` // "pending", "processing", "success", "failed", "refunded"
    FailureReason   *string            `bson:"failure_reason,omitempty" json:"failure_reason,omitempty"`
    Metadata        map[string]interface{} `bson:"metadata" json:"metadata"`
    CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}
```

### 15. Support Tickets Collection
```go
type SupportTicket struct {
    ID              primitive.ObjectID `bson:"_id" json:"id"`
    UserID          primitive.ObjectID `bson:"user_id" json:"user_id"`
    OrderID         *primitive.ObjectID `bson:"order_id,omitempty" json:"order_id,omitempty"`
    Subject         string             `bson:"subject" json:"subject"`
    Category        string             `bson:"category" json:"category"` // "order", "product", "payment", "shipping", "other"
    Priority        string             `bson:"priority" json:"priority"` // "low", "medium", "high", "urgent"
    Status          string             `bson:"status" json:"status"` // "open", "in_progress", "resolved", "closed"
    Messages        []TicketMessage    `bson:"messages" json:"messages"`
    AssignedTo      *primitive.ObjectID `bson:"assigned_to,omitempty" json:"assigned_to,omitempty"`
    CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
}

type TicketMessage struct {
    ID          primitive.ObjectID `bson:"_id" json:"id"`
    UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
    IsAdmin     bool               `bson:"is_admin" json:"is_admin"`
    Message     string             `bson:"message" json:"message"`
    Attachments []string           `bson:"attachments,omitempty" json:"attachments,omitempty"`
    CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}
```

## Additional API Endpoints

### Product Reviews
- `GET /api/products/:id/reviews` - Get product reviews
- `POST /api/products/:id/reviews` - Submit review (authenticated, verified purchase only)
- `PUT /api/reviews/:id` - Update own review
- `DELETE /api/reviews/:id` - Delete own review
- `POST /api/reviews/:id/helpful` - Mark review as helpful
- `GET /api/admin/reviews` - List all reviews (admin)
- `PUT /api/admin/reviews/:id/status` - Approve/reject review (admin)

### Returns & Refunds
- `POST /api/orders/:id/return` - Request return
- `GET /api/returns` - Get user returns
- `GET /api/returns/:id` - Get return details
- `PUT /api/admin/returns/:id/approve` - Approve return (admin)
- `PUT /api/admin/returns/:id/reject` - Reject return (admin)
- `PUT /api/admin/returns/:id/process` - Process refund (admin)
- `GET /api/admin/returns` - List all returns (admin)

### Shipping
- `GET /api/shipping/methods` - Get available shipping methods
- `POST /api/shipping/calculate` - Calculate shipping cost
- `GET /api/admin/shipping/methods` - List shipping methods (admin)
- `POST /api/admin/shipping/methods` - Create shipping method (admin)
- `PUT /api/admin/shipping/methods/:id` - Update shipping method (admin)
- `DELETE /api/admin/shipping/methods/:id` - Delete shipping method (admin)

### Tax
- `POST /api/tax/calculate` - Calculate tax for order
- `GET /api/admin/tax/rates` - List tax rates (admin)
- `POST /api/admin/tax/rates` - Create tax rate (admin)
- `PUT /api/admin/tax/rates/:id` - Update tax rate (admin)
- `DELETE /api/admin/tax/rates/:id` - Delete tax rate (admin)

### Payment Gateway
- `POST /api/payments/create-intent` - Create payment intent (Stripe/Razorpay)
- `POST /api/payments/confirm` - Confirm payment
- `POST /api/payments/webhook` - Payment webhook handler
- `POST /api/payments/refund` - Process refund (admin)
- `GET /api/admin/payments/transactions` - List payment transactions (admin)

### Support
- `POST /api/support/tickets` - Create support ticket
- `GET /api/support/tickets` - Get user tickets
- `GET /api/support/tickets/:id` - Get ticket details
- `POST /api/support/tickets/:id/messages` - Add message to ticket
- `GET /api/admin/support/tickets` - List all tickets (admin)
- `PUT /api/admin/support/tickets/:id/assign` - Assign ticket (admin)
- `PUT /api/admin/support/tickets/:id/status` - Update ticket status (admin)

### Email Service
- `POST /api/admin/email/test` - Send test email (admin)
- `GET /api/admin/email/templates` - List email templates (admin)
- `PUT /api/admin/email/templates/:id` - Update email template (admin)

## Implementation Phases

### Phase 6: Product Reviews System
1. **Review Repository & Service**
   - Create review repository
   - Review service with moderation
   - Auto-update product ratings
   - Helpful vote tracking

2. **Review Handlers**
   - Submit review (with purchase verification)
   - Get reviews with pagination
   - Review moderation endpoints
   - Helpful voting

3. **Integration**
   - Update product model to recalculate ratings
   - Add review count to products
   - Review moderation workflow

### Phase 7: Returns & Refunds Management
1. **Return Repository & Service**
   - Return request creation
   - Return approval workflow
   - Refund calculation
   - Refund processing

2. **Return Handlers**
   - Create return request
   - Return status management
   - Refund processing
   - Return tracking

3. **Integration**
   - Link returns to orders
   - Update order status on return
   - Restore inventory on return
   - Process refunds via payment gateway

### Phase 8: Payment Gateway Integration
1. **Stripe Integration**
   - Payment intent creation
   - Webhook handling
   - Refund processing
   - Payment status tracking

2. **Razorpay Integration (India)**
   - Order creation
   - Payment verification
   - Webhook handling
   - Refund processing

3. **Payment Service**
   - Unified payment interface
   - Gateway abstraction
   - Transaction logging
   - Payment retry logic

### Phase 9: Email Service Implementation
1. **Email Service**
   - SMTP configuration
   - Email template system
   - Queue for async sending
   - Email delivery tracking

2. **Email Templates**
   - Order confirmation
   - Shipping notification
   - Order status updates
   - Password reset
   - Return confirmation
   - Refund notification

3. **Integration**
   - Send emails on order events
   - Email preferences per user
   - Unsubscribe handling

### Phase 10: Advanced Shipping Management
1. **Shipping Service**
   - Shipping method management
   - Rate calculation by zone
   - Carrier API integration (optional)
   - Shipping label generation (optional)

2. **Shipping Handlers**
   - Calculate shipping costs
   - Get available methods
   - Admin shipping management
   - Tracking integration

3. **Integration**
   - Update order service to use shipping methods
   - Dynamic shipping cost calculation
   - Shipping zone management

### Phase 11: Dynamic Tax Calculation
1. **Tax Service**
   - Tax rate management
   - Location-based tax calculation
   - Tax exemption handling
   - GST/VAT support

2. **Tax Handlers**
   - Calculate tax endpoint
   - Admin tax rate management
   - Tax reporting

3. **Integration**
   - Update order service to use tax service
   - Dynamic tax calculation in checkout
   - Tax breakdown in orders

### Phase 12: Inventory Management Enhancements
1. **Inventory Alerts**
   - Low stock detection
   - Stock alert notifications
   - Reorder point management

2. **Multi-Warehouse Support**
   - Warehouse management
   - Stock allocation
   - Transfer management

3. **Stock Reservation**
   - Reserve stock on cart
   - Release on timeout
   - Hold for orders

### Phase 13: Customer Support System
1. **Support Service**
   - Ticket creation and management
   - Message threading
   - Assignment workflow
   - Priority management

2. **Support Handlers**
   - Create ticket
   - Ticket management
   - Message handling
   - Admin assignment

3. **Integration**
   - Link tickets to orders
   - Email notifications for tickets
   - Support analytics

## Updated Environment Variables

```env
# Payment Gateways
STRIPE_SECRET_KEY=sk_test_...
STRIPE_PUBLISHABLE_KEY=pk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...

RAZORPAY_KEY_ID=rzp_test_...
RAZORPAY_KEY_SECRET=...
RAZORPAY_WEBHOOK_SECRET=...

# Shipping (Optional)
SHIPPING_CARRIER_API_KEY=...
FEDEX_API_KEY=...
UPS_API_KEY=...

# Tax (Optional)
TAX_API_KEY=... # For tax calculation services
```

## Additional Dependencies

```go
// Payment gateways
github.com/stripe/stripe-go/v76
github.com/razorpay/razorpay-go

// Email
github.com/go-mail/mail/v2

// Shipping (optional)
// Carrier-specific SDKs

// Tax calculation (optional)
// Tax service SDKs
```

## Implementation Priority

### Critical (Before Launch)
1. Payment Gateway Integration
2. Email Service
3. Returns & Refunds (basic)

### Important (Post-Launch)
4. Product Reviews
5. Advanced Shipping
6. Dynamic Tax

### Nice to Have
7. Inventory Alerts
8. Customer Support System
9. Multi-warehouse

## Testing Requirements

1. Payment gateway webhook testing
2. Email delivery testing
3. Return workflow testing
4. Tax calculation accuracy
5. Shipping cost calculation
6. Review moderation workflow

## Security Considerations

1. Payment data encryption
2. PCI DSS compliance for payment handling
3. Secure webhook verification
4. Email template injection prevention
5. Return fraud prevention
6. Review spam prevention
