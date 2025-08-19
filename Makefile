.PHONY: help build clean email-service notification-service google-analytics order-basic order-improved email-worker notification-worker outbox-worker test-basic test-improved

help:
	@echo "Available commands:"
	@echo "  Services:"
	@echo "    make email-service        - Run email service on port 8081"
	@echo "    make notification-service - Run notification service on port 8082"
	@echo "    make google-analytics     - Run analytics service on port 9000"
	@echo "    make order-basic          - Run basic order service on port 8080"
	@echo "    make order-improved       - Run improved order service on port 8083"
	@echo "  Workers:"
	@echo "    make email-worker         - Run email sender worker"
	@echo "    make notification-worker  - Run notification sender worker"
	@echo "    make outbox-worker        - Run outbox worker"
	@echo "  Testing:"
	@echo "    make test-basic ARGS=100  - Run 100 basic order simulations"
	@echo "    make test-improved ARGS=100 - Run 100 improved order simulations"
	@echo "  Utils:"
	@echo "    make build                - Build all services"
	@echo "    make clean                - Clean build artifacts"

build:
	@echo "Building all services..."
	@go build -o bin/email-service cmd/main.go
	@go build -o bin/notification-service cmd/main.go
	@go build -o bin/google-analytics cmd/main.go
	@go build -o bin/order-basic cmd/main.go
	@go build -o bin/order-improved cmd/main.go
	@go build -o bin/email-worker cmd/main.go
	@go build -o bin/notification-worker cmd/main.go
	@go build -o bin/outbox-worker cmd/main.go
	@go build -o bin/test-simulation test-simulation/main.go

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@go clean

email-service:
	@echo "Starting email service on port 8081..."
	@go run cmd/main.go email-service

notification-service:
	@echo "Starting notification service on port 8082..."
	@go run cmd/main.go notification-service

google-analytics:
	@echo "Starting google analytics service on port 9000..."
	@go run cmd/main.go google-analytics

order-basic:
	@echo "Starting basic order service on port 8080..."
	@go run cmd/main.go order-basic

order-improved:
	@echo "Starting improved order service on port 8083..."
	@go run cmd/main.go order-improved

email-worker:
	@echo "Starting email sender worker..."
	@go run cmd/main.go email-worker

notification-worker:
	@echo "Starting notification sender worker..."
	@go run cmd/main.go notification-worker

outbox-worker:
	@echo "Starting outbox worker..."
	@go run cmd/main.go outbox-worker

test-basic:
	@echo "Running basic order simulation with $(or $(ARGS),100) orders..."
	@go run test-simulation/main.go basic $(or $(ARGS),100)

test-improved:
	@echo "Running improved order simulation with $(or $(ARGS),100) orders..."
	@go run test-simulation/main.go improved $(or $(ARGS),100)
