#!/usr/bin/env bash
# Quick API check – run with backend and MongoDB up.
# Usage: ./scripts/check-api.sh [BASE_URL]
# Example: ./scripts/check-api.sh http://localhost:8080

set -e
BASE="${1:-http://localhost:8080}"
API="${BASE}/api"

echo "Checking API at $BASE"
echo ""

# Health (no /api prefix)
check() {
  local method="$1"
  local path="$2"
  local desc="$3"
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$path" 2>/dev/null || echo "000")
  if [ "$code" = "200" ] || [ "$code" = "201" ] || [ "$code" = "204" ]; then
    echo "  OK   $method $path -> $code"
  else
    [ "$code" = "000" ] && code="connection refused"
    echo "  FAIL $method $path -> $code"
    return 1
  fi
}

err=0
check GET "$BASE/health" "health" || err=1
check GET "$API/config" "public config" || err=1
check GET "$API/categories" "categories" || err=1
check GET "$API/products/featured?limit=2" "featured products" || err=1
check GET "$API/products/new-arrivals?limit=2" "new arrivals" || err=1
check GET "$API/products/on-sale?limit=2" "on sale" || err=1
check GET "$API/shipping/methods" "shipping methods" || err=1
check GET "$API/videos?limit=2" "videos" || err=1

echo ""
if [ $err -eq 0 ]; then
  echo "All public API checks passed."
else
  echo "Some checks failed. Ensure backend is running (e.g. go run ./cmd/server) and MongoDB is up."
  exit 1
fi
