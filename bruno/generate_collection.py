#!/usr/bin/env python3
"""
Generate Bruno .bru files from routes in cmd/server/main.go + handler JSON shapes.
Run: python3 generate_collection.py  (from bruno/)
"""
from __future__ import annotations

import json
import shutil
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parent
COLLECTION = ROOT / "Rloco API"

# body: None = no request body | dict = JSON | "multipart" | "webhook"

# --- Example JSON bodies (field names match Go json tags) ---

B_REGISTER = {"email": "user@example.com", "password": "secret12", "name": "Jane Doe"}
B_REG_OTP_SEND = {"phone": "+15551234567"}
B_REG_OTP_COMPLETE = {
    "phone": "+15551234567",
    "code": "123456",
    "email": "user@example.com",
    "password": "secret12",
    "name": "Jane Doe",
}
B_LOGIN_OTP_COMPLETE = {"phone": "+15551234567", "code": "123456"}
B_LOGIN = {"email": "user@example.com", "password": "yourpassword"}
B_GOOGLE = {"id_token": "PASTE_GOOGLE_ID_TOKEN_JWT"}
B_RESET_PW = {"token": "token-from-email", "new_password": "newsecret12"}
B_VERIFY_EMAIL = {"token": "verification-token"}
B_UPDATE_PROFILE = {
    "name": "Jane Doe",
    "phone": "+15551234567",
    "email": "jane@example.com",
    "birthday": "1990-01-15",
}
B_CHANGE_PW = {"current_password": "oldpass", "new_password": "newsecret12"}
B_NEWSLETTER_SUB = {"email": "news@example.com", "name": "Subscriber"}
B_NEWSLETTER_UNSUB = {"email": "news@example.com"}

B_CART_ADD = {
    "product_id": "{{productId}}",
    "product_name": "Example product",
    "image": "https://example.com/image.jpg",
    "price": 99.99,
    "size": "M",
    "quantity": 1,
    "is_gift": False,
}
B_CART_UPDATE = {"quantity": 2, "size": "M"}
B_CART_GIFT = {
    "product_id": "{{productId}}",
    "size": "M",
    "gift_wrap_color": "gold",
    "gift_message": "Happy birthday",
}
B_CART_REMOVE = {"size": "M"}

B_WISHLIST_ADD = {"product_id": "{{productId}}"}

B_ORDER_CREATE = {
    "items": [
        {
            "product_id": "{{productId}}",
            "product_name": "Dress",
            "image": "",
            "price": 99.99,
            "size": "M",
            "quantity": 1,
            "is_gift": False,
        }
    ],
    "shipping_info": {
        "first_name": "Jane",
        "last_name": "Doe",
        "email": "jane@example.com",
        "phone": "+15551234567",
        "address": "123 Fashion Ave",
        "city": "New York",
        "state": "NY",
        "zip_code": "10001",
        "country": "US",
    },
    "payment_info": {},
    "payment_method": "stripe",
    "gift_packing_charge": 0,
}
B_ORDER_CANCEL = {"reason": "Changed my mind"}
B_ORDER_STATUS = {"status": "shipped"}
B_RETURN_CREATE = {
    "items": [
        {
            "order_item_id": "{{orderItemId}}",
            "product_id": "{{productId}}",
            "product_name": "Dress",
            "size": "M",
            "quantity": 1,
            "price": 99.99,
        }
    ],
    "reason": "defective",
    "description": "Optional longer description",
}
B_REVIEW_CREATE = {
    "rating": 5,
    "title": "Love it",
    "comment": "Great quality and fit.",
    "images": [],
    "verified": True,
}
B_REVIEW_UPDATE = {
    "title": "Updated title",
    "comment": "Still happy after a month.",
    "images": [],
}
B_SUPPORT_CREATE = {
    "subject": "Order question",
    "category": "orders",
    "priority": "normal",
    "message": "Where is my package?",
    "order_id": "{{orderId}}",
}
B_SUPPORT_MESSAGE = {"message": "Additional details here.", "attachments": []}
B_SUPPORT_STATUS = {"status": "resolved"}
B_SUPPORT_ASSIGN = {"assigned_to": "{{customerId}}"}

B_SHIP_CALC = {
    "country": "US",
    "state": "NY",
    "city": "New York",
    "address": "123 Main St",
    "postal_code": "10001",
    "first_name": "Jane",
    "last_name": "Doe",
    "email": "jane@example.com",
    "phone": "+15551234567",
    "subtotal": 150.0,
    "weight": 1.2,
}
B_TAX_CALC = {
    "country": "US",
    "state": "CA",
    "city": "Los Angeles",
    "postal_code": "90001",
    "subtotal": 100.0,
}

B_PAY_INTENT = {
    "order_id": "{{orderId}}",
    "amount": 99.99,
    "currency": "usd",
    "gateway": "stripe",
    "payment_method": "card",
}
B_PAY_PROCESS = {
    "payment_intent_id": "pi_xxx",
    "payment_method_id": "pm_xxx",
    "gateway": "stripe",
}
B_PAY_REFUND = {"amount": 25.5}

# Stripe verifies HMAC of raw body; for manual Bruno tests use a real payload + X-Stripe-Signature from Stripe CLI.
B_STRIPE_WEBHOOK = {
    "id": "evt_example",
    "object": "event",
    "type": "payment_intent.succeeded",
    "data": {"object": {"id": "pi_example", "object": "payment_intent", "status": "succeeded"}},
}

B_PROMO_VALIDATE = {"code": "SAVE10", "subtotal": 199.99}

B_ADDRESS = {
    "name": "Jane Doe",
    "type": "HOME",
    "address_line": "123 Main Street",
    "address_line2": "Apt 4",
    "city": "New York",
    "state": "NY",
    "pincode": "10001",
    "mobile": "+15551234567",
    "country": "US",
    "is_default": True,
}

B_PRODUCT = {
    "name": "Silk Dress",
    "sku": "DRS-001",
    "price": 249.99,
    "images": ["https://example.com/1.jpg"],
    "category": "women",
    "subcategory": "dresses",
    "gender": "women",
    "colors": ["Black", "Navy"],
    "sizes": ["S", "M", "L"],
    "description": "Elegant silk dress.",
    "details": ["Lined", "Dry clean only"],
    "material": "100% silk",
    "care": "Dry clean",
    "featured": False,
    "new_arrival": True,
    "on_sale": False,
    "is_gift": True,
    "rating": 0,
    "reviews": 0,
    "stock": {"S": 3, "M": 5, "L": 2},
    "available_markets": ["US", "IN"],
}

# Product PUT accepts partial fields (map[string]interface{}), not full Product.
B_PRODUCT_UPDATE = {"name": "Updated name", "price": 199.99, "featured": True, "stock": {"S": 2, "M": 4}}

B_CATEGORY = {
    "name": "Dresses",
    "slug": "dresses",
    "gender": "women",
    "subcategories": ["Mini", "Midi"],
    "image": "https://example.com/cat.jpg",
    "order": 1,
}

B_PROMOTION = {
    "name": "Summer Sale",
    "code": "SUMMER20",
    "type": "percentage",
    "value": 20,
    "min_purchase": 100,
    "max_discount": 50,
    "start_date": "2026-06-01T00:00:00Z",
    "end_date": "2026-08-31T23:59:59Z",
    "usage_limit": 500,
    "is_active": True,
}

B_VENDOR_CREATE = {
    "name": "Acme Fashion",
    "email": "vendor@acme.com",
    "logo": "https://example.com/logo.png",
    "subscription_plan": "standard",
    "permissions": {"products": True},
    "status": "pending",
    "initial_password": "TempPass12",
}
B_VENDOR_UPDATE = {
    "name": "Acme Fashion",
    "email": "vendor@acme.com",
    "logo": "",
    "subscription_plan": "standard",
    "permissions": {},
    "status": "active",
}
B_VENDOR_PERMS = {"permissions": {"can_manage_inventory": True}}

B_VENDOR_ME = {
    "name": "My Boutique",
    "email": "me@boutique.com",
    "logo": "https://example.com/vlogo.png",
    "preferences": {"theme": "dark"},
}

B_VIDEO = {
    "title": "Spring lookbook",
    "video_url": "https://cdn.example.com/video.mp4",
    "thumbnail_url": "https://cdn.example.com/thumb.jpg",
    "category": "lookbook",
    "featured": False,
    "is_active": True,
}

B_USER_ADMIN = {
    "email": "customer@example.com",
    "name": "Customer Name",
    "role": "customer",
    "avatar": "",
    "active": True,
    "email_verified": True,
}

B_ADMIN_PATCH = {"general": {"siteName": "Rloco Store", "tagline": "Luxury fashion"}}
B_ADMIN_CONFIG = {
    "general": {"siteName": "Rloco", "currency": "usd"},
    "design": {"colors": {"primary": "#B4770E"}},
}

B_REVIEW_STATUS = {"status": "approved"}
B_RETURN_STATUS = {"status": "approved"}
B_RETURN_REJECT = {"reason": "Outside return window"}
B_RETURN_REFUND = {"refund_method": "original_payment"}

B_SHIPPING_METHOD = {
    "name": "Standard US",
    "carrier": "custom",
    "type": "standard",
    "base_cost": 5.99,
    "currency": "usd",
    "estimated_days": 5,
    "zones": [{"countries": ["US"], "cost": 5.99, "estimated_days": 5}],
    "is_active": True,
}

B_TAX_RATE = {
    "country": "US",
    "state": "CA",
    "rate": 8.25,
    "tax_type": "sales_tax",
    "is_active": True,
}

ROUTES: list[tuple[str, str, str, str, str, Any]] = [
    ("Health", "Health check", "GET", "/health", "none", None),
    ("Auth", "Register", "POST", "/api/auth/register", "none", B_REGISTER),
    ("Auth", "Register OTP send", "POST", "/api/auth/register-otp/send", "none", B_REG_OTP_SEND),
    ("Auth", "Register OTP complete", "POST", "/api/auth/register-otp/complete", "none", B_REG_OTP_COMPLETE),
    ("Auth", "Login OTP send", "POST", "/api/auth/login-otp/send", "none", B_REG_OTP_SEND),
    ("Auth", "Login OTP complete", "POST", "/api/auth/login-otp/complete", "none", B_LOGIN_OTP_COMPLETE),
    ("Auth", "Login", "POST", "/api/auth/login", "none", B_LOGIN),
    ("Auth", "Google sign-in", "POST", "/api/auth/google", "none", B_GOOGLE),
    ("Auth", "Logout", "POST", "/api/auth/logout", "none", None),
    ("Auth", "Get me", "GET", "/api/auth/me", "bearer", None),
    ("Auth", "Delete account", "DELETE", "/api/auth/me", "bearer", None),
    ("Auth", "Refresh token", "POST", "/api/auth/refresh", "none", None),
    ("Auth", "Forgot password", "POST", "/api/auth/forgot-password", "none", {"email": "user@example.com"}),
    ("Auth", "Reset password", "POST", "/api/auth/reset-password", "none", B_RESET_PW),
    ("Auth", "Verify email", "POST", "/api/auth/verify-email", "none", B_VERIFY_EMAIL),
    ("Auth", "Resend verification", "POST", "/api/auth/resend-verification", "none", {"email": "user@example.com"}),
    ("Auth", "Update profile", "PUT", "/api/auth/profile", "bearer", B_UPDATE_PROFILE),
    ("Auth", "Change password", "PUT", "/api/auth/password", "bearer", B_CHANGE_PW),
    ("Auth", "Deactivate account", "POST", "/api/auth/deactivate", "bearer", None),
    ("Products", "List products", "GET", "/api/products", "none", None),
    ("Products", "Featured products", "GET", "/api/products/featured", "none", None),
    ("Products", "New arrivals", "GET", "/api/products/new-arrivals", "none", None),
    ("Products", "On sale", "GET", "/api/products/on-sale", "none", None),
    ("Products", "Get product by id", "GET", "/api/products/{{productId}}", "none", None),
    ("Products", "Create product", "POST", "/api/products", "bearer", B_PRODUCT),
    ("Products", "Update product", "PUT", "/api/products/{{productId}}", "bearer", B_PRODUCT_UPDATE),
    ("Products", "Delete product", "DELETE", "/api/products/{{productId}}", "bearer", None),
    ("Products", "Upload product images", "POST", "/api/products/{{productId}}/images", "bearer", "multipart"),
    ("Categories", "List categories", "GET", "/api/categories", "none", None),
    ("Categories", "Get category", "GET", "/api/categories/{{categoryId}}", "none", None),
    ("Categories", "Create category", "POST", "/api/categories", "bearer", B_CATEGORY),
    ("Categories", "Update category", "PUT", "/api/categories/{{categoryId}}", "bearer", B_CATEGORY),
    ("Categories", "Delete category", "DELETE", "/api/categories/{{categoryId}}", "bearer", None),
    ("Cart", "Get cart", "GET", "/api/cart", "bearer", None),
    ("Cart", "Add cart item", "POST", "/api/cart/items", "bearer", B_CART_ADD),
    ("Cart", "Update cart item", "PUT", "/api/cart/items/{{cartItemId}}", "bearer", B_CART_UPDATE),
    ("Cart", "Update gift options", "PUT", "/api/cart/gift-options", "bearer", B_CART_GIFT),
    ("Cart", "Remove cart item", "DELETE", "/api/cart/items/{{cartItemId}}", "bearer", B_CART_REMOVE),
    ("Cart", "Clear cart", "DELETE", "/api/cart", "bearer", None),
    ("Wishlist", "Get wishlist", "GET", "/api/wishlist", "bearer", None),
    ("Wishlist", "Add wishlist item", "POST", "/api/wishlist/items", "bearer", B_WISHLIST_ADD),
    ("Wishlist", "Remove wishlist item", "DELETE", "/api/wishlist/items/{{wishlistItemId}}", "bearer", None),
    ("Newsletter", "Subscribe", "POST", "/api/newsletter/subscribe", "none", B_NEWSLETTER_SUB),
    ("Newsletter", "Unsubscribe", "POST", "/api/newsletter/unsubscribe", "none", B_NEWSLETTER_UNSUB),
    ("Orders", "List orders", "GET", "/api/orders", "bearer", None),
    ("Orders", "Get order", "GET", "/api/orders/{{orderId}}", "bearer", None),
    ("Orders", "Create order", "POST", "/api/orders", "bearer", B_ORDER_CREATE),
    ("Orders", "Track by order number", "GET", "/api/orders/tracking/{{orderNumber}}", "bearer", None),
    ("Orders", "Get order tracking", "GET", "/api/orders/{{orderId}}/tracking", "bearer", None),
    ("Orders", "Cancel order", "POST", "/api/orders/{{orderId}}/cancel", "bearer", B_ORDER_CANCEL),
    ("Orders", "Create return", "POST", "/api/orders/{{orderId}}/return", "bearer", B_RETURN_CREATE),
    ("Orders", "Update order status (admin)", "PUT", "/api/orders/{{orderId}}/status", "bearer", B_ORDER_STATUS),
    ("Returns", "List my returns", "GET", "/api/returns", "bearer", None),
    ("Returns", "Get return", "GET", "/api/returns/{{returnId}}", "bearer", None),
    ("Reviews", "My reviews", "GET", "/api/reviews/me", "bearer", None),
    ("Reviews", "List product reviews", "GET", "/api/products/{{productId}}/reviews", "none", None),
    ("Reviews", "Create review", "POST", "/api/products/{{productId}}/reviews", "bearer", B_REVIEW_CREATE),
    ("Reviews", "Update review", "PUT", "/api/products/{{productId}}/reviews/{{reviewId}}", "bearer", B_REVIEW_UPDATE),
    ("Reviews", "Delete review", "DELETE", "/api/products/{{productId}}/reviews/{{reviewId}}", "bearer", None),
    ("Reviews", "Mark review helpful", "POST", "/api/products/{{productId}}/reviews/{{reviewId}}/helpful", "none", None),
    ("Support", "List tickets", "GET", "/api/support/tickets", "bearer", None),
    ("Support", "Create ticket", "POST", "/api/support/tickets", "bearer", B_SUPPORT_CREATE),
    ("Support", "Get ticket", "GET", "/api/support/tickets/{{ticketId}}", "bearer", None),
    ("Support", "Add ticket message", "POST", "/api/support/tickets/{{ticketId}}/messages", "bearer", B_SUPPORT_MESSAGE),
    ("Shipping", "Calculate shipping", "POST", "/api/shipping/calculate", "none", B_SHIP_CALC),
    ("Shipping", "List shipping methods", "GET", "/api/shipping/methods", "none", None),
    ("Tax", "Calculate tax", "POST", "/api/tax/calculate", "none", B_TAX_CALC),
    ("Tax", "List tax rates", "GET", "/api/tax/rates", "none", None),
    ("Payments", "Create payment intent", "POST", "/api/payments/intent", "bearer", B_PAY_INTENT),
    ("Payments", "Process payment", "POST", "/api/payments/process", "bearer", B_PAY_PROCESS),
    ("Payments", "Get transaction", "GET", "/api/payments/transactions/{{transactionId}}", "bearer", None),
    ("Payments", "Refund (admin)", "POST", "/api/payments/refund/{{transactionId}}", "bearer", B_PAY_REFUND),
    ("Webhooks", "Payment webhook", "POST", "/api/webhooks/{{gateway}}", "none", B_STRIPE_WEBHOOK),
    ("Promotions", "List promotions", "GET", "/api/promotions", "none", None),
    ("Promotions", "Validate promotion", "POST", "/api/promotions/validate", "none", B_PROMO_VALIDATE),
    ("Videos", "List videos", "GET", "/api/videos", "none", None),
    ("Videos", "Featured videos", "GET", "/api/videos/featured", "none", None),
    ("Videos", "Get video", "GET", "/api/videos/{{videoId}}", "none", None),
    ("Addresses", "List addresses", "GET", "/api/addresses", "bearer", None),
    ("Addresses", "Create address", "POST", "/api/addresses", "bearer", B_ADDRESS),
    ("Addresses", "Get address", "GET", "/api/addresses/{{addressId}}", "bearer", None),
    ("Addresses", "Update address", "PUT", "/api/addresses/{{addressId}}", "bearer", B_ADDRESS),
    ("Addresses", "Delete address", "DELETE", "/api/addresses/{{addressId}}", "bearer", None),
    ("Addresses", "Set default address", "PUT", "/api/addresses/{{addressId}}/default", "bearer", None),
    ("Config", "Get public config", "GET", "/api/config", "none", None),
    ("Config", "Get public content", "GET", "/api/content", "none", None),
    ("Upload", "Upload file", "POST", "/api/upload", "bearer", "multipart"),
    ("Upload", "Delete upload", "DELETE", "/api/upload/{{filename}}", "bearer", None),
    ("Vendor", "Get vendor me", "GET", "/api/vendor/me", "bearer", None),
    ("Vendor", "Update vendor me", "PUT", "/api/vendor/me", "bearer", B_VENDOR_ME),
    ("Admin", "Dashboard stats", "GET", "/api/admin/dashboard/stats", "bearer", None),
    ("Admin", "Dashboard sales", "GET", "/api/admin/dashboard/sales", "bearer", None),
    ("Admin", "Dashboard orders", "GET", "/api/admin/dashboard/orders", "bearer", None),
    ("Admin", "Dashboard products", "GET", "/api/admin/dashboard/products", "bearer", None),
    ("Admin", "List customers", "GET", "/api/admin/customers", "bearer", None),
    ("Admin", "Get customer", "GET", "/api/admin/customers/{{customerId}}", "bearer", None),
    ("Admin", "Update customer", "PUT", "/api/admin/customers/{{customerId}}", "bearer", B_USER_ADMIN),
    ("Admin", "List vendors", "GET", "/api/admin/vendors", "bearer", None),
    ("Admin", "Get vendor", "GET", "/api/admin/vendors/{{vendorId}}", "bearer", None),
    ("Admin", "Create vendor", "POST", "/api/admin/vendors", "bearer", B_VENDOR_CREATE),
    ("Admin", "Update vendor", "PUT", "/api/admin/vendors/{{vendorId}}", "bearer", B_VENDOR_UPDATE),
    ("Admin", "Delete vendor", "DELETE", "/api/admin/vendors/{{vendorId}}", "bearer", None),
    ("Admin", "Update vendor permissions", "PUT", "/api/admin/vendors/{{vendorId}}/permissions", "bearer", B_VENDOR_PERMS),
    ("Admin", "List admin promotions", "GET", "/api/admin/promotions", "bearer", None),
    ("Admin", "Create promotion", "POST", "/api/admin/promotions", "bearer", B_PROMOTION),
    ("Admin", "Update promotion", "PUT", "/api/admin/promotions/{{promotionId}}", "bearer", B_PROMOTION),
    ("Admin", "Delete promotion", "DELETE", "/api/admin/promotions/{{promotionId}}", "bearer", None),
    ("Admin", "Analytics revenue", "GET", "/api/admin/analytics/revenue", "bearer", None),
    ("Admin", "Analytics orders", "GET", "/api/admin/analytics/orders", "bearer", None),
    ("Admin", "Analytics products", "GET", "/api/admin/analytics/products", "bearer", None),
    ("Admin", "Analytics customers", "GET", "/api/admin/analytics/customers", "bearer", None),
    ("Admin", "Analytics traffic", "GET", "/api/admin/analytics/traffic", "bearer", None),
    ("Admin", "Get content (admin)", "GET", "/api/admin/content", "bearer", None),
    ("Admin", "Update content", "PUT", "/api/admin/content", "bearer", B_ADMIN_PATCH),
    ("Admin", "Get settings", "GET", "/api/admin/settings", "bearer", None),
    ("Admin", "Update settings", "PUT", "/api/admin/settings", "bearer", B_ADMIN_PATCH),
    ("Admin", "Get configuration", "GET", "/api/admin/configuration", "bearer", None),
    ("Admin", "Update configuration", "PUT", "/api/admin/configuration", "bearer", B_ADMIN_CONFIG),
    ("Admin", "List reviews (admin)", "GET", "/api/admin/reviews", "bearer", None),
    ("Admin", "Update review status", "PUT", "/api/admin/reviews/{{reviewId}}/status", "bearer", B_REVIEW_STATUS),
    ("Admin", "List all returns", "GET", "/api/admin/returns", "bearer", None),
    ("Admin", "Update return status", "PUT", "/api/admin/returns/{{returnId}}/status", "bearer", B_RETURN_STATUS),
    ("Admin", "Approve return", "PUT", "/api/admin/returns/{{returnId}}/approve", "bearer", None),
    ("Admin", "Reject return", "PUT", "/api/admin/returns/{{returnId}}/reject", "bearer", B_RETURN_REJECT),
    ("Admin", "Process return refund", "PUT", "/api/admin/returns/{{returnId}}/process-refund", "bearer", B_RETURN_REFUND),
    ("Admin", "List shipping methods", "GET", "/api/admin/shipping/methods", "bearer", None),
    ("Admin", "Create shipping method", "POST", "/api/admin/shipping/methods", "bearer", B_SHIPPING_METHOD),
    ("Admin", "Update shipping method", "PUT", "/api/admin/shipping/methods/{{shippingMethodId}}", "bearer", B_SHIPPING_METHOD),
    ("Admin", "Delete shipping method", "DELETE", "/api/admin/shipping/methods/{{shippingMethodId}}", "bearer", None),
    ("Admin", "List tax rates", "GET", "/api/admin/tax/rates", "bearer", None),
    ("Admin", "Create tax rate", "POST", "/api/admin/tax/rates", "bearer", B_TAX_RATE),
    ("Admin", "Update tax rate", "PUT", "/api/admin/tax/rates/{{taxRateId}}", "bearer", B_TAX_RATE),
    ("Admin", "Delete tax rate", "DELETE", "/api/admin/tax/rates/{{taxRateId}}", "bearer", None),
    ("Admin", "Inventory low stock", "GET", "/api/admin/inventory/low-stock", "bearer", None),
    ("Admin", "Inventory alerts", "GET", "/api/admin/inventory/alerts", "bearer", None),
    ("Admin", "List support tickets (admin)", "GET", "/api/admin/support/tickets", "bearer", None),
    ("Admin", "Update ticket status", "PUT", "/api/admin/support/tickets/{{ticketId}}/status", "bearer", B_SUPPORT_STATUS),
    ("Admin", "Assign ticket", "PUT", "/api/admin/support/tickets/{{ticketId}}/assign", "bearer", B_SUPPORT_ASSIGN),
    ("Admin", "Wishlist product analytics", "GET", "/api/admin/wishlist/analytics", "bearer", None),
    ("Admin", "Wishlist user analytics", "GET", "/api/admin/wishlist/analytics/users", "bearer", None),
    ("Admin", "Create video", "POST", "/api/admin/videos", "bearer", B_VIDEO),
    ("Admin", "Update video", "PUT", "/api/admin/videos/{{videoId}}", "bearer", B_VIDEO),
    ("Admin", "Delete video", "DELETE", "/api/admin/videos/{{videoId}}", "bearer", None),
]


def slug_filename(name: str) -> str:
    s = "".join(c if c.isalnum() or c in " -" else "" for c in name)
    return s.replace(" ", "-").lower() + ".bru"


def emit_bru(name: str, method: str, path: str, auth: str, body: Any, seq: int) -> str:
    is_stripe_webhook = path == "/api/webhooks/{{gateway}}" and method == "POST"
    url = "{{baseUrl}}" + path
    m = method.lower()
    lines: list[str] = [
        "meta {",
        f"  name: {name}",
        "  type: http",
        f"  seq: {seq}",
        "}",
        "",
        f"{m} {{",
        f"  url: {url}",
    ]

    wants_json_headers = False

    if body == "multipart":
        lines.append("  body: multipartForm")
    elif method in ("POST", "PUT", "PATCH") and body is not None and isinstance(body, dict):
        lines.append("  body: json")
        wants_json_headers = True
    elif method == "DELETE" and isinstance(body, dict):
        lines.append("  body: json")
        wants_json_headers = True

    lines.append("  auth: none")
    lines.append("}")
    lines.append("")

    hdr: list[str] = []
    if auth == "bearer":
        hdr.append("  Authorization: Bearer {{token}}")
    if wants_json_headers:
        hdr.append("  Content-Type: application/json")
    if is_stripe_webhook:
        hdr.append("  X-Stripe-Signature: paste_signature_from_stripe_cli")
    if hdr:
        lines.append("headers {")
        lines.extend(hdr)
        lines.append("}")
        lines.append("")

    if body == "multipart":
        lines.append("body:multipart-form {")
        lines.append("  file: @file()")
        lines.append("}")
        lines.append("")
    elif isinstance(body, dict):
        lines.append("body:json {")
        lines.append("  " + json.dumps(body, separators=(",", ":")))
        lines.append("}")
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


def main() -> None:
    if COLLECTION.exists():
        shutil.rmtree(COLLECTION)
    COLLECTION.mkdir(parents=True)

    folders: list[str] = []
    seen: set[str] = set()
    for folder, *_ in ROUTES:
        if folder not in seen:
            seen.add(folder)
            folders.append(folder)

    for i, folder in enumerate(folders, start=1):
        (COLLECTION / folder).mkdir()
        (COLLECTION / folder / "folder.bru").write_text(
            f"meta {{\n  name: {folder}\n  seq: {i}\n}}\n",
            encoding="utf-8",
        )

    seq_by_folder: dict[str, int] = {}
    for folder, name, method, path, auth, body in ROUTES:
        seq_by_folder[folder] = seq_by_folder.get(folder, 0) + 1
        content = emit_bru(name, method, path, auth, body, seq_by_folder[folder])
        (COLLECTION / folder / slug_filename(name)).write_text(content, encoding="utf-8")

    (COLLECTION / "bruno.json").write_text(
        """{
  "version": "1",
  "name": "Rloco API",
  "type": "collection",
  "ignore": ["node_modules", ".git"]
}
""",
        encoding="utf-8",
    )
    env_dir = COLLECTION / "environments"
    env_dir.mkdir(exist_ok=True)
    (env_dir / "local.bru").write_text(
        """vars {
  baseUrl: http://localhost:8080
  token: 
  productId: 
  orderId: 
  orderItemId: 
  categoryId: 
  cartItemId: 
  wishlistItemId: 
  orderNumber: 
  returnId: 
  reviewId: 
  ticketId: 
  transactionId: 
  gateway: stripe
  addressId: 
  filename: 
  customerId: 
  vendorId: 
  promotionId: 
  shippingMethodId: 
  taxRateId: 
  videoId: 
}
""",
        encoding="utf-8",
    )

    print(f"Wrote {len(ROUTES)} requests under {COLLECTION}")


if __name__ == "__main__":
    main()
