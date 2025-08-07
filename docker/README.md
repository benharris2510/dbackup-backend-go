# Docker Setup for dbackup Backend

This directory contains Docker configurations for running the dbackup backend in different environments.

## Quick Start

1. **Development Environment**:
   ```bash
   # Copy environment variables
   cp .env.example .env
   
   # Start all services
   docker-compose up -d
   
   # View logs
   docker-compose logs -f api
   ```

2. **Production Environment**:
   ```bash
   # Set production environment variables
   cp .env.example .env
   # Edit .env with production values
   
   # Start with production overrides
   docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
   ```

## Available Services

### Development Services
- **API**: Go backend API (port 8080)
- **PostgreSQL**: Main database (port 5432)
- **Redis**: Job queue and caching (port 6379)
- **MinIO**: S3-compatible storage (port 9000, console 9001)
- **MySQL**: Test database for MySQL backups (port 3306)
- **Adminer**: Database administration UI (port 8081)
- **Redis Commander**: Redis administration UI (port 8082)
- **pgAdmin**: PostgreSQL administration UI (port 8083)
- **MailHog**: Email testing UI (port 8025)

### Production Services (additional)
- **NGINX**: Reverse proxy and load balancer (ports 80, 443)
- **Prometheus**: Metrics collection (port 9090)
- **Grafana**: Monitoring dashboards (port 3000)

## Service URLs

### Development
- API: http://localhost:8080
- MinIO Console: http://localhost:9001
- Adminer: http://localhost:8081
- Redis Commander: http://localhost:8082
- pgAdmin: http://localhost:8083
- MailHog: http://localhost:8025

### Production
- API: http://localhost (through NGINX)
- Grafana: http://localhost:3000
- Prometheus: http://localhost:9090

## Docker Compose Files

- `docker-compose.yml`: Base configuration for all environments
- `docker-compose.override.yml`: Development-specific overrides (auto-loaded)
- `docker-compose.prod.yml`: Production-specific configuration

## Dockerfiles

- `Dockerfile`: Production multi-stage build
- `Dockerfile.dev`: Development build with hot reload support

## Database Initialization

Both PostgreSQL and MySQL containers are configured with initialization scripts:
- `scripts/init-db.sql`: PostgreSQL setup with test data
- `scripts/init-mysql.sql`: MySQL setup with sample tables

## Development Workflow

1. **Start Development Environment**:
   ```bash
   docker-compose up -d
   ```

2. **View API Logs**:
   ```bash
   docker-compose logs -f api
   ```

3. **Run Database Migrations**:
   ```bash
   docker-compose exec api go run cmd/migrate/main.go up
   ```

4. **Access Database**:
   ```bash
   # PostgreSQL
   docker-compose exec postgres psql -U postgres -d dbackup
   
   # MySQL
   docker-compose exec mysql mysql -u root -p testdb
   ```

5. **Hot Reload**: The development container uses Air for automatic reloading when Go code changes.

## Production Deployment

1. **Environment Variables**: Set all required production environment variables in `.env`

2. **SSL Certificates**: Place SSL certificates in the `nginx/ssl/` directory

3. **NGINX Configuration**: Update `nginx/nginx.conf` with your domain

4. **Start Production Stack**:
   ```bash
   docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
   ```

5. **Monitoring**: Access Grafana at http://localhost:3000 (admin/admin by default)

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure ports 5432, 6379, 8080, 9000, 3306, 8081, 8082, 8083 are not in use

2. **Permission Issues**: 
   ```bash
   # Fix volume permissions
   docker-compose down -v
   docker volume prune
   docker-compose up -d
   ```

3. **Database Connection Issues**: Check that services are healthy:
   ```bash
   docker-compose ps
   docker-compose logs postgres
   ```

4. **Hot Reload Not Working**: Ensure the source code is properly mounted:
   ```bash
   docker-compose exec api ls -la /app
   ```

### Health Checks

All services include health checks. Check service status:
```bash
docker-compose ps
```

### Logs

View logs for specific services:
```bash
docker-compose logs -f api
docker-compose logs -f postgres
docker-compose logs -f redis
```

## Data Persistence

All data is persisted in Docker volumes:
- `postgres_data`: PostgreSQL database files
- `redis_data`: Redis persistence files
- `minio_data`: MinIO object storage
- `mysql_data`: MySQL database files
- `go_mod_cache`: Go module cache for faster builds

## Security Considerations

### Development
- Default passwords are used for convenience
- Services expose ports for easy access
- Debug ports are available

### Production
- Change all default passwords in `.env`
- Services don't expose unnecessary ports
- SSL/TLS encryption enabled
- Resource limits applied
- Non-root user containers

## Backup Testing

The setup includes sample databases for testing backup functionality:
- PostgreSQL with development schema
- MySQL with test tables and data
- MinIO as S3-compatible storage target

Test backup operations:
```bash
# Create a test backup
curl -X POST http://localhost:8080/api/backups \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d '{"database_id": "your-db-id", "tables": ["users", "orders"]}'
```

## Configuration Files

### Required for Production
- `nginx/nginx.conf`: NGINX configuration
- `nginx/ssl/`: SSL certificate files
- `prometheus/prometheus.yml`: Prometheus configuration
- `grafana/provisioning/`: Grafana dashboards and datasources

Create these directories and files before running production deployment.