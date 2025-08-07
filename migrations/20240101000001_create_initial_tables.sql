-- +migrate Up
-- Create initial database tables
-- Created: 2024-01-01 00:00:01

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    is_active BOOLEAN DEFAULT true,
    is_verified BOOLEAN DEFAULT false,
    verification_token VARCHAR(255),
    reset_token VARCHAR(255),
    reset_expires_at TIMESTAMP,
    totp_secret VARCHAR(255),
    totp_enabled BOOLEAN DEFAULT false,
    backup_codes JSON,
    last_login_at TIMESTAMP,
    last_login_ip INET,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Teams table
CREATE TABLE IF NOT EXISTS teams (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Team members table
CREATE TABLE IF NOT EXISTS team_members (
    id SERIAL PRIMARY KEY,
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) DEFAULT 'member',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(team_id, user_id)
);

-- Storage configurations table
CREATE TABLE IF NOT EXISTS storage_configurations (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    endpoint VARCHAR(500),
    region VARCHAR(100),
    bucket VARCHAR(255) NOT NULL,
    access_key_encrypted BYTEA,
    secret_key_encrypted BYTEA,
    encryption_key_id VARCHAR(255),
    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Database connections table
CREATE TABLE IF NOT EXISTS database_connections (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    database_type VARCHAR(50) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL,
    database_name VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL,
    password_encrypted BYTEA NOT NULL,
    ssl_mode VARCHAR(50) DEFAULT 'prefer',
    ssl_cert BYTEA,
    ssl_key BYTEA,
    ssl_ca BYTEA,
    connection_params JSON,
    encryption_key_id VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    last_tested_at TIMESTAMP,
    test_result VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Database tables metadata
CREATE TABLE IF NOT EXISTS database_tables (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    connection_id INTEGER NOT NULL REFERENCES database_connections(id) ON DELETE CASCADE,
    schema_name VARCHAR(255) NOT NULL,
    table_name VARCHAR(255) NOT NULL,
    table_type VARCHAR(50),
    row_count BIGINT DEFAULT 0,
    size_bytes BIGINT DEFAULT 0,
    last_analyzed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(connection_id, schema_name, table_name)
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_uid ON users(uid);
CREATE INDEX IF NOT EXISTS idx_users_verification_token ON users(verification_token);
CREATE INDEX IF NOT EXISTS idx_users_reset_token ON users(reset_token);

CREATE INDEX IF NOT EXISTS idx_teams_owner_id ON teams(owner_id);
CREATE INDEX IF NOT EXISTS idx_teams_uid ON teams(uid);

CREATE INDEX IF NOT EXISTS idx_team_members_team_id ON team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user_id ON team_members(user_id);

CREATE INDEX IF NOT EXISTS idx_storage_configs_team_id ON storage_configurations(team_id);
CREATE INDEX IF NOT EXISTS idx_storage_configs_uid ON storage_configurations(uid);

CREATE INDEX IF NOT EXISTS idx_db_connections_team_id ON database_connections(team_id);
CREATE INDEX IF NOT EXISTS idx_db_connections_uid ON database_connections(uid);

CREATE INDEX IF NOT EXISTS idx_db_tables_connection_id ON database_tables(connection_id);
CREATE INDEX IF NOT EXISTS idx_db_tables_uid ON database_tables(uid);

-- +migrate Down
-- Rollback for: Create initial database tables

-- Drop indexes
DROP INDEX IF EXISTS idx_db_tables_uid;
DROP INDEX IF EXISTS idx_db_tables_connection_id;
DROP INDEX IF EXISTS idx_db_connections_uid;
DROP INDEX IF EXISTS idx_db_connections_team_id;
DROP INDEX IF EXISTS idx_storage_configs_uid;
DROP INDEX IF EXISTS idx_storage_configs_team_id;
DROP INDEX IF EXISTS idx_team_members_user_id;
DROP INDEX IF EXISTS idx_team_members_team_id;
DROP INDEX IF EXISTS idx_teams_uid;
DROP INDEX IF EXISTS idx_teams_owner_id;
DROP INDEX IF EXISTS idx_users_reset_token;
DROP INDEX IF EXISTS idx_users_verification_token;
DROP INDEX IF EXISTS idx_users_uid;
DROP INDEX IF EXISTS idx_users_email;

-- Drop tables in reverse order
DROP TABLE IF EXISTS database_tables;
DROP TABLE IF EXISTS database_connections;
DROP TABLE IF EXISTS storage_configurations;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS users;