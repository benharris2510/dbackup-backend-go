# Database Migration System

This directory contains database migration files for the dbackup backend. The migration system provides versioned database schema management with support for forward and backward migrations.

## Migration System Features

- **Versioned Migrations**: Each migration has a timestamp-based version
- **Rollback Support**: Bidirectional migrations with up/down functions
- **Transaction Safety**: All migrations run within database transactions
- **Batch Tracking**: Groups migrations into batches for better management
- **Error Handling**: Comprehensive error tracking and recovery
- **Checksum Validation**: Ensures migration integrity
- **SQL File Support**: Load migrations from .sql files with special syntax
- **Status Tracking**: Track migration status (pending, applied, failed, rollback)

## Migration File Format

Migration files use a timestamp-based naming convention:
```
YYYYMMDDHHMMSS_migration_name.sql
```

Example: `20240101123456_create_users_table.sql`

### File Structure

```sql
-- +migrate Up
-- Description of what this migration does
-- Created: YYYY-MM-DD HH:MM:SS

-- Your UP migration SQL here
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- +migrate Down
-- Rollback for: Description

-- Your DOWN migration SQL here (optional)
DROP TABLE users;
```

### Important Notes

- The `-- +migrate Up` and `-- +migrate Down` comments are required
- UP migrations are required, DOWN migrations are optional
- SQL statements are automatically split by semicolons
- Each migration runs in its own transaction

## Command Line Interface

The migration system includes a comprehensive CLI tool:

### Available Commands

```bash
# Run pending migrations
go run cmd/migrate/main.go up

# Rollback migrations
go run cmd/migrate/main.go down

# Show migration status
go run cmd/migrate/main.go status

# Create a new migration file
go run cmd/migrate/main.go create -name="add user indexes"

# Reset database (rollback all migrations)
go run cmd/migrate/main.go reset

# Refresh database (reset and rerun all)
go run cmd/migrate/main.go refresh

# Show current version
go run cmd/migrate/main.go version
```

### Command Options

```bash
-config string     Configuration file path (default: config.yaml)
-migrations string Migrations directory path (default: migrations)
-version string    Target migration version
-name string       Migration name for create command
```

### Examples

```bash
# Run all pending migrations
go run cmd/migrate/main.go up

# Migrate to specific version
go run cmd/migrate/main.go up -version=20240101123456

# Rollback to specific version
go run cmd/migrate/main.go down -version=20240101123456

# Create a new migration
go run cmd/migrate/main.go create -name="add user authentication"

# Check migration status
go run cmd/migrate/main.go status
```

## Makefile Shortcuts

For convenience, use the provided Makefile commands:

```bash
# Migration commands
make migrate-up                    # Run pending migrations
make migrate-down                  # Rollback latest migration
make migrate-create name="description"  # Create new migration
make migrate-status                # Show migration status
make migrate-version               # Show current version
make migrate-reset                 # Reset database
make migrate-refresh               # Reset and rerun all migrations
```

## Migration Status

The `status` command shows detailed information about migrations:

```
VERSION        STATUS     NAME                         APPLIED AT           EXECUTION TIME
20240101000001 applied    create initial tables        2024-01-01 12:34:56  145ms
20240101000002 applied    create backup tables         2024-01-01 12:35:12  89ms
20240101000003 pending    add security enhancements   -                    -
```

Status values:
- **pending**: Migration not yet applied
- **applied**: Migration successfully applied
- **failed**: Migration failed during execution
- **rollback**: Migration was rolled back

## Best Practices

### 1. Migration Naming

Use descriptive names that clearly indicate what the migration does:

```bash
# Good
20240101123456_create_users_table.sql
20240101123500_add_email_index_to_users.sql
20240101123600_add_password_reset_columns.sql

# Avoid
20240101123456_migration.sql
20240101123500_update.sql
```

### 2. Incremental Changes

Keep migrations small and focused on a single change:

```sql
-- Good: Single purpose migration
-- +migrate Up
CREATE INDEX idx_users_email ON users(email);

-- Avoid: Multiple unrelated changes in one migration
-- +migrate Up
CREATE INDEX idx_users_email ON users(email);
ALTER TABLE orders ADD COLUMN status VARCHAR(50);
CREATE TABLE logs (id SERIAL PRIMARY KEY);
```

### 3. Backward Compatibility

Write migrations that are backward compatible when possible:

```sql
-- Good: Add column with default value
-- +migrate Up
ALTER TABLE users ADD COLUMN phone VARCHAR(20) DEFAULT '';

-- Risky: Non-nullable column without default
-- +migrate Up
ALTER TABLE users ADD COLUMN phone VARCHAR(20) NOT NULL;
```

### 4. Data Migrations

For data transformations, include both schema and data changes:

```sql
-- +migrate Up
-- Add new column
ALTER TABLE users ADD COLUMN full_name VARCHAR(255);

-- Populate with existing data
UPDATE users SET full_name = CONCAT(first_name, ' ', last_name) 
WHERE first_name IS NOT NULL AND last_name IS NOT NULL;

-- Make column not null after populating
ALTER TABLE users ALTER COLUMN full_name SET NOT NULL;

-- +migrate Down
ALTER TABLE users DROP COLUMN full_name;
```

### 5. Index Management

Create indexes separately from table creation for better control:

```sql
-- +migrate Up
-- Create table
CREATE TABLE user_sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    session_token VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_token ON user_sessions(session_token);
CREATE INDEX idx_user_sessions_created_at ON user_sessions(created_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_user_sessions_created_at;
DROP INDEX IF EXISTS idx_user_sessions_token;
DROP INDEX IF EXISTS idx_user_sessions_user_id;
DROP TABLE user_sessions;
```

## Programmatic Usage

You can also use the migration system programmatically:

```go
package main

import (
    "github.com/dbackup/backend-go/internal/database"
    "gorm.io/gorm"
)

func runMigrations(db *gorm.DB) error {
    // Initialize migration system
    migrationSystem := database.NewMigrationSystem(db, "migrations")
    if err := migrationSystem.Initialize(); err != nil {
        return err
    }

    // Load migrations from directory
    if err := migrationSystem.LoadMigrationsFromDir(); err != nil {
        return err
    }

    // Run pending migrations
    if err := migrationSystem.Up(""); err != nil {
        return err
    }

    return nil
}

// Register migrations programmatically
func registerCustomMigrations(migrationSystem *database.MigrationSystem) error {
    migration := &database.MigrationDefinition{
        Version:     "20240101000001",
        Name:        "custom migration",
        Description: "A programmatically registered migration",
        Up: func(db *gorm.DB) error {
            return db.Exec("CREATE TABLE custom_table (id SERIAL PRIMARY KEY)").Error
        },
        Down: func(db *gorm.DB) error {
            return db.Exec("DROP TABLE custom_table").Error
        },
    }

    return migrationSystem.RegisterMigration(migration)
}
```

## Docker Integration

The migration system works seamlessly with Docker:

```bash
# Run migrations in Docker container
docker-compose exec api go run cmd/migrate/main.go up

# Create migration in container
docker-compose exec api go run cmd/migrate/main.go create -name="new feature"

# Check status
docker-compose exec api go run cmd/migrate/main.go status
```

## Testing

The migration system includes comprehensive tests:

```bash
# Run migration system tests
go test ./internal/database/... -v

# Run specific migration tests
go test ./internal/database/ -run TestMigrationSystem -v

# Benchmark migration performance
go test ./internal/database/ -bench=BenchmarkMigration -benchmem
```

## Production Considerations

### 1. Backup Before Migrations

Always backup your database before running migrations in production:

```bash
# Create backup
pg_dump -h localhost -U postgres dbackup > backup_$(date +%Y%m%d_%H%M%S).sql

# Run migrations
make migrate-up

# If issues occur, restore from backup
psql -h localhost -U postgres dbackup < backup_20240101_123456.sql
```

### 2. Migration Rollback Plan

Have a rollback plan for each migration:

```bash
# If migration fails, rollback to previous version
go run cmd/migrate/main.go down -version=20240101123456
```

### 3. Zero-Downtime Migrations

For zero-downtime deployments:

1. **Additive Changes**: Add columns/tables without removing old ones
2. **Multiple Deployments**: Deploy schema changes first, then code changes
3. **Feature Flags**: Use feature flags to control new functionality
4. **Gradual Rollout**: Test with a subset of users first

### 4. Monitoring

Monitor migration execution in production:

```bash
# Check migration status
go run cmd/migrate/main.go status

# Monitor migration logs
tail -f /var/log/dbackup/migrations.log
```

## Troubleshooting

### Common Issues

1. **Migration Fails Mid-Execution**
   ```bash
   # Check status to see which migration failed
   go run cmd/migrate/main.go status
   
   # Fix the issue and retry
   go run cmd/migrate/main.go up
   ```

2. **Checksum Mismatch**
   ```
   Error: Migration checksum mismatch for version 20240101123456
   ```
   This means the migration file was modified after being applied. This is generally not safe.

3. **Transaction Deadlock**
   ```bash
   # Retry the migration
   go run cmd/migrate/main.go up
   ```

4. **Missing Migration File**
   ```bash
   # Check if all migration files are present
   ls -la migrations/
   ```

### Recovery Procedures

1. **Reset Migration State**
   ```sql
   -- Only if absolutely necessary
   DELETE FROM schema_migrations WHERE version = '20240101123456';
   ```

2. **Manual Migration Rollback**
   ```sql
   -- Manually run the down migration SQL
   DROP TABLE problem_table;
   UPDATE schema_migrations SET status = 'rollback' WHERE version = '20240101123456';
   ```

3. **Force Migration Status**
   ```sql
   -- Mark migration as applied without running it (dangerous)
   UPDATE schema_migrations SET status = 'applied', applied_at = NOW() 
   WHERE version = '20240101123456';
   ```

## Migration History

This directory contains the following migrations:

1. **20240101000001_create_initial_tables.sql** - Creates core tables (users, teams, storage configurations, database connections)
2. **20240101000002_create_backup_tables.sql** - Creates backup-related tables (backup jobs, files, permissions, policies)  
3. **20240101000003_add_security_enhancements.sql** - Adds security features (sessions, API keys, webhooks, audit improvements)

Each migration includes comprehensive up and down scripts with proper indexing and constraints.