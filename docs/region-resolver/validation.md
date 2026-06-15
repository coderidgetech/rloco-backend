# Validation — `GET /api/region/resolve`

Live run against local server (port 8081) backed by Docker MongoDB (`mongo:7`, localhost:28017).
Server booted clean; route registered as `GET /api/region/resolve --> (*RegionHandler).Resolve-fm`.

Date: 2026-06-10

## Curl matrix

| # | Request | HTTP | Response |
|---|---------|------|----------|
| 1 | `?pincode=560001` (6-digit → IN) | 200 | `{"market":"IN","currency":"INR","city":"","enabled":false,"comingSoonMessage":"We're launching in India soon. Stay tuned!"}` |
| 2 | `?pincode=94107` (5-digit → US) | 200 | `{"market":"US","currency":"USD","city":"","enabled":true,"comingSoonMessage":""}` |
| 3 | `?pincode=94107-1234` (zip+4 → US) | 200 | `{"market":"US","currency":"USD","city":"","enabled":true,"comingSoonMessage":""}` |
| 4 | `?country=IN` (country fallback, no pincode) | 200 | `{"market":"IN","currency":"INR",...,"enabled":false,...}` |
| 5 | `?pincode=560001&city=Bengaluru` (city echo) | 200 | `{"market":"IN",...,"city":"Bengaluru",...}` |
| 6 | `?pincode=abc` (invalid) | 400 | `{"error":"could not resolve a market from the provided pincode or country"}` |
| 7 | `` (no params) | 400 | `{"error":"could not resolve a market from the provided pincode or country"}` |

## Notes

- IN returns `enabled:false` + coming-soon message because the live site config gates the IN region (matches `GET /api/config` semantics and the order-flow gate). This is correct, not a bug — the client gates entry on this.
- US is enabled; pincode-first resolution with country-hint fallback both work.
- City echo passes through (rune-capped in handler).
- Invalid / empty input → 400 with a clear message.

Conclusion: endpoint verified end-to-end against a real Mongo-backed config. Matches the unit + HTTP-router test suite.
