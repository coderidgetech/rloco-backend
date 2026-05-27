package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents a user in the system
type User struct {
	ID           primitive.ObjectID  `bson:"_id" json:"id"`
	Email        string              `bson:"email" json:"email"`
	PasswordHash string              `bson:"password_hash" json:"-"`
	Name         string              `bson:"name" json:"name"`
	Role         string              `bson:"role" json:"role"` // "customer", "admin", "vendor"
	VendorID     *primitive.ObjectID `bson:"vendor_id,omitempty" json:"vendor_id,omitempty"`
	Avatar       string              `bson:"avatar" json:"avatar"`
	Phone        *string             `bson:"phone,omitempty" json:"phone,omitempty"`
	PhoneKey     string              `bson:"phone_key,omitempty" json:"-"` // digits-only lookup key; not exposed in JSON
	Birthday     *time.Time          `bson:"birthday,omitempty" json:"birthday,omitempty"`
	City         string              `bson:"city,omitempty" json:"city,omitempty"`
	Active       bool                `bson:"active" json:"active"` // User account status
	EmailVerified bool               `bson:"email_verified" json:"email_verified"`
	FCMTokens    []string            `bson:"fcm_tokens,omitempty" json:"-"` // Firebase Cloud Messaging device tokens (max 5)
	CreatedAt    time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time           `bson:"updated_at" json:"updated_at"`
}

// Product represents a product in the catalog
type Product struct {
	ID               primitive.ObjectID  `bson:"_id" json:"id"`
	Name             string              `bson:"name" json:"name"`
	SKU              string              `bson:"sku" json:"sku"`
	Price            float64             `bson:"price" json:"price"`
	OriginalPrice    *float64            `bson:"original_price,omitempty" json:"original_price,omitempty"`
	PriceINR         *float64            `bson:"price_inr,omitempty" json:"price_inr,omitempty"`
	OriginalPriceINR *float64            `bson:"original_price_inr,omitempty" json:"original_price_inr,omitempty"`
	Images           []string            `bson:"images" json:"images"`
	Category         string              `bson:"category" json:"category"`
	Subcategory      string              `bson:"subcategory" json:"subcategory"`
	Gender           string              `bson:"gender" json:"gender"` // "women", "men", "unisex"
	Colors           []string            `bson:"colors" json:"colors"`
	Sizes            []string            `bson:"sizes" json:"sizes"`
	Description      string              `bson:"description" json:"description"`
	Details          []string            `bson:"details" json:"details"`
	Material         string              `bson:"material" json:"material"`
	Care             string              `bson:"care" json:"care"`
	Featured         bool                `bson:"featured" json:"featured"`
	NewArrival       bool                `bson:"new_arrival" json:"new_arrival"`
	OnSale           bool                `bson:"on_sale" json:"on_sale"`
	IsGift           bool                `bson:"is_gift" json:"is_gift"` // Gift-worthy product (show in Gift For Her/Him, allow gift wrap)
	Rating           float64             `bson:"rating" json:"rating"`
	Reviews          int                 `bson:"reviews" json:"reviews"`
	Badge            *string             `bson:"badge,omitempty" json:"badge,omitempty"`
	VideoURL         *string             `bson:"video_url,omitempty" json:"video_url,omitempty"`
	Stock            map[string]int      `bson:"stock" json:"stock"` // size -> quantity
	// AvailableMarkets lists markets where the product is sold (e.g. "IN", "US").
	AvailableMarkets []string            `bson:"available_markets,omitempty" json:"available_markets,omitempty"`
	VendorID         *primitive.ObjectID `bson:"vendor_id,omitempty" json:"vendor_id,omitempty"`
	// Variant fields — products sharing the same VariantGroupID are color siblings.
	VariantGroupID *primitive.ObjectID `bson:"variant_group_id,omitempty" json:"variant_group_id,omitempty"`
	Color          string              `bson:"color,omitempty" json:"color,omitempty"`
	IsMainVariant  bool                `bson:"is_main_variant,omitempty" json:"is_main_variant,omitempty"`
	CreatedAt      time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time           `bson:"updated_at" json:"updated_at"`
}

// Category represents a product category
type Category struct {
	ID            primitive.ObjectID `bson:"_id" json:"id"`
	Name          string             `bson:"name" json:"name"`
	Slug          string             `bson:"slug" json:"slug"`
	Gender        string             `bson:"gender" json:"gender"` // "women", "men", "unisex"
	Subcategories []string           `bson:"subcategories" json:"subcategories"`
	Image         string             `bson:"image" json:"image"`
	Order         int                `bson:"order" json:"order"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
}

// Order represents an order
type Order struct {
	ID             primitive.ObjectID  `bson:"_id" json:"id"`
	OrderNumber    string              `bson:"order_number" json:"order_number"`
	UserID         primitive.ObjectID  `bson:"user_id" json:"user_id"`
	GuestEmail     *string             `bson:"guest_email,omitempty" json:"guest_email,omitempty"`
	GuestName      *string             `bson:"guest_name,omitempty" json:"guest_name,omitempty"`
	Items          []OrderItem        `bson:"items" json:"items"`
	ShippingInfo   ShippingInfo       `bson:"shipping_info" json:"shipping_info"`
	PaymentInfo    PaymentInfo        `bson:"payment_info" json:"payment_info"`
	Subtotal         float64            `bson:"subtotal" json:"subtotal"`
	Discount         float64            `bson:"discount" json:"discount"`
	ShippingCost     float64            `bson:"shipping_cost" json:"shipping_cost"`
	GiftPackingCharge float64            `bson:"gift_packing_charge" json:"gift_packing_charge"` // Sum of gift packing (e.g. 50 per gift item)
	Tax              float64            `bson:"tax" json:"tax"`
	Total            float64            `bson:"total" json:"total"`
	Status         string             `bson:"status" json:"status"` // "pending", "processing", "shipped", "delivered", "cancelled"
	PaymentMethod  string             `bson:"payment_method" json:"payment_method"`
	PaymentStatus  string             `bson:"payment_status" json:"payment_status"` // "pending", "paid", "failed"
	TrackingNumber      *string            `bson:"tracking_number,omitempty" json:"tracking_number,omitempty"`
	LabelURL            *string            `bson:"label_url,omitempty" json:"label_url,omitempty"`
	PromotionCode       *string            `bson:"promotion_code,omitempty" json:"promotion_code,omitempty"`
	RewardPointsApplied int64              `bson:"reward_points_applied,omitempty" json:"reward_points_applied,omitempty"`
	CreatedAt           time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time          `bson:"updated_at" json:"updated_at"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ProductID     primitive.ObjectID `bson:"product_id" json:"product_id"`
	ProductName   string             `bson:"product_name" json:"product_name"`
	Image         string             `bson:"image" json:"image"`
	Price         float64            `bson:"price" json:"price"`
	Size          string             `bson:"size" json:"size"`
	Quantity      int                `bson:"quantity" json:"quantity"`
	IsGift        bool               `bson:"is_gift" json:"is_gift"`
	GiftWrapColor string             `bson:"gift_wrap_color,omitempty" json:"gift_wrap_color,omitempty"`
	GiftMessage   string             `bson:"gift_message,omitempty" json:"gift_message,omitempty"`
}

// ShippingInfo represents shipping information
type ShippingInfo struct {
	FirstName string `bson:"first_name" json:"first_name"`
	LastName  string `bson:"last_name" json:"last_name"`
	Email     string `bson:"email" json:"email"`
	Phone     string `bson:"phone" json:"phone"`
	Address   string `bson:"address" json:"address"`
	City      string `bson:"city" json:"city"`
	State     string `bson:"state" json:"state"`
	ZipCode   string `bson:"zip_code" json:"zip_code"`
	Country   string `bson:"country" json:"country"`
}

// PaymentInfo represents payment information
type PaymentInfo struct {
	UPIID      string `bson:"upi_id,omitempty" json:"upi_id,omitempty"`
	WalletName string `bson:"wallet_name,omitempty" json:"wallet_name,omitempty"`
}

// Cart represents a shopping cart
type Cart struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Items     []CartItem         `bson:"items" json:"items"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// CartItem represents an item in a cart
type CartItem struct {
	ProductID     primitive.ObjectID `bson:"product_id" json:"product_id"`
	ProductName   string             `bson:"product_name" json:"product_name"`
	Image         string             `bson:"image" json:"image"`
	Price         float64            `bson:"price" json:"price"`
	PriceINR      *float64           `bson:"price_inr,omitempty" json:"price_inr,omitempty"`
	Size          string             `bson:"size" json:"size"`
	Quantity      int                `bson:"quantity" json:"quantity"`
	IsGift        bool               `bson:"is_gift" json:"is_gift"`
	GiftWrapColor string             `bson:"gift_wrap_color,omitempty" json:"gift_wrap_color,omitempty"`
	GiftMessage   string             `bson:"gift_message,omitempty" json:"gift_message,omitempty"`
}

// Wishlist represents a wishlist
type Wishlist struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	ProductID primitive.ObjectID `bson:"product_id" json:"product_id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// Promotion represents a promotion/coupon
type Promotion struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Code        string             `bson:"code" json:"code"`
	Type        string             `bson:"type" json:"type"` // "percentage", "fixed", "free_shipping"
	Value       float64            `bson:"value" json:"value"`
	MinPurchase *float64           `bson:"min_purchase,omitempty" json:"min_purchase,omitempty"`
	MaxDiscount *float64           `bson:"max_discount,omitempty" json:"max_discount,omitempty"`
	StartDate   time.Time          `bson:"start_date" json:"start_date"`
	EndDate     time.Time          `bson:"end_date" json:"end_date"`
	UsageCount  int                `bson:"usage_count" json:"usage_count"`
	UsageLimit  *int               `bson:"usage_limit,omitempty" json:"usage_limit,omitempty"`
	IsActive    bool               `bson:"is_active" json:"is_active"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

// Vendor represents a vendor
type Vendor struct {
	ID               primitive.ObjectID     `bson:"_id" json:"id"`
	Name             string                 `bson:"name" json:"name"`
	Email            string                 `bson:"email" json:"email"`
	Logo             string                 `bson:"logo" json:"logo"`
	SubscriptionPlan string                 `bson:"subscription_plan" json:"subscription_plan"`
	Permissions      map[string]interface{} `bson:"permissions" json:"permissions"`
	Preferences      map[string]interface{} `bson:"preferences,omitempty" json:"preferences,omitempty"`
	Status           string                 `bson:"status" json:"status"` // "active", "suspended", "pending"
	CreatedAt        time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time              `bson:"updated_at" json:"updated_at"`
}

// SiteConfig represents site configuration
type SiteConfig struct {
	ID        primitive.ObjectID     `bson:"_id" json:"id"`
	Config    map[string]interface{} `bson:"config" json:"config"`
	UpdatedAt time.Time              `bson:"updated_at" json:"updated_at"`
}

// AnalyticsEvent represents an analytics event
type AnalyticsEvent struct {
	ID        primitive.ObjectID     `bson:"_id" json:"id"`
	EventType string                 `bson:"event_type" json:"event_type"` // "page_view", "add_to_cart", "purchase", etc.
	UserID    *primitive.ObjectID    `bson:"user_id,omitempty" json:"user_id,omitempty"`
	ProductID *primitive.ObjectID    `bson:"product_id,omitempty" json:"product_id,omitempty"`
	OrderID   *primitive.ObjectID    `bson:"order_id,omitempty" json:"order_id,omitempty"`
	Metadata  map[string]interface{} `bson:"metadata" json:"metadata"`
	CreatedAt time.Time              `bson:"created_at" json:"created_at"`
}

// ProductReview represents a product review
type ProductReview struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	ProductID primitive.ObjectID `bson:"product_id" json:"product_id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	UserName  string             `bson:"user_name" json:"user_name"`
	Rating    int                `bson:"rating" json:"rating"` // 1-5
	Title     string             `bson:"title" json:"title"`
	Comment   string             `bson:"comment" json:"comment"`
	Images    []string           `bson:"images,omitempty" json:"images,omitempty"`
	Verified  bool               `bson:"verified" json:"verified"` // Verified purchase
	Helpful   int                `bson:"helpful" json:"helpful"`   // Helpful votes
	Status    string             `bson:"status" json:"status"`     // "pending", "approved", "rejected"
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// Return represents a return/refund request
type Return struct {
	ID             primitive.ObjectID `bson:"_id" json:"id"`
	OrderID        primitive.ObjectID `bson:"order_id" json:"order_id"`
	OrderNumber    string             `bson:"order_number" json:"order_number"`
	UserID         primitive.ObjectID `bson:"user_id" json:"user_id"`
	Items          []ReturnItem       `bson:"items" json:"items"`
	Reason         string             `bson:"reason" json:"reason"`
	Description    string             `bson:"description" json:"description"`
	Status         string             `bson:"status" json:"status"` // "requested", "approved", "rejected", "processing", "completed"
	RefundAmount   float64            `bson:"refund_amount" json:"refund_amount"`
	RefundMethod   string             `bson:"refund_method" json:"refund_method"` // "original", "store_credit"
	RefundStatus   string             `bson:"refund_status" json:"refund_status"` // "pending", "processed", "failed"
	TrackingNumber *string            `bson:"tracking_number,omitempty" json:"tracking_number,omitempty"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at" json:"updated_at"`
}

// ReturnItem represents an item in a return
type ReturnItem struct {
	OrderItemID primitive.ObjectID `bson:"order_item_id" json:"order_item_id"`
	ProductID   primitive.ObjectID `bson:"product_id" json:"product_id"`
	ProductName string             `bson:"product_name" json:"product_name"`
	Size        string             `bson:"size" json:"size"`
	Quantity    int                `bson:"quantity" json:"quantity"`
	Price       float64            `bson:"price" json:"price"`
}

// ShippingMethod represents a shipping method
type ShippingMethod struct {
	ID                    primitive.ObjectID `bson:"_id" json:"id"`
	Name                  string             `bson:"name" json:"name"`
	Carrier               string             `bson:"carrier" json:"carrier"` // "fedex", "ups", "dhl", "custom"
	Type                  string             `bson:"type" json:"type"`       // "standard", "express", "overnight"
	BaseCost              float64            `bson:"base_cost" json:"base_cost"`
	Currency              string             `bson:"currency,omitempty" json:"currency,omitempty"`
	CostPerKg             *float64           `bson:"cost_per_kg,omitempty" json:"cost_per_kg,omitempty"`
	FreeShippingThreshold *float64           `bson:"free_shipping_threshold,omitempty" json:"free_shipping_threshold,omitempty"`
	EstimatedDays         int                `bson:"estimated_days" json:"estimated_days"`
	Zones                 []ShippingZone     `bson:"zones" json:"zones"`
	IsActive              bool               `bson:"is_active" json:"is_active"`
	CreatedAt             time.Time          `bson:"created_at" json:"created_at"`
}

// ShippingZone represents a shipping zone
type ShippingZone struct {
	Countries     []string `bson:"countries" json:"countries"`
	Cost          float64  `bson:"cost" json:"cost"`
	EstimatedDays int      `bson:"estimated_days" json:"estimated_days"`
}

// TaxRate represents a tax rate
type TaxRate struct {
	ID         primitive.ObjectID `bson:"_id" json:"id"`
	Country    string             `bson:"country" json:"country"`
	State      *string            `bson:"state,omitempty" json:"state,omitempty"`
	City       *string            `bson:"city,omitempty" json:"city,omitempty"`
	PostalCode *string            `bson:"postal_code,omitempty" json:"postal_code,omitempty"`
	Rate       float64            `bson:"rate" json:"rate"`         // Percentage (e.g., 8.0 for 8%)
	TaxType    string             `bson:"tax_type" json:"tax_type"` // "gst", "vat", "sales_tax"
	IsActive   bool               `bson:"is_active" json:"is_active"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
}

// PaymentTransaction represents a payment transaction
type PaymentTransaction struct {
	ID                   primitive.ObjectID     `bson:"_id" json:"id"`
	OrderID              primitive.ObjectID     `bson:"order_id" json:"order_id"`
	UserID               primitive.ObjectID     `bson:"user_id" json:"user_id"`
	Amount               float64                `bson:"amount" json:"amount"`
	Currency             string                 `bson:"currency" json:"currency"`
	PaymentMethod        string                 `bson:"payment_method" json:"payment_method"`
	Gateway              string                 `bson:"gateway" json:"gateway"` // "stripe", "razorpay", "payu"
	GatewayTransactionID string                 `bson:"gateway_transaction_id" json:"gateway_transaction_id"`
	Status               string                 `bson:"status" json:"status"` // "pending", "processing", "success", "failed", "refunded"
	FailureReason        *string                `bson:"failure_reason,omitempty" json:"failure_reason,omitempty"`
	Metadata             map[string]interface{} `bson:"metadata" json:"metadata"`
	CreatedAt            time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt            time.Time              `bson:"updated_at" json:"updated_at"`
}

// SupportTicket represents a customer support ticket
type SupportTicket struct {
	ID         primitive.ObjectID  `bson:"_id" json:"id"`
	UserID     primitive.ObjectID  `bson:"user_id" json:"user_id"`
	OrderID    *primitive.ObjectID `bson:"order_id,omitempty" json:"order_id,omitempty"`
	Subject    string              `bson:"subject" json:"subject"`
	Category   string              `bson:"category" json:"category"` // "order", "product", "payment", "shipping", "other"
	Priority   string              `bson:"priority" json:"priority"` // "low", "medium", "high", "urgent"
	Status     string              `bson:"status" json:"status"`     // "open", "in_progress", "resolved", "closed"
	Messages   []TicketMessage     `bson:"messages" json:"messages"`
	AssignedTo *primitive.ObjectID `bson:"assigned_to,omitempty" json:"assigned_to,omitempty"`
	CreatedAt  time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time           `bson:"updated_at" json:"updated_at"`
}

// TicketMessage represents a message in a support ticket
type TicketMessage struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	IsAdmin     bool               `bson:"is_admin" json:"is_admin"`
	Message     string             `bson:"message" json:"message"`
	Attachments []string           `bson:"attachments,omitempty" json:"attachments,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

// InspirationVideo represents an inspiration video for the homepage
type InspirationVideo struct {
	ID          primitive.ObjectID  `bson:"_id" json:"id"`
	Title       string             `bson:"title" json:"title"`
	VideoURL    string             `bson:"video_url" json:"video_url"`
	ThumbnailURL string             `bson:"thumbnail_url" json:"thumbnail_url"`
	Category    string             `bson:"category" json:"category"`
	Featured    bool               `bson:"featured" json:"featured"`
	UploadedBy  *primitive.ObjectID `bson:"uploaded_by,omitempty" json:"uploaded_by,omitempty"`
	UploadedByName string           `bson:"uploaded_by_name,omitempty" json:"uploaded_by_name,omitempty"`
	IsActive    bool               `bson:"is_active" json:"is_active"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}

// Address represents a saved user address
type Address struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	Name        string             `bson:"name" json:"name"`
	Type        string             `bson:"type" json:"type"` // "HOME", "OFFICE", "OTHER"
	AddressLine string             `bson:"address_line" json:"address_line"`
	AddressLine2 string            `bson:"address_line2,omitempty" json:"address_line2,omitempty"`
	City        string             `bson:"city" json:"city"`
	State       string             `bson:"state" json:"state"`
	Pincode     string             `bson:"pincode" json:"pincode"`
	Mobile      string             `bson:"mobile" json:"mobile"`
	Country     string             `bson:"country" json:"country"`
	IsDefault   bool               `bson:"is_default" json:"is_default"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}

// OrderTrackingUpdate represents a tracking update for an order
type OrderTrackingUpdate struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	OrderID     primitive.ObjectID `bson:"order_id" json:"order_id"`
	Status      string             `bson:"status" json:"status"`
	Date        time.Time          `bson:"date" json:"date"`
	Location    string             `bson:"location" json:"location"`
	Description string             `bson:"description" json:"description"`
	TrackingNumber *string          `bson:"tracking_number,omitempty" json:"tracking_number,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

// PasswordResetToken represents a password reset token
type PasswordResetToken struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Token     string             `bson:"token" json:"token"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
	Used      bool               `bson:"used" json:"used"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// EmailVerificationToken represents an email verification token
type EmailVerificationToken struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Token     string             `bson:"token" json:"token"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
	Used      bool               `bson:"used" json:"used"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

// NewsletterSubscription represents a newsletter subscription
type NewsletterSubscription struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	Email     string             `bson:"email" json:"email"`
	Name      *string            `bson:"name,omitempty" json:"name,omitempty"`
	Active    bool               `bson:"active" json:"active"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// VendorApplication is submitted by anyone who wants to sell on Rloko.
// Status flow: pending → approved | rejected
type VendorApplication struct {
	ID primitive.ObjectID `bson:"_id" json:"id"`

	// Business details
	BusinessName string `bson:"business_name" json:"business_name"`
	BusinessType string `bson:"business_type" json:"business_type"` // individual | partnership | company
	GSTNumber    string `bson:"gst_number,omitempty" json:"gst_number,omitempty"`
	Website      string `bson:"website,omitempty" json:"website,omitempty"`
	Instagram    string `bson:"instagram,omitempty" json:"instagram,omitempty"`

	// Contact
	ContactName string `bson:"contact_name" json:"contact_name"`
	Email       string `bson:"email" json:"email"`
	Phone       string `bson:"phone" json:"phone"`
	WhatsApp    string `bson:"whatsapp,omitempty" json:"whatsapp,omitempty"`

	// Location
	AddressLine1 string `bson:"address_line1" json:"address_line1"`
	AddressLine2 string `bson:"address_line2,omitempty" json:"address_line2,omitempty"`
	City         string `bson:"city" json:"city"`
	State        string `bson:"state" json:"state"`
	PINCode      string `bson:"pin_code" json:"pin_code"`
	Country      string `bson:"country" json:"country"`

	// Products
	Category            string `bson:"category" json:"category"`
	ProductDescription  string `bson:"product_description" json:"product_description"`
	PriceRange          string `bson:"price_range" json:"price_range"`   // "0-500" | "500-2000" | "2000-10000" | "10000+"
	EstimatedListings   string `bson:"estimated_listings" json:"estimated_listings"` // "1-10" | "10-50" | "50-100" | "100+"

	// Extra
	HowDidYouHear string `bson:"how_did_you_hear,omitempty" json:"how_did_you_hear,omitempty"`
	Message       string `bson:"message,omitempty" json:"message,omitempty"`

	// Review
	Status     string `bson:"status" json:"status"` // pending | approved | rejected
	AdminNotes string `bson:"admin_notes,omitempty" json:"admin_notes,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// RewardsTransaction records points earned or redeemed by a user
type RewardsTransaction struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	Type        string             `bson:"type" json:"type"`               // "earned" | "redeemed"
	Points      int64              `bson:"points" json:"points"`
	Reference   string             `bson:"reference" json:"reference"`     // order number or manual label
	Description string             `bson:"description" json:"description"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}
