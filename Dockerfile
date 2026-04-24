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

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/server .

RUN mkdir -p /app/uploads

EXPOSE 8080

ENV PORT=8080
# App Platform often omits spec/env vars for Dockerfile services; production needs a
# non-default JWT or the server exits. Runtime env from the platform overrides this ENV.
ENV JWT_SECRET=b53383d58eca2bf5800f60a5a1bc29ce10542d49bf8725b113377d6209878a52055fe546820eb6985a2cd77ebbb5d72f

CMD ["./server"]
