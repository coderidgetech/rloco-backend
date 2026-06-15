# Change List — Phase 1: Region Resolver (`/api/region/resolve`)

## Goal
Make pincode/ZIP the source of truth for the storefront market. Add a public endpoint
that resolves a pincode to its market (`IN`/`US`), currency, and live region
availability, so clients can derive `market` instead of using a manual toggle.

## Scope (this change)
- **1.1** Service-layer pincode → market/currency resolver.
- **1.2** Public `GET /api/region/resolve` endpoint.

Out of scope: client changes (mobile/web), product-list filtering (already honors
`?market=`), pincode-level serviceability (Phase 2).

## API Changes

### New endpoint: `GET /api/region/resolve`
- **Auth:** none (public, same group as `GET /api/config`).
- **Query params:**
  - `pincode` (required) — 5/5+4 digits ⇒ US, 6 digits ⇒ IN.
  - `country` (optional) — hint (`US`/`IN`/`United States`/`India`) used only when a
    pincode is ambiguous/empty.
  - `city` (optional) — echoed back so clients can round-trip a user-typed city hint
    (city is **not** derivable from a pincode in this codebase).
- **200 response:**
  ```json
  {
    "market": "IN",
    "currency": "INR",
    "city": "Bangalore",
    "enabled": false,
    "comingSoonMessage": "We're launching in India soon. Stay tuned!"
  }
  ```
- **400 response:** `{ "error": "..." }` when pincode is missing AND no usable country
  hint, or when the value matches no known market format.

### Resolution rules (mirror client regex in `mobile-app .../guest_delivery_cubit.dart`)
| Input | Market |
|-------|--------|
| `^\d{6}$` | `IN` |
| `^\d{5}(-\d{4})?$` | `US` |
| else, fall back to `country` hint via existing `marketFromCountry` | IN/US |
| no match | 400 error |

Currency: `IN ⇒ INR`, `US ⇒ USD`.

`enabled` + `comingSoonMessage` come from the live `general.regions` site config
(same source the order flow uses), backfilled by `ensureRegionDefaults`. Fails **open**
(`enabled:true`, empty message) when regions config is absent — matching order-flow
behavior.

## Files

### Create
- `internal/services/region_service.go`
  - `RegionService` interface + `regionService` impl (stateless).
  - `MarketFromPincode(pincode, countryHint string) (market string, ok bool)`.
  - `CurrencyForMarket(market string) string`.
- `internal/handlers/region_handler.go`
  - `RegionHandler` holding `configService` + `RegionService`.
  - `Resolve(c *gin.Context)`.
  - Package-level `regionStatusFromConfig(cfg map[string]interface{}, market string) (bool, string)`
    helper (reads `general.regions[market]`), reused via existing
    `getDefaultConfig()` / `ensureRegionDefaults()`.

### Modify
- `cmd/server/main.go`
  - Instantiate `regionService := services.NewRegionService()`.
  - Instantiate `regionHandler := handlers.NewRegionHandler(configService, regionService)`.
  - Register `api.GET("/region/resolve", regionHandler.Resolve)` next to
    `api.GET("/config", ...)` (line ~431).

## Edge cases / validation
- Empty/whitespace pincode + no country ⇒ 400.
- Pincode with spaces ⇒ stripped before matching (client does the same).
- Unknown country hint ⇒ ignored; if no pincode match ⇒ 400.
- Disabled region (e.g. `IN`) ⇒ 200 with `enabled:false` + message (NOT an error;
  client decides how to gate). Mirrors `GET /api/config` semantics.
- Missing regions config ⇒ fail open (`enabled:true`).

## Conventions matched
- Thin handler, logic in service layer (pincode parsing) + reused config helpers.
- Standard error format `{"error": "..."}` and `c.JSON` responses.
- Reuses `marketFromCountry`, `getDefaultConfig`, `ensureRegionDefaults` already in the
  handlers package — no duplication, no refactor of the order flow.

## Open question (flagged, non-blocking)
- `city` is echoed, not derived. If a server-side pincode→city lookup is wanted, that's
  a separate data task (Phase 2 zone data). Proceeding with echo for now.
