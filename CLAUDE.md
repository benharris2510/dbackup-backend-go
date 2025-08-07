# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the Go backend for dbackup - a database backup management platform. The application uses Echo as the web framework with PostgreSQL as the primary database, Redis for caching/queuing, and supports backup operations to S3-compatible storage.

## Development Commands

### Core Commands
```bash
# Build the application
make build

# Run the application
make run

# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests
make test-integration

# Generate test coverage report
make coverage

# Run linter
make lint

# Format code
make fmt
```

### Database Migration Commands
```bash
# Run pending migrations
make migrate-up

# Rollback migrations
make migrate-down

# Create new migration
make migrate-create name="migration_name"

# Show migration status
make migrate-status

# Show current migration version
make migrate-version

# Reset database (rollback all)
make migrate-reset

# Refresh database (reset and rerun all)
make migrate-refresh
```

### Docker Commands
```bash
# Start development environment
make compose-up

# Start with build
make compose-build

# Stop services
make compose-down

# View logs
make compose-logs

# Quick setup for new developers
make quick-start

# Run integration tests with Docker
make test-integration

# Access API container shell
make compose-shell

# Access PostgreSQL shell
make compose-db-shell

# Access Redis CLI
make compose-redis-cli
```

### Development Tools
```bash
# Run with hot reload (requires air)
make dev

# Install development tools
make install-tools

# Generate mocks
make mocks

# Security audit
make audit

# Run benchmarks
make bench

# Check for outdated dependencies
make outdated

# Update all dependencies
make update-deps
```

## Architecture

### Project Structure
- `cmd/` - Entry points (api server, migration tool)
- `internal/` - Private application code
  - `auth/` - JWT, password hashing, TOTP
  - `config/` - Configuration management with Viper
  - `database/` - Database connection and migration system
  - `encryption/` - Data encryption services
  - `handlers/` - HTTP handlers for API endpoints
  - `middleware/` - Echo middleware (CORS, auth, security)
  - `models/` - GORM models for database entities
  - `routes/` - Route definitions and groupings
  - `services/` - Business logic services
  - `websocket/` - WebSocket functionality
  - `workers/` - Background job processors
  - `queue/` - Queue management
- `migrations/` - SQL migration files
- `tests/` - Integration and unit tests
- `scripts/` - Database initialization scripts
- `docker/` - Docker configurations

### Key Dependencies
- **Web Framework**: Echo v4 for HTTP server
- **Database**: GORM with PostgreSQL driver, also supports MySQL/SQLite
- **Configuration**: Viper for config management
- **Authentication**: JWT tokens, bcrypt password hashing, TOTP for 2FA
- **Caching**: Redis with go-redis client
- **Cloud Storage**: AWS SDK v2 for S3-compatible storage
- **Background Jobs**: Asynq for job queuing
- **Testing**: Testify for assertions, sqlmock for database testing
- **Encryption**: Built-in crypto for data encryption

### Database Migration System
The project uses a custom migration system with comprehensive features:
- Timestamp-based versioning (YYYYMMDDHHMMSS format)
- SQL file support with `-- +migrate Up` and `-- +migrate Down` markers
- Transaction safety and rollback support
- Batch tracking and status monitoring
- CLI tool at `cmd/migrate/main.go`

### Configuration
Configuration uses Viper with YAML files:
- `config.example.yaml` - Example configuration template
- Environment-based config loading
- Singleton pattern for config access
- Development/production environment detection

### Authentication & Security
- JWT-based authentication with access/refresh tokens
- TOTP-based 2FA support
- bcrypt password hashing
- Data encryption service with master key
- CORS middleware with configurable origins
- Security headers middleware
- Rate limiting support

### Background Processing
- Asynq for async job processing
- Redis as message broker
- Backup workers for database operations
- WebSocket for real-time progress updates
- Graceful shutdown handling

## Testing Strategy

### Unit Tests
- Located alongside source files (*_test.go)
- Use testify for assertions
- sqlmock for database testing
- Run with `make test-unit`

### Integration Tests
- Located in `tests/integration/`
- Docker Compose stack validation
- End-to-end functionality testing
- Service dependency testing
- Run with `make test-integration`

### Test Tags
- Use `-tags=integration` for integration tests
- Tests detect Docker environment automatically
- Separate test configuration in docker-compose.test.yml

## Development Environment

### Docker Setup
The project provides comprehensive Docker setup:
- `docker-compose.yml` - Base configuration
- `docker-compose.override.yml` - Development overrides (auto-loaded)
- `docker-compose.prod.yml` - Production configuration
- `docker-compose.test.yml` - Test-specific settings

### Services
- API: Go backend (port 8080)
- PostgreSQL: Primary database (port 5432)
- Redis: Cache/queue (port 6379)
- MinIO: S3-compatible storage (port 9000, console 9001)
- MySQL: Test database (port 3306)
- Adminer: Database UI (port 8081)

### Hot Reload
Development container uses Air for automatic reloading on Go code changes.

## Best Practices

### Code Organization
- Follow the established internal package structure
- Use GORM models in the `models/` package
- Business logic in `services/` package
- HTTP concerns in `handlers/` package
- Middleware in `middleware/` package

### Database Operations
- Always use migrations for schema changes
- Include both up and down migrations
- Test migrations on a copy of production data
- Use transactions for data integrity

### Error Handling
- Use structured error responses
- Log errors with appropriate context
- Handle graceful shutdowns properly
- Validate input data with the validator package

### Security
- Never commit secrets to the repository
- Use the encryption service for sensitive data
- Implement proper authentication middleware
- Follow OWASP security guidelines

### Testing
- Write tests for all new functionality
- Use table-driven tests where appropriate
- Include both positive and negative test cases
- Run integration tests in Docker environment

## Configuration Management

### Environment Variables
Set via `.env` file or environment:
- `DATABASE_URL` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string
- `JWT_SECRET_KEY` - JWT signing secret
- `ENCRYPTION_MASTER_KEY` - Data encryption key

### Config Files
- Copy `config.example.yaml` to `config.yaml`
- Modify database URLs and secrets
- Configure CORS origins for frontend
- Set appropriate log levels

## Common Development Workflows

### Adding New API Endpoints
1. Create handler function in `internal/handlers/`
2. Add route in `internal/routes/`
3. Add middleware if needed
4. Write unit tests
5. Add integration tests if applicable

### Database Schema Changes
1. Create migration: `make migrate-create name="description"`
2. Edit the generated SQL file with up/down migrations
3. Test migration: `make migrate-up`
4. Update GORM models if needed
5. Write tests for model changes

### Adding Background Jobs
1. Define job struct and handler in `internal/workers/`
2. Register with queue service
3. Add job scheduling logic in services
4. Test job execution
5. Add monitoring/logging

### Debugging
- Use `make compose-logs` to view container logs
- Access container shell with `make compose-shell`
- Use database shell with `make compose-db-shell`
- Enable debug logging in configuration
- Use Go debugger with `dlv` in development

This project follows Go best practices and uses established patterns for web services, database operations, and testing.