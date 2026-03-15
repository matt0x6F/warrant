# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /warrant ./cmd/server

# Migrate binary (for entrypoint)
FROM migrate/migrate:v4.18.3 AS migrate

# Run stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /warrant .
COPY --from=migrate /usr/local/bin/migrate /usr/local/bin/migrate
COPY db/migrations /app/db/migrations
EXPOSE 8080
CMD ["sh", "-c", "migrate -path /app/db/migrations -database \"$DATABASE_URL\" up && exec ./warrant"]
