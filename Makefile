.PHONY: build run test clean docker-up docker-down seed seed-demo

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server/main.go

test:
	go test ./...

clean:
	rm -rf bin/
	rm -rf uploads/

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down

docker-logs:
	docker compose -f docker/docker-compose.yml logs -f

docker-build:
	docker build -f docker/Dockerfile -t rloco-backend:local .

seed:
	go run migrations/seed.go

seed-demo:
	go run migrations/seed_demo.go

tidy:
	go mod tidy
