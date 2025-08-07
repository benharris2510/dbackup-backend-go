-- Initialize PostgreSQL database for development
-- This script creates additional databases and users if needed

-- Create test database
CREATE DATABASE dbackup_test;

-- Create sample data for testing
\c dbackup;

-- Sample users table (will be created by migrations, this is just sample data)
-- INSERT INTO users (email, password_hash, created_at, updated_at) 
-- VALUES ('dev@example.com', '$2a$10$example_hash', NOW(), NOW())
-- ON CONFLICT (email) DO NOTHING;

-- Create additional development user
DO $$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'dbackup_dev') THEN
      CREATE USER dbackup_dev WITH PASSWORD 'dev_password';
      GRANT ALL PRIVILEGES ON DATABASE dbackup TO dbackup_dev;
      GRANT ALL PRIVILEGES ON DATABASE dbackup_test TO dbackup_dev;
   END IF;
END
$$;