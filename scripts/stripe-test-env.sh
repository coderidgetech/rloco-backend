#!/usr/bin/env bash
# Use Stripe keys securely via environment (no keys in this file).
# 1. Copy backend/.env.example to backend/.env and frontend/.env.example to frontend/.env
# 2. Add your Stripe keys to those .env files (never commit them)
# 3. Ensure MongoDB is running (e.g. Docker or local install)
# 4. Run this script from repo root, or run backend and frontend manually:
#
#   Terminal 1 (optional, for webhook): stripe listen --forward-to http://localhost:8080/api/webhooks/stripe
#   Terminal 2: cd backend && go run ./cmd/server
#   Terminal 3: cd frontend && pnpm run dev
#
# Then open the app, add items to cart, checkout with card 4242 4242 4242 4242.

set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ ! -f backend/.env ]; then
  echo "Creating backend/.env from backend/.env.example"
  cp backend/.env.example backend/.env
  echo "  → Add STRIPE_SECRET_KEY (and STRIPE_WEBHOOK_SECRET if using stripe listen) to backend/.env"
fi
if [ ! -f frontend/.env ]; then
  echo "Creating frontend/.env from frontend/.env.example"
  cp frontend/.env.example frontend/.env
  echo "  → Add VITE_STRIPE_PUBLISHABLE_KEY to frontend/.env"
fi
echo "Env files ready. Start MongoDB, then run backend and frontend (see comments at top of this script)."
echo "Stripe test card: 4242 4242 4242 4242"
