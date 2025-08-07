#!/bin/bash

# Docker Compose Integration Test Script
# This script starts the Docker Compose stack and runs integration tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker and Docker Compose are installed
check_dependencies() {
    log_info "Checking dependencies..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose is not installed"
        exit 1
    fi
    
    log_success "Dependencies check passed"
}

# Clean up any existing containers
cleanup() {
    log_info "Cleaning up existing containers..."
    docker-compose down --remove-orphans || true
    docker-compose -f docker-compose.yml -f docker-compose.override.yml down --remove-orphans || true
}

# Start the Docker Compose stack
start_stack() {
    log_info "Starting Docker Compose stack..."
    
    # Ensure we have an .env file
    if [ ! -f .env ]; then
        log_info "Creating .env file from .env.example"
        cp .env.example .env
    fi
    
    # Start services
    docker-compose up -d --build
    
    log_info "Waiting for services to be ready..."
    
    # Wait for PostgreSQL
    log_info "Waiting for PostgreSQL..."
    timeout=60
    while [ $timeout -gt 0 ]; do
        if docker-compose exec postgres pg_isready -U postgres > /dev/null 2>&1; then
            log_success "PostgreSQL is ready"
            break
        fi
        sleep 2
        timeout=$((timeout - 2))
    done
    
    if [ $timeout -le 0 ]; then
        log_error "PostgreSQL failed to start within timeout"
        docker-compose logs postgres
        exit 1
    fi
    
    # Wait for Redis
    log_info "Waiting for Redis..."
    timeout=30
    while [ $timeout -gt 0 ]; do
        if docker-compose exec redis redis-cli ping > /dev/null 2>&1; then
            log_success "Redis is ready"
            break
        fi
        sleep 2
        timeout=$((timeout - 2))
    done
    
    if [ $timeout -le 0 ]; then
        log_error "Redis failed to start within timeout"
        docker-compose logs redis
        exit 1
    fi
    
    # Wait for API
    log_info "Waiting for API..."
    timeout=60
    while [ $timeout -gt 0 ]; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            log_success "API is ready"
            break
        fi
        sleep 2
        timeout=$((timeout - 2))
    done
    
    if [ $timeout -le 0 ]; then
        log_error "API failed to start within timeout"
        docker-compose logs api
        exit 1
    fi
    
    # Wait for MySQL
    log_info "Waiting for MySQL..."
    timeout=60
    while [ $timeout -gt 0 ]; do
        if docker-compose exec mysql mysqladmin ping -h localhost -u root -prootpassword > /dev/null 2>&1; then
            log_success "MySQL is ready"
            break
        fi
        sleep 2
        timeout=$((timeout - 2))
    done
    
    if [ $timeout -le 0 ]; then
        log_warning "MySQL failed to start within timeout (continuing anyway)"
        docker-compose logs mysql
    fi
    
    # Wait for MinIO
    log_info "Waiting for MinIO..."
    timeout=30
    while [ $timeout -gt 0 ]; do
        if curl -s http://localhost:9000/minio/health/live > /dev/null 2>&1; then
            log_success "MinIO is ready"
            break
        fi
        sleep 2
        timeout=$((timeout - 2))
    done
    
    if [ $timeout -le 0 ]; then
        log_warning "MinIO failed to start within timeout (continuing anyway)"
        docker-compose logs minio
    fi
    
    log_success "Docker Compose stack is running"
    
    # Show service status
    log_info "Service status:"
    docker-compose ps
}

# Run integration tests
run_tests() {
    log_info "Running integration tests..."
    
    # Create a test container to run tests from
    docker-compose exec api go test -v -tags=integration ./tests/integration/docker_compose_test.go || {
        log_error "Integration tests failed"
        return 1
    }
    
    log_success "Integration tests passed"
}

# Run performance benchmarks
run_benchmarks() {
    log_info "Running performance benchmarks..."
    
    docker-compose exec api go test -bench=. -benchmem ./tests/integration/docker_compose_test.go || {
        log_warning "Benchmarks failed or not available"
        return 1
    }
    
    log_success "Performance benchmarks completed"
}

# Show service logs
show_logs() {
    log_info "Showing service logs (last 50 lines each):"
    
    services=("api" "postgres" "redis" "mysql" "minio")
    
    for service in "${services[@]}"; do
        echo -e "\n${BLUE}=== $service logs ===${NC}"
        docker-compose logs --tail=50 "$service" 2>/dev/null || log_warning "$service service not available"
    done
}

# Health check for all services
health_check() {
    log_info "Performing health checks..."
    
    # API health check
    if curl -s http://localhost:8080/health | grep -q "healthy"; then
        log_success "API health check passed"
    else
        log_error "API health check failed"
        return 1
    fi
    
    # Database connections
    if docker-compose exec postgres pg_isready -U postgres > /dev/null 2>&1; then
        log_success "PostgreSQL health check passed"
    else
        log_error "PostgreSQL health check failed"
        return 1
    fi
    
    if docker-compose exec redis redis-cli ping | grep -q "PONG"; then
        log_success "Redis health check passed"
    else
        log_error "Redis health check failed"
        return 1
    fi
    
    if docker-compose exec mysql mysqladmin ping -h localhost -u root -prootpassword > /dev/null 2>&1; then
        log_success "MySQL health check passed"
    else
        log_warning "MySQL health check failed (optional service)"
    fi
    
    if curl -s http://localhost:9000/minio/health/live > /dev/null 2>&1; then
        log_success "MinIO health check passed"
    else
        log_warning "MinIO health check failed (optional service)"
    fi
    
    log_success "Health checks completed"
}

# Cleanup on exit
trap cleanup EXIT

# Main execution
main() {
    local run_tests=false
    local run_benchmarks=false
    local show_logs_only=false
    local health_only=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --test)
                run_tests=true
                shift
                ;;
            --benchmark)
                run_benchmarks=true
                shift
                ;;
            --logs)
                show_logs_only=true
                shift
                ;;
            --health)
                health_only=true
                shift
                ;;
            --help)
                echo "Usage: $0 [OPTIONS]"
                echo "Options:"
                echo "  --test       Run integration tests"
                echo "  --benchmark  Run performance benchmarks"
                echo "  --logs       Show service logs only"
                echo "  --health     Run health checks only"
                echo "  --help       Show this help"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    check_dependencies
    
    if [ "$show_logs_only" = true ]; then
        show_logs
        return
    fi
    
    if [ "$health_only" = true ]; then
        health_check
        return
    fi
    
    cleanup
    start_stack
    
    # Give services a moment to stabilize
    sleep 5
    
    health_check
    
    if [ "$run_tests" = true ]; then
        run_tests || exit 1
    fi
    
    if [ "$run_benchmarks" = true ]; then
        run_benchmarks || log_warning "Benchmarks failed"
    fi
    
    # If no specific action requested, run tests by default
    if [ "$run_tests" = false ] && [ "$run_benchmarks" = false ]; then
        run_tests || exit 1
    fi
    
    log_success "Docker Compose integration testing completed successfully"
    
    log_info "Services are running. Access URLs:"
    echo "  - API: http://localhost:8080"
    echo "  - Adminer: http://localhost:8081"
    echo "  - Redis Commander: http://localhost:8082" 
    echo "  - MinIO Console: http://localhost:9001"
    echo ""
    echo "Run 'docker-compose down' to stop services"
}

# Run main function
main "$@"