# R-Loko Backend API

A robust Go-based backend API for the R-Loko e-commerce platform, built with Gin framework and MongoDB.

## Features

- **RESTful API** with Gin framework
- **MongoDB** database with connection pooling
- **JWT Authentication** with HttpOnly cookies
- **Role-Based Access Control** (Customer, Admin, Vendor)
- **Product Management** with image uploads
- **Order Processing** with inventory management
- **Payment Integration** (Stripe & PayPal)
- **Email Notifications** via SMTP
- **Returns & Refunds** management
- **Customer Support** ticket system
- **Reviews & Ratings** system
- **Dynamic Shipping & Tax** calculation
- **Analytics & Reporting** for admin dashboard

## Tech Stack

- **Go 1.21+**
- **Gin** - HTTP web framework
- **MongoDB** - NoSQL database
- **JWT** - Authentication
- **MinIO** - Object storage (optional)
- **Docker** - Containerization

## Quick Start

### Prerequisites

- Go 1.21 or higher
- MongoDB (or Docker)
- MinIO (optional, for file storage)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd rloco-backend
```

**Monorepo-style workspace (optional):** If you keep this repo inside a parent folder next to `frontend/` and `mobile/`, the backend clone often lives at `your-workspace/backend/`. Run all `git` and `go` commands from **`backend/`** (where this `.git` lives). A fresh clone of `rloco-backend` alone has `go.mod` at the **clone root** — there is no `backend/` subfolder on GitHub.

2. Install dependencies:
```bash
go mod download
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. Start MongoDB and MinIO (using Docker):
```bash
cd docker
docker-compose up -d mongodb minio
```

5. Run the server:
```bash
make run
# or
go run cmd/server/main.go
```

The API will be available at `http://localhost:8080`

## Environment Variables

Create a `.env` file in the backend directory:

```env
PORT=8080
ENV=development
MONGODB_URI=mongodb://admin:password@localhost:27017/rloco?authSource=admin
JWT_SECRET=your-secret-key-change-in-production
JWT_EXPIRY=24h

# Storage (optional)
STORAGE_TYPE=local
STORAGE_ENDPOINT=localhost:9000
STORAGE_ACCESS_KEY=minioadmin
STORAGE_SECRET_KEY=minioadmin
STORAGE_BUCKET=rloco-uploads

# Email (optional)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM=noreply@rloco.com
SMTP_FROM_NAME=R-Loko

# Payment Gateways (optional)
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...
PAYPAL_CLIENT_ID=...
PAYPAL_SECRET=...
PAYPAL_MODE=sandbox
```

## API Endpoints

### Public Endpoints

- `GET /health` - Health check
- `POST /api/auth/register` - User registration
- `POST /api/auth/login` - User login
- `GET /api/products` - List products
- `GET /api/products/:id` - Get product details
- `GET /api/products/featured` - Featured products
- `GET /api/products/new-arrivals` - New arrivals
- `GET /api/products/on-sale` - Sale products
- `GET /api/categories` - List categories
- `GET /api/promotions` - List active promotions
- `POST /api/promotions/validate` - Validate promotion code
- `GET /api/config` - Public site configuration

### Authenticated Endpoints

- `GET /api/auth/me` - Get current user
- `POST /api/auth/logout` - Logout
- `GET /api/cart` - Get cart
- `POST /api/cart/items` - Add to cart
- `PUT /api/cart/items/:id` - Update cart item
- `DELETE /api/cart/items/:id` - Remove from cart
- `GET /api/wishlist` - Get wishlist
- `POST /api/wishlist/items` - Add to wishlist
- `DELETE /api/wishlist/items/:id` - Remove from wishlist
- `GET /api/orders` - Get user orders
- `POST /api/orders` - Create order
- `GET /api/orders/:id` - Get order details

### Admin/Vendor Endpoints

- Product CRUD operations
- Category management
- Order management
- Customer management
- Vendor management
- Promotion management
- Analytics endpoints
- Configuration management

See `docs/README.md` for detailed API documentation.

## Project Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── handlers/            # HTTP handlers
│   ├── middleware/          # HTTP middleware
│   ├── models/              # Data models
│   ├── repositories/        # Database repositories
│   └── services/            # Business logic
├── docker/                  # Docker configuration
├── migrations/              # Database migrations
├── docs/                    # Documentation
├── go.mod                   # Go dependencies
├── Makefile                 # Development commands
└── README.md               # This file
```

## Development

### Available Commands

```bash
make run          # Run the server
make build        # Build the binary
make test         # Run tests
make clean        # Clean build artifacts
make docker-up    # Start Docker services
make docker-down  # Stop Docker services
make seed         # Seed the database
```

### Running Tests

```bash
go test ./...
```

### Database Seeding

```bash
make seed
```

## Docker Deployment

### Using Docker Compose

```bash
cd docker
docker-compose up -d
```

This starts:
- MongoDB on port 27017
- MinIO on ports 9000-9001
- Backend API on port 8080

### Building Docker Image

```bash
cd docker
docker build -t rloco-backend .
```

## Documentation

- **Quick Start**: `docs/QUICKSTART.md`
- **API Documentation**: `docs/README.md`
- **Payment Integration**: `docs/PAYMENT_INTEGRATION.md`
- **RBAC Guide**: `docs/RBAC_IMPLEMENTATION_COMPLETE.md`
- **Scalability**: `docs/SCALABILITY_FIXES_APPLIED.md`

## Security

- JWT tokens stored in HttpOnly cookies
- Password hashing with bcrypt
- CORS protection
- Rate limiting (production)
- Input validation
- SQL injection protection (MongoDB)
- XSS protection

## License

[Your License Here]
