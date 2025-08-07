.PHONY: build run test clean deps lint migrate docker-build docker-run

# Variables
BINARY_NAME=api
MAIN_PATH=cmd/api/main.go
DOCKER_IMAGE=dbackup-backend
DOCKER_TAG=latest

# Build the application
build:
	@echo "Building..."
	@go build -o $(BINARY_NAME) $(MAIN_PATH)

# Run the application
run:
	@echo "Running..."
	@go run $(MAIN_PATH)

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Run unit tests only
test-unit:
	@echo "Running unit tests..."
	@go test -v -race -short ./...

# Run integration tests only
test-integration:
	@echo "Running integration tests..."
	@go test -v -race -run Integration ./tests/integration/...

# Generate test coverage report
coverage:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.txt coverage.out coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Run linters
lint:
	@echo "Running linters..."
	@golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run database migrations up
migrate-up:
	@echo "Running migrations up..."
	@go run cmd/migrate/main.go up

# Run database migrations down
migrate-down:
	@echo "Running migrations down..."
	@go run cmd/migrate/main.go down

# Create a new migration
migrate-create:
	@echo "Creating migration..."
	@go run cmd/migrate/main.go create -name="$(name)"

# Show migration status
migrate-status:
	@echo "Showing migration status..."
	@go run cmd/migrate/main.go status

# Show current migration version
migrate-version:
	@echo "Showing current migration version..."
	@go run cmd/migrate/main.go version

# Reset database (rollback all migrations)
migrate-reset:
	@echo "Resetting database..."
	@go run cmd/migrate/main.go reset

# Refresh database (reset and rerun all migrations)
migrate-refresh:
	@echo "Refreshing database..."
	@go run cmd/migrate/main.go refresh

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE):$(DOCKER_TAG)

# Run with hot reload (requires air)
dev:
	@echo "Running with hot reload..."
	@air

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Generate mocks (requires mockgen)
mocks:
	@echo "Generating mocks..."
	@go generate ./...

# Security audit
audit:
	@echo "Running security audit..."
	@go list -json -m all | nancy sleuth

# Benchmark tests
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Check for outdated dependencies
outdated:
	@echo "Checking for outdated dependencies..."
	@go list -u -m all

# Update all dependencies
update-deps:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

# Docker Compose Commands
compose-up: ## Start development environment with Docker Compose
	@echo "Starting development environment..."
	@docker-compose up -d
	@echo "Services started. API available at http://localhost:8080"

compose-build: ## Build and start development environment
	@echo "Building and starting development environment..."
	@docker-compose up -d --build

compose-down: ## Stop development environment
	@echo "Stopping development environment..."
	@docker-compose down

compose-logs: ## Follow development logs
	@docker-compose logs -f api

compose-prod: ## Start production environment
	@echo "Starting production environment..."
	@docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

compose-prod-down: ## Stop production environment
	@echo "Stopping production environment..."
	@docker-compose -f docker-compose.yml -f docker-compose.prod.yml down

compose-clean: ## Stop services and remove containers, networks
	@echo "Cleaning up containers and networks..."
	@docker-compose down --remove-orphans

compose-clean-all: ## Stop services and remove everything including volumes
	@echo "Cleaning up everything including volumes..."
	@docker-compose down -v --remove-orphans

compose-shell: ## Access API container shell
	@echo "Accessing API container shell..."
	@docker-compose exec api sh

compose-db-shell: ## Access PostgreSQL shell
	@echo "Connecting to PostgreSQL..."
	@docker-compose exec postgres psql -U postgres -d dbackup

compose-redis-cli: ## Access Redis CLI
	@echo "Connecting to Redis..."
	@docker-compose exec redis redis-cli

compose-status: ## Check Docker Compose service status
	@echo "Checking service status..."
	@docker-compose ps

setup-env: ## Copy .env.example to .env if not exists
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env file from .env.example"; \
		echo "Please review and update .env file if needed"; \
	else \
		echo ".env file already exists"; \
	fi

# Quick setup for new developers
quick-start: setup-env compose-up ## Quick setup: copy env and start services
	@echo "Quick start complete!"
	@echo "Services available:"
	@echo "  - API: http://localhost:8080"
	@echo "  - Adminer: http://localhost:8081"
	@echo "  - MinIO Console: http://localhost:9001"

# Integration Testing
test-integration: ## Run integration tests with Docker Compose
	@echo "Running Docker Compose integration tests..."
	@./scripts/test-docker-compose.sh --test

test-integration-full: ## Run full integration test suite including benchmarks
	@echo "Running full integration test suite..."
	@./scripts/test-docker-compose.sh --test --benchmark

test-docker-health: ## Run health checks only
	@echo "Running Docker Compose health checks..."
	@./scripts/test-docker-compose.sh --health

test-docker-logs: ## Show Docker Compose service logs
	@echo "Showing Docker Compose service logs..."
	@./scripts/test-docker-compose.sh --logs

# Performance Testing
benchmark-docker: ## Run performance benchmarks on Docker Compose stack
	@echo "Running Docker Compose performance benchmarks..."
	@./scripts/test-docker-compose.sh --benchmark