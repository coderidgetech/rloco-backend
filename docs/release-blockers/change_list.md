# Change List — Release-Blocker Correctness Fixes (backend)

No new endpoints, no schema/migration. Modifications to existing order/payment flow.

## #1 — Paid-gated fulfillment  ✅ FIX
- **File:** `internal/services/order_service.go` → `FulfillOrder`.
- Current: blocks only `shipped/delivered/cancelled/returned`; an admin can buy a label for an unpaid card order.
- Change: after the status check, reject when the order is **non-COD and `PaymentStatus != "paid"`**: `order cannot be fulfilled until payment is completed`.
- Reuse existing `isCODPaymentMethod(order.PaymentMethod)`.
- Edge cases: COD is unpaid-until-delivery → must still be fulfillable (allowed). `refunding`/`refunded`/`failed` payment states are not `paid` → correctly blocked.

## #4 — Idempotent / atomic re-fulfill  ✅ FIX
- **File:** `internal/services/order_service.go` → `FulfillOrder`.
- Current: always calls `shippingService.FulfillShipment` (buys a label). If `SetTrackingNumber` or `UpdateStatus` failed on a prior attempt (tracking set, status not yet `shipped`), a retry **double-buys** the label.
- Change: if `order.TrackingNumber` is already set (non-nil, non-empty), **skip the label purchase**, reuse the existing tracking number/label URL, and just (re)advance status to `shipped` (which is idempotent — `UpdateStatus` no-ops if already shipped).
- Edge: a fully-fulfilled order (`status == "shipped"`) is already rejected by the existing status guard; this fix covers the partial-failure window only.

## #2 — Guest checkout idempotency  ✅ FIX
- **File:** `internal/handlers/order_handler.go` → `CreateGuest`.
- Current: no `Idempotency-Key` handling; double-submit creates duplicate COD orders and double-decrements stock. (Authed `Create` at `order_handler.go:323` already does this correctly via `h.idempotency`.)
- Change: mirror the authed flow — read `Idempotency-Key` header (≤256 chars), `LookupOrderID` → return existing on hit; `TryReserve` → on lease create order then `Commit`; on create failure `Release`; if not leased, look up the in-flight result.
- **Design choice (flagged):** the idempotency repo keys by `(userID, clientKey)`. Guests have no user id → scope by **`primitive.NilObjectID` + client key**. The client-generated key is the dedup token, so a fixed guest namespace is safe. Confirm this is acceptable.
- Edge: key is optional (absent → no dedup, same as authed). Concurrent double-submit → 2nd request sees the reservation and returns the same order.

## #3 — Stock release on payment failure  ⚠️ FLAGGED — needs a decision, NOT implementing blind
- **File:** `internal/services/payment_service.go` → `HandleWebhook`, `payment_intent.payment_failed` (~line 325). Currently marks the transaction `failed` only; does not touch the order or release stock.
- **Already covered:** `CancelOrder` (`order_service.go:603`) restores stock for `pending`/`processing` orders, so the "customer cancels" path is handled.
- **Why this is risky to auto-fix:** a Stripe PaymentIntent can emit `payment_failed` for one attempt and then **succeed on retry (same PI)**. If we release stock on the failed attempt and `succeeded` fires later, the order proceeds to `paid` with no reservation → **oversell**. Not releasing leaves stock stuck for truly-abandoned carts.
- **Options:**
  - (a) Treat `payment_failed` as terminal: release stock + set order `cancelled`/`payment_status=failed`. Simple, but wrong when the customer retries the same PI.
  - (b) **Leave create-time reservation, add a TTL sweeper** (cron) that releases stock for `pending`+unpaid orders older than N minutes. Correct, but needs cron infra (none exists — explicitly out of scope until approved).
  - (c) Release on `failed`, re-reserve on a later `succeeded`. Complex; re-reserve can fail if sold out (→ paid order with no stock, worse).
- **Recommendation:** defer #3 from this batch; do **(b)** as a separate, approved follow-up. Implementing (a) now would trade one bug for a worse one.

## Files modified this batch
- `internal/services/order_service.go` (FulfillOrder: #1 + #4)
- `internal/handlers/order_handler.go` (CreateGuest: #2)

## Ambiguities flagged
1. Guest idempotency namespace = `primitive.NilObjectID` (confirm).
2. #3 payment-failure stock-release strategy — needs product decision (recommend deferring to a TTL sweeper, option b).

## Review-driven revisions (Go review, Step 3)
- **#4 made concurrency-safe (was sequential-only).** Added `OrderRepository.ClaimForFulfillment` / `ReleaseFulfillmentClaim` (atomic `UpdateOne` on `tracking_number` empty→sentinel `__FULFILLING__`). `FulfillOrder` now: claims exclusively before buying a label (concurrent calls / double-clicks can't double-buy); on label-buy failure releases the claim for retry; on label-bought-but-persist-failed it keeps the sentinel (no double-buy) and logs `[FULFILL][CRITICAL]` with tracking+label for manual reconciliation; a sentinel seen on entry returns "fulfillment in progress" rather than shipping a bogus number.
- **#2 guest namespace changed from `NilObjectID` → per-guest hash of email** (`guestIdempotencyNamespace`) to prevent a cross-guest key collision leaking another guest's order (PII). Body is now bound *before* reserving so the email is known. Unit-tested for case/whitespace-insensitivity, no collision, non-Nil.
- **#1 helper normalizes `paymentStatus`/`status`** (trim+lower) for symmetry with the COD method check.
- Tests added: `order_fulfillment_test.go` (paid-gate matrix), `order_guest_idem_test.go` (namespace isolation).
