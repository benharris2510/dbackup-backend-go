.PHONY: build run test clean deps lint migrate docker-build docker-run

# Variables
BINARY_NAME=api
MAIN_PATH=cmd/api/main.go
DOCKER_IMAGE=dbackup-backend-go
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
	@migrate -path migrations -database "${DATABASE_URL}" up

# Run database migrations down
migrate-down:
	@echo "Running migrations down..."
	@migrate -path migrations -database "${DATABASE_URL}" down

# Create a new migration
migrate-create:
	@echo "Creating migration..."
	@migrate create -ext sql -dir migrations -seq $(name)

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