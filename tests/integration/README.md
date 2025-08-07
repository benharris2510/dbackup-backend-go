# Integration Tests

This directory contains integration tests for the dbackup backend, focusing on Docker Compose stack validation and end-to-end functionality testing.

## Test Files

- `docker_compose_test.go` - Docker Compose stack integration tests
- `auth_test.go` - Authentication integration tests  
- `server_test.go` - Server integration tests
- `twofa_test.go` - Two-factor authentication integration tests

## Docker Compose Integration Tests

The Docker Compose integration tests (`docker_compose_test.go`) validate the entire containerized stack:

### Test Coverage

1. **Service Health Checks**
   - API health endpoint validation
   - Service availability verification
   - Response time and status code validation

2. **Database Connectivity**  
   - PostgreSQL connection and query execution
   - MySQL connection and sample data validation
   - Database initialization script verification
   - Connection pooling and performance

3. **Cache Layer Testing**
   - Redis connectivity and basic operations
   - Persistence configuration validation
   - Performance benchmarking

4. **Storage Testing**
   - MinIO S3-compatible storage availability
   - Health endpoint validation
   - Basic connectivity testing

5. **Service Dependencies**
   - Inter-service communication validation
   - Network connectivity between containers
   - Dependency health verification

6. **Environment Configuration**
   - Environment variable validation
   - Configuration loading verification
   - Service-specific settings validation

7. **Volume Persistence**
   - Database data persistence testing
   - Redis AOF persistence validation
   - Volume mount verification

8. **Performance Benchmarking**
   - API response time benchmarking
   - Database query performance
   - Redis operation benchmarking
   - Service startup time measurement

## Running Integration Tests

### Using Make Commands

```bash
# Run integration tests only
make test-integration

# Run full integration test suite with benchmarks
make test-integration-full

# Run health checks only
make test-docker-health

# View service logs
make test-docker-logs

# Run performance benchmarks
make benchmark-docker
```

### Using the Test Script

```bash
# Run integration tests
./scripts/test-docker-compose.sh --test

# Run with benchmarks
./scripts/test-docker-compose.sh --test --benchmark

# Health checks only
./scripts/test-docker-compose.sh --health

# View logs only  
./scripts/test-docker-compose.sh --logs
```

### Manual Docker Compose Testing

```bash
# Start the stack
docker-compose up -d

# Wait for services to be ready
docker-compose ps

# Run tests inside container
docker-compose exec api go test -v -tags=integration ./tests/integration/docker_compose_test.go

# Run benchmarks
docker-compose exec api go test -bench=. -benchmem ./tests/integration/docker_compose_test.go

# Stop the stack
docker-compose down
```

## Test Configuration

### Environment Variables

The integration tests use the following test-specific environment variables:

```env
GO_ENV=test
DATABASE_URL=postgres://postgres:postgres@postgres:5432/dbackup?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET_KEY=test-jwt-secret-key
ENCRYPTION_MASTER_KEY=test-master-key-32-chars-long1234
```

### Test Docker Compose

A separate `docker-compose.test.yml` file provides test-specific overrides:

- Optimized database settings for testing
- Disabled admin interfaces
- Faster health check intervals
- Test-specific environment variables

Use with:
```bash
docker-compose -f docker-compose.yml -f docker-compose.test.yml up -d
```

## CI/CD Integration

### GitHub Actions

The tests are automatically run in GitHub Actions workflow:
- `.github/workflows/docker-integration-tests.yml`

The workflow includes:
- Docker Compose stack startup
- Service readiness verification  
- Integration test execution
- Performance benchmarking
- Security scanning with Trivy
- Load testing with Apache Bench

### Local CI Testing

To simulate the CI environment locally:

```bash
# Use the same environment as CI
cp .env.example .env

# Run the full test suite as CI would
docker-compose up -d --build
docker-compose exec -T api go test -v -tags=integration ./tests/integration/docker_compose_test.go
docker-compose down --volumes
```

## Test Structure

### Test Functions

1. **`TestDockerComposeStack`** - Main test suite
   - Runs all sub-tests in order
   - Validates complete stack functionality

2. **Individual Service Tests**
   - `testAPIHealthCheck` - API availability
   - `testPostgreSQLConnection` - PostgreSQL functionality  
   - `testRedisConnection` - Redis operations
   - `testMySQLConnection` - MySQL connectivity
   - `testMinIOConnection` - MinIO S3 storage

3. **Integration Tests**
   - `testServiceDependencies` - Inter-service communication
   - `testEnvironmentVariables` - Configuration validation
   - `testAPIDatabaseIntegration` - API + DB integration
   - `testAPIRedisIntegration` - API + Redis integration

4. **System Tests**
   - `TestDockerComposeServicesAvailability` - Service availability
   - `TestDockerComposeNetworking` - Network connectivity
   - `TestDockerComposeVolumes` - Volume persistence

5. **Performance Tests**
   - `BenchmarkDockerComposePerformance` - Performance benchmarking

### Test Helpers

- `isDockerEnvironment()` - Detects Docker environment
- Service connection helpers for each database type
- Performance measurement utilities
- Test data setup and cleanup functions

## Expected Test Results

### Successful Test Run

```
=== RUN   TestDockerComposeStack
=== RUN   TestDockerComposeStack/API_Health_Check
=== RUN   TestDockerComposeStack/PostgreSQL_Connection  
=== RUN   TestDockerComposeStack/Redis_Connection
=== RUN   TestDockerComposeStack/MySQL_Connection
=== RUN   TestDockerComposeStack/MinIO_Connection
=== RUN   TestDockerComposeStack/Service_Dependencies
--- PASS: TestDockerComposeStack (15.23s)
    --- PASS: TestDockerComposeStack/API_Health_Check (1.02s)
    --- PASS: TestDockerComposeStack/PostgreSQL_Connection (2.15s)
    --- PASS: TestDockerComposeStack/Redis_Connection (0.98s)
    --- PASS: TestDockerComposeStack/MySQL_Connection (3.45s)
    --- PASS: TestDockerComposeStack/MinIO_Connection (1.25s)
    --- PASS: TestDockerComposeStack/Service_Dependencies (6.38s)
PASS
```

### Performance Benchmarks

```
BenchmarkDockerComposePerformance/API_Health_Check-8         	     100	  12345678 ns/op
BenchmarkDockerComposePerformance/Database_Connection-8      	      50	  23456789 ns/op
BenchmarkDockerComposePerformance/Redis_Operations-8        	    1000	   1234567 ns/op
```

## Troubleshooting

### Common Issues

1. **Port Conflicts**
   ```bash
   # Check for port usage
   netstat -tulpn | grep :8080
   
   # Stop conflicting services
   docker-compose down
   ```

2. **Service Startup Timeouts**
   ```bash
   # Check service logs
   docker-compose logs postgres
   docker-compose logs api
   
   # Increase timeout in test script
   ```

3. **Database Connection Failures**
   ```bash
   # Verify PostgreSQL is ready
   docker-compose exec postgres pg_isready -U postgres
   
   # Check database exists
   docker-compose exec postgres psql -U postgres -l
   ```

4. **Test Environment Detection**
   ```bash
   # Force Docker environment detection
   export DOCKER_ENV=true
   
   # Run tests with explicit tags
   go test -tags=integration ./tests/integration/
   ```

### Debug Mode

Enable verbose logging:

```bash
# Set debug environment
export GO_ENV=test
export LOG_LEVEL=debug

# Run tests with verbose output
docker-compose exec api go test -v -tags=integration ./tests/integration/docker_compose_test.go
```

### Performance Issues

If tests are running slowly:

1. **Use test-specific compose file**
   ```bash
   docker-compose -f docker-compose.yml -f docker-compose.test.yml up -d
   ```

2. **Optimize database settings**
   - Disable fsync for testing
   - Use smaller buffer pools
   - Skip binary logging

3. **Use tmpfs for temporary data**
   ```yaml
   tmpfs:
     - /tmp
     - /var/tmp
   ```

## Contributing

When adding new integration tests:

1. Follow existing test naming conventions
2. Include proper error handling and cleanup
3. Add performance benchmarks for critical paths
4. Update documentation and test coverage
5. Ensure tests work in CI environment

### Test Guidelines

- Tests should be deterministic and repeatable
- Clean up all test data and resources
- Use appropriate timeouts and retries
- Validate both positive and negative scenarios
- Include performance assertions where relevant