# Railway "Application failed to respond" – Troubleshooting

## 0. Monorepo: Root Directory must be `backend`

This repo keeps the Go API under **`backend/`** (not at the repository root).

**In Railway → your backend service → Settings → Service:**

1. Set **Root Directory** to: **`backend`**
2. Save and redeploy.

Railway will then use **`backend/Dockerfile`** and **`backend/railway.toml`** / **`backend/railway.json`** as the build context. If Root Directory is left empty (repo root), the build will not find `go.mod` at the top level and deploys will fail or use the wrong layout.

---

## 1. Check deploy logs

**Railway → Your service → Deployments → Click latest → View logs**

Look for errors like:
- `Failed to load configuration: production requires a strong JWT_SECRET`
- `Failed to connect to database`
- `panic` or `Fatal`

---

## 2. Required Railway variables

Set these in **Railway → Settings → Variables**:

| Variable | Required | Notes |
|----------|----------|-------|
| `PORT` | Auto-set by Railway | Don't override |
| `ENV` | Yes | Use `production` or `development` |
| `MONGODB_URI` | Yes | MongoDB Atlas connection string |
| `JWT_SECRET` | Yes (if ENV=production) | Must be a strong random string, NOT `your-secret-key-change-in-production` |
| `JWT_EXPIRY` | No | Default: 24h |
| `STORAGE_TYPE` | Yes | Use `s3` for R2/Cloudflare (Railway has no MinIO) |
| `STORAGE_ENDPOINT` | If s3 | e.g. `https://xxx.r2.cloudflarestorage.com` |
| `STORAGE_ACCESS_KEY` | If s3 | R2/S3 access key |
| `STORAGE_SECRET_KEY` | If s3 | R2/S3 secret key |
| `STORAGE_BUCKET` | If s3 | e.g. `rloco-uploads` |
| `CORS_ALLOWED_ORIGINS` | Recommended | Your frontend URL(s), comma-separated |
| `APP_BASE_URL` | Recommended | Your Railway URL, e.g. `https://xxx.up.railway.app` |

---

## 3. Common fixes

### A. JWT_SECRET error (production)

If you see: `production requires a strong JWT_SECRET`

**Fix:** Either:
- Set `JWT_SECRET` to a long random string (e.g. 32+ chars), or
- Set `ENV=development` temporarily (less secure)

### B. MongoDB connection failed

- Ensure `MONGODB_URI` is correct (MongoDB Atlas)
- In MongoDB Atlas: **Network Access** → Add `0.0.0.0/0` to allow Railway IPs
- Check the connection string includes `?retryWrites=true` if needed

### C. Storage (MinIO not available on Railway)

Railway has no MinIO. You must use S3-compatible storage:

- Set `STORAGE_TYPE=s3`
- Use Cloudflare R2 or AWS S3
- Set `STORAGE_ENDPOINT`, `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, `STORAGE_BUCKET`

### D. App binds correctly

The app already binds to `0.0.0.0:PORT` – no code change needed.

---

## 4. Quick test after fix

```bash
curl https://YOUR-RAILWAY-URL.up.railway.app/health
```

Expected: `{"status":"ok"}`
