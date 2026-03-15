# Email Notifications and Alerts

## Overview

The backend sends transactional emails and admin alerts via SMTP. When SMTP is not configured, send attempts are skipped and logged (no error returned so the app keeps running).

## Environment Variables

| Variable        | Required | Description |
|----------------|----------|-------------|
| `SMTP_HOST`    | For sending | SMTP server (e.g. `smtp.sendgrid.net`, `smtp.gmail.com`) |
| `SMTP_PORT`    | For sending | Usually `587` (STARTTLS) or `465` (TLS) |
| `SMTP_USER`    | For sending | SMTP username / API user |
| `SMTP_PASSWORD`| For sending | SMTP password or API key |
| `SMTP_FROM`    | No | Sender address (default `noreply@rloco.com`) |
| `SMTP_FROM_NAME` | No | Sender name (default `R-Loko`) |
| `APP_BASE_URL` | No | Base URL for links in emails (default `https://rloco.com`). Used for verify-email and reset-password links. |
| `ADMIN_EMAIL`  | No | If set, receives a **new order alert** for every placed order. |

## Notifications Sent

| Event | Recipient | Email |
|-------|-----------|--------|
| Order created | Customer | Order confirmation |
| Order created | Admin (if `ADMIN_EMAIL` set) | New order alert |
| Payment succeeded (Stripe webhook) | Customer | Payment received |
| Order status → shipped | Customer | Shipping notification (with tracking number) |
| Order status change | Customer | Order status update |
| Order cancelled | Customer | Order status update (cancelled) |
| Password reset requested | User | Password reset link |
| Email verification / resend | User | Verify email link |
| Return requested | Customer | Return confirmation |
| Refund processed | Customer | Refund notification |

## Providers

- **SendGrid**: Use SMTP with API key; `SMTP_USER=apikey`, `SMTP_PASSWORD=<your_key>`, `SMTP_HOST=smtp.sendgrid.net`, `SMTP_PORT=587`.
- **Gmail**: Use App Password; `SMTP_HOST=smtp.gmail.com`, `SMTP_PORT=587`, enable “Less secure app access” or use App Passwords.
- **AWS SES**, **Mailgun**, **Postmark**: Use their SMTP endpoints and credentials.

## Logging

- When SMTP is not configured: `[Email] Not configured: would send to <to>: <subject>`.
- On send success: `[Email] Sent to <to>: <subject>`.
- On send failure: `[Email] Send failed to <to>: <error>`.

## Links in Emails

- **Verify email**: `{APP_BASE_URL}/verify-email?token=...`
- **Reset password**: `{APP_BASE_URL}/reset-password?token=...`

Set `APP_BASE_URL` to your frontend URL (e.g. `https://shop.yourdomain.com`) so links point to the correct site.
