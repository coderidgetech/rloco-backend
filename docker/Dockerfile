FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go.mod first (go.sum optional in some deploy contexts)
COPY go.mod ./
RUN go mod download

# Full source (includes go.sum when present)
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/server .

# Create uploads directory
RUN mkdir -p /app/uploads

EXPOSE 8080

CMD ["./server"]
