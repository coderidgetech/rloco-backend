# syntax=docker/dockerfile:1.4
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Full source
COPY . .

# -a removed: much faster; CGO_ENABLED=0 still gives static binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

COPY --from=builder /app/server .

RUN mkdir -p /app/uploads

EXPOSE 8080

ENV PORT=8080
# JWT_SECRET is NOT baked into the image. Provide it at runtime (platform env or the
# droplet .env). In production the server refuses to start without a non-default value.

CMD ["./server"]
