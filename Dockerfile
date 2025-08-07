# Build stage
FROM golang:1.24-alpine AS builder

# Install system dependencies required for building
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    postgresql-client \
    mysql-client \
    gcc \
    musl-dev \
    sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 go build \
    -ldflags='-w -s' \
    -a \
    -o main ./cmd/api

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    postgresql-client \
    mysql-client \
    sqlite \
    curl \
    && rm -rf /var/cache/apk/*

# Create non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/main .

# Copy configuration example file if it exists
COPY --from=builder /app/config.example.yaml ./config.example.yaml

# Create directories for logs and data
RUN mkdir -p /app/logs /app/data && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:8080/api/health/live || exit 1

# Set environment variables
ENV GO_ENV=production \
    PORT=8080 \
    LOG_LEVEL=info

# Run the binary
CMD ["./main"]