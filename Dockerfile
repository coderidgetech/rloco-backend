FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

# Copy go.mod first and download deps. Do not require go.sum in this step — some
# deploy contexts omit go.sum (not committed / wrong root); go mod download still works.
COPY go.mod ./
RUN go mod download

# Full source (includes go.sum when present for reproducible builds)
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/server .

RUN mkdir -p /app/uploads

EXPOSE 8080

ENV PORT=8080

CMD ["./server"]
