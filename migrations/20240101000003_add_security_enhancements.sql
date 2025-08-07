-- +migrate Up
-- Add security enhancements and constraints
-- Created: 2024-01-01 00:00:03

-- Add security constraints and triggers

-- Update users table with security enhancements
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS require_password_change BOOLEAN DEFAULT false;

-- Add session tracking for users
CREATE TABLE IF NOT EXISTS user_sessions (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    refresh_token VARCHAR(255) UNIQUE,
    ip_address INET NOT NULL,
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    last_accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add API keys table for programmatic access
CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    key_prefix VARCHAR(10) NOT NULL,
    permissions JSON,
    last_used_at TIMESTAMP,
    usage_count BIGINT DEFAULT 0,
    expires_at TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add webhooks table for event notifications
CREATE TABLE IF NOT EXISTS webhooks (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(1000) NOT NULL,
    secret VARCHAR(255),
    events TEXT[] NOT NULL,
    headers JSON,
    timeout_seconds INTEGER DEFAULT 30,
    retry_count INTEGER DEFAULT 3,
    is_active BOOLEAN DEFAULT true,
    last_triggered_at TIMESTAMP,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add webhook deliveries table for tracking
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    webhook_id INTEGER NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    payload JSON NOT NULL,
    response_status INTEGER,
    response_body TEXT,
    response_headers JSON,
    attempt_count INTEGER DEFAULT 1,
    delivered_at TIMESTAMP,
    failed_at TIMESTAMP,
    next_retry_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add password history table to prevent reuse
CREATE TABLE IF NOT EXISTS password_history (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create function to update timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add updated_at triggers to tables that need them
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_teams_updated_at BEFORE UPDATE ON teams 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_storage_configurations_updated_at BEFORE UPDATE ON storage_configurations 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_database_connections_updated_at BEFORE UPDATE ON database_connections 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_database_tables_updated_at BEFORE UPDATE ON database_tables 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_backup_jobs_updated_at BEFORE UPDATE ON backup_jobs 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_backup_files_updated_at BEFORE UPDATE ON backup_files 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_table_permissions_updated_at BEFORE UPDATE ON table_permissions 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_backup_policies_updated_at BEFORE UPDATE ON backup_policies 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_sessions_updated_at BEFORE UPDATE ON user_sessions 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_webhooks_updated_at BEFORE UPDATE ON webhooks 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create security indexes
CREATE INDEX IF NOT EXISTS idx_users_failed_login_attempts ON users(failed_login_attempts);
CREATE INDEX IF NOT EXISTS idx_users_locked_until ON users(locked_until);
CREATE INDEX IF NOT EXISTS idx_users_password_changed_at ON users(password_changed_at);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_session_token ON user_sessions(session_token);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_sessions_uid ON user_sessions(uid);

CREATE INDEX IF NOT EXISTS idx_api_keys_team_id ON api_keys(team_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_uid ON api_keys(uid);

CREATE INDEX IF NOT EXISTS idx_webhooks_team_id ON webhooks(team_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_uid ON webhooks(uid);
CREATE INDEX IF NOT EXISTS idx_webhooks_is_active ON webhooks(is_active);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_event_type ON webhook_deliveries(event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry_at ON webhook_deliveries(next_retry_at);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_uid ON webhook_deliveries(uid);

CREATE INDEX IF NOT EXISTS idx_password_history_user_id ON password_history(user_id);

-- Add check constraints for data integrity
ALTER TABLE users ADD CONSTRAINT check_email_format 
    CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');

ALTER TABLE backup_jobs ADD CONSTRAINT check_progress_percentage 
    CHECK (progress_percentage >= 0 AND progress_percentage <= 100);

ALTER TABLE backup_jobs ADD CONSTRAINT check_retry_count 
    CHECK (retry_count >= 0 AND retry_count <= max_retries);

ALTER TABLE table_permissions ADD CONSTRAINT check_expires_at_future 
    CHECK (expires_at IS NULL OR expires_at > created_at);

ALTER TABLE user_sessions ADD CONSTRAINT check_expires_at_future_session 
    CHECK (expires_at > created_at);

ALTER TABLE api_keys ADD CONSTRAINT check_expires_at_future_api 
    CHECK (expires_at IS NULL OR expires_at > created_at);

-- +migrate Down
-- Rollback for: Add security enhancements and constraints

-- Drop check constraints
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS check_expires_at_future_api;
ALTER TABLE user_sessions DROP CONSTRAINT IF EXISTS check_expires_at_future_session;
ALTER TABLE table_permissions DROP CONSTRAINT IF EXISTS check_expires_at_future;
ALTER TABLE backup_jobs DROP CONSTRAINT IF EXISTS check_retry_count;
ALTER TABLE backup_jobs DROP CONSTRAINT IF EXISTS check_progress_percentage;
ALTER TABLE users DROP CONSTRAINT IF EXISTS check_email_format;

-- Drop indexes
DROP INDEX IF EXISTS idx_password_history_user_id;
DROP INDEX IF EXISTS idx_webhook_deliveries_uid;
DROP INDEX IF EXISTS idx_webhook_deliveries_next_retry_at;
DROP INDEX IF EXISTS idx_webhook_deliveries_event_type;
DROP INDEX IF EXISTS idx_webhook_deliveries_webhook_id;
DROP INDEX IF EXISTS idx_webhooks_is_active;
DROP INDEX IF EXISTS idx_webhooks_uid;
DROP INDEX IF EXISTS idx_webhooks_team_id;
DROP INDEX IF EXISTS idx_api_keys_uid;
DROP INDEX IF EXISTS idx_api_keys_expires_at;
DROP INDEX IF EXISTS idx_api_keys_key_hash;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_api_keys_team_id;
DROP INDEX IF EXISTS idx_user_sessions_uid;
DROP INDEX IF EXISTS idx_user_sessions_expires_at;
DROP INDEX IF EXISTS idx_user_sessions_session_token;
DROP INDEX IF EXISTS idx_user_sessions_user_id;
DROP INDEX IF EXISTS idx_users_password_changed_at;
DROP INDEX IF EXISTS idx_users_locked_until;
DROP INDEX IF EXISTS idx_users_failed_login_attempts;

-- Drop triggers
DROP TRIGGER IF EXISTS update_webhooks_updated_at ON webhooks;
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS update_user_sessions_updated_at ON user_sessions;
DROP TRIGGER IF EXISTS update_backup_policies_updated_at ON backup_policies;
DROP TRIGGER IF EXISTS update_table_permissions_updated_at ON table_permissions;
DROP TRIGGER IF EXISTS update_backup_files_updated_at ON backup_files;
DROP TRIGGER IF EXISTS update_backup_jobs_updated_at ON backup_jobs;
DROP TRIGGER IF EXISTS update_database_tables_updated_at ON database_tables;
DROP TRIGGER IF EXISTS update_database_connections_updated_at ON database_connections;
DROP TRIGGER IF EXISTS update_storage_configurations_updated_at ON storage_configurations;
DROP TRIGGER IF EXISTS update_teams_updated_at ON teams;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order
DROP TABLE IF EXISTS password_history;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS user_sessions;

-- Remove columns from users table
ALTER TABLE users DROP COLUMN IF EXISTS require_password_change;
ALTER TABLE users DROP COLUMN IF EXISTS password_changed_at;
ALTER TABLE users DROP COLUMN IF EXISTS locked_until;
ALTER TABLE users DROP COLUMN IF EXISTS failed_login_attempts;