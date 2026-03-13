.PHONY: run run-mcp migrate migrate-down test docker-up docker-down

run:
	go run ./cmd/server

run-mcp:
	go run ./cmd/mcp

migrate:
	migrate -path db/migrations -database "$${DATABASE_URL:-postgres://warrant:warrant@localhost:5433/warrant?sslmode=disable}" up

migrate-down:
	migrate -path db/migrations -database "$${DATABASE_URL:-postgres://warrant:warrant@localhost:5433/warrant?sslmode=disable}" down 1

test:
	go test ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down
