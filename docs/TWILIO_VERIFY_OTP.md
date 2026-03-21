# Twilio Verify — registration & login OTP

Phone **signup** and **customer login** use **Twilio Verify** (SMS channel) only. There is no fallback provider or local OTP.

## Environment variables

| Variable | Description |
|----------|-------------|
| `TWILIO_ACCOUNT_SID` | Account SID from [Twilio Console](https://console.twilio.com/). |
| `TWILIO_AUTH_TOKEN` | Auth token for the account (keep secret). |
| `TWILIO_VERIFY_SERVICE_SID` | **Verify Service SID** (`VA...`) from Verify → Services. |
| `OTP_DEFAULT_COUNTRY_CODE` | Optional. Default `91`. If the user enters a **10-digit** local number, this country code is prefixed before E.164 is sent to Twilio. |

The server **exits at startup** if any of the three Twilio variables are missing.

### Trial accounts (error 21608)

**Trial** Twilio projects can only send SMS to **verified** numbers. Add each test handset under **Console → Phone Numbers → Verified Caller IDs**, or upgrade the account. If OTP send fails, the API returns a short message pointing to [Twilio error 21608](https://www.twilio.com/docs/errors/21608).

## Docker (local)

[`backend/docker/docker-compose.yml`](../docker/docker-compose.yml) loads **`backend/.env`** into the backend container (`env_file` with `required: false`). Put `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, and `TWILIO_VERIFY_SERVICE_SID` in **`backend/.env`**, then from the repo root run:

```bash
./docker-run.sh --stop
./docker-run.sh --build   # or without --build for faster restart
```

API base from the host: **`http://localhost:8080/api`** (frontend dev compose uses `VITE_API_URL=http://localhost:8080/api`).

## API

**Signup**

- `POST /api/auth/register-otp/send` — body `{ "phone": "..." }` (digits; normalized server-side). Fails if an account with that phone already exists.
- `POST /api/auth/register-otp/complete` — body `{ "phone", "code", "email", "password", "name" }`.

**Login (existing customers only)**

- `POST /api/auth/login-otp/send` — body `{ "phone": "..." }`. Requires an **active customer** with that `phone_key` on file (e.g. after phone signup or profile update). Admins/vendors must use email sign-in.
- `POST /api/auth/login-otp/complete` — body `{ "phone", "code" }` (`code`: 6 digits). Sets the same `auth_token` cookie as password/Google login and returns `{ "user", "token" }`.

Flow: **CreateVerification** (SMS) → store Twilio **Verification SID** (`VE...`) in Mongo (per purpose: `registration` vs `login`) → **CreateVerificationCheck** with **`To` (E.164) + `Code`**, matching the Twilio REST API (same as a curl `VerificationCheck` with `To` and `Code`). Server logs each send at INFO (`twilio verify: verification started …`).

### Confirm env inside Docker

After editing `backend/.env`, **rebuild** the backend image so the new binary is included (`docker compose build backend` or `./docker-run.sh --build`), then restart. Confirm the container sees your Twilio vars (values redacted when sharing):

```bash
docker exec rloco-backend printenv | grep TWILIO
```

You should see `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, and `TWILIO_VERIFY_SERVICE_SID` exactly as in the Twilio Console for the Verify service you test with curl.

There is **no** dev bypass or fixed “test” OTP in the API: every send uses Twilio Verify, and every verify call is checked against Twilio.

## Twilio Console checklist

1. **Verify** → **Services** → create a service → copy **Service SID** (`VA...`).
2. Enable **SMS** for the service; set **geo permissions** for countries you need (e.g. India, US).
3. Complete **regulatory / sender** requirements Twilio shows for those regions (delivery can fail until done).
4. Trial accounts can only message **verified** caller IDs until you upgrade.

## References

- [Twilio Verify API](https://www.twilio.com/docs/verify/api)
