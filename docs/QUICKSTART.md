# Quick Start Guide

## Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- Make (optional, but recommended)

## Step 1: Install Dependencies

```bash
cd backend
go mod tidy
```

## Step 2: Start Docker Services

```bash
make docker-up
# or
docker-compose -f docker/docker-compose.yml up -d
```

This starts:
- MongoDB on port 27017
- MinIO on ports 9000 and 9001
- Backend will start automatically

## Step 3: Seed Database

```bash
make seed
# or
go run migrations/seed.go
```

This creates:
- Admin user: `admin@rloco.com` / `admin123`
- Default categories

## Step 4: Start Backend Server

```bash
make run
# or
go run cmd/server/main.go
```

The API will be available at `http://localhost:8080`

## Step 5: Test the API

### Health Check
```bash
curl http://localhost:8080/health
```

### Register a User
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123",
    "name": "Test User"
  }'
```

### Login
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

### Get Products (Public)
```bash
curl http://localhost:8080/api/products
```

### Admin Login
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@rloco.com",
    "password": "admin123"
  }'
```

Save the token from the response and use it in subsequent requests:
```bash
curl http://localhost:8080/api/auth/me \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

## Frontend Integration

1. Update your frontend API base URL to `http://localhost:8080/api`
2. Update authentication to use JWT tokens
3. Replace all mock data with API calls
4. Update context providers to use the API

## Troubleshooting

### MongoDB Connection Issues
- Ensure MongoDB container is running: `docker ps`
- Check MongoDB logs: `docker logs rloco-mongodb`
- Verify connection string in `.env`

### Port Already in Use
- Change port in `.env` or `docker-compose.yml`
- Kill process using the port

### Dependencies Not Found
- Run `go mod tidy`
- Ensure you're in the `backend` directory

## Next Steps

1. Export product data from `src/app/data/products.ts`
2. Create a script to import products to MongoDB
3. Configure email service (SMTP settings)
4. Update frontend to use the API
5. Test all features end-to-end

## Useful Commands

```bash
# View logs
make docker-logs

# Stop services
make docker-down

# Rebuild and restart
make docker-down && make docker-up

# Clean build artifacts
make clean
```
