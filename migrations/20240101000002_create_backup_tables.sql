-- +migrate Up
-- Create backup-related tables
-- Created: 2024-01-01 00:00:02

-- Backup jobs table
CREATE TABLE IF NOT EXISTS backup_jobs (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    connection_id INTEGER NOT NULL REFERENCES database_connections(id) ON DELETE CASCADE,
    storage_config_id INTEGER NOT NULL REFERENCES storage_configurations(id) ON DELETE CASCADE,
    initiated_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_type VARCHAR(50) NOT NULL DEFAULT 'manual',
    backup_type VARCHAR(50) NOT NULL DEFAULT 'full',
    status VARCHAR(50) DEFAULT 'pending',
    tables_to_backup TEXT[],
    backup_options JSON,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    failed_at TIMESTAMP,
    error_message TEXT,
    total_size_bytes BIGINT DEFAULT 0,
    compressed_size_bytes BIGINT DEFAULT 0,
    rows_backed_up BIGINT DEFAULT 0,
    progress_percentage SMALLINT DEFAULT 0,
    worker_id VARCHAR(255),
    priority INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    scheduled_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Backup files table
CREATE TABLE IF NOT EXISTS backup_files (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    backup_job_id INTEGER NOT NULL REFERENCES backup_jobs(id) ON DELETE CASCADE,
    storage_config_id INTEGER NOT NULL REFERENCES storage_configurations(id) ON DELETE CASCADE,
    file_path VARCHAR(1000) NOT NULL,
    file_name VARCHAR(500) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    compressed_size_bytes BIGINT DEFAULT 0,
    checksum_md5 VARCHAR(32),
    checksum_sha256 VARCHAR(64),
    compression_type VARCHAR(50),
    encryption_type VARCHAR(50),
    metadata JSON,
    is_encrypted BOOLEAN DEFAULT false,
    upload_started_at TIMESTAMP,
    upload_completed_at TIMESTAMP,
    retention_until TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Table permissions table
CREATE TABLE IF NOT EXISTS table_permissions (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    database_table_id INTEGER NOT NULL REFERENCES database_tables(id) ON DELETE CASCADE,
    permission_type VARCHAR(50) NOT NULL DEFAULT 'read',
    can_backup BOOLEAN DEFAULT false,
    can_restore BOOLEAN DEFAULT false,
    can_download BOOLEAN DEFAULT false,
    granted_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, database_table_id)
);

-- Backup policies table for scheduled backups
CREATE TABLE IF NOT EXISTS backup_policies (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    connection_id INTEGER NOT NULL REFERENCES database_connections(id) ON DELETE CASCADE,
    storage_config_id INTEGER NOT NULL REFERENCES storage_configurations(id) ON DELETE CASCADE,
    created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    cron_expression VARCHAR(100) NOT NULL,
    timezone VARCHAR(50) DEFAULT 'UTC',
    backup_type VARCHAR(50) NOT NULL DEFAULT 'full',
    tables_to_backup TEXT[],
    backup_options JSON,
    retention_days INTEGER DEFAULT 30,
    is_active BOOLEAN DEFAULT true,
    last_run_at TIMESTAMP,
    next_run_at TIMESTAMP,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audit logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(36) UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    team_id INTEGER REFERENCES teams(id) ON DELETE SET NULL,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(36),
    old_values JSON,
    new_values JSON,
    metadata JSON,
    ip_address INET,
    user_agent TEXT,
    success BOOLEAN DEFAULT true,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for backup tables
CREATE INDEX IF NOT EXISTS idx_backup_jobs_team_id ON backup_jobs(team_id);
CREATE INDEX IF NOT EXISTS idx_backup_jobs_connection_id ON backup_jobs(connection_id);
CREATE INDEX IF NOT EXISTS idx_backup_jobs_status ON backup_jobs(status);
CREATE INDEX IF NOT EXISTS idx_backup_jobs_created_at ON backup_jobs(created_at);
CREATE INDEX IF NOT EXISTS idx_backup_jobs_uid ON backup_jobs(uid);
CREATE INDEX IF NOT EXISTS idx_backup_jobs_scheduled_at ON backup_jobs(scheduled_at);

CREATE INDEX IF NOT EXISTS idx_backup_files_job_id ON backup_files(backup_job_id);
CREATE INDEX IF NOT EXISTS idx_backup_files_storage_config_id ON backup_files(storage_config_id);
CREATE INDEX IF NOT EXISTS idx_backup_files_uid ON backup_files(uid);
CREATE INDEX IF NOT EXISTS idx_backup_files_retention_until ON backup_files(retention_until);

CREATE INDEX IF NOT EXISTS idx_table_permissions_user_id ON table_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_table_permissions_database_table_id ON table_permissions(database_table_id);
CREATE INDEX IF NOT EXISTS idx_table_permissions_team_id ON table_permissions(team_id);
CREATE INDEX IF NOT EXISTS idx_table_permissions_uid ON table_permissions(uid);

CREATE INDEX IF NOT EXISTS idx_backup_policies_team_id ON backup_policies(team_id);
CREATE INDEX IF NOT EXISTS idx_backup_policies_connection_id ON backup_policies(connection_id);
CREATE INDEX IF NOT EXISTS idx_backup_policies_next_run_at ON backup_policies(next_run_at);
CREATE INDEX IF NOT EXISTS idx_backup_policies_uid ON backup_policies(uid);

CREATE INDEX IF NOT EXISTS idx_audit_logs_team_id ON audit_logs(team_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_uid ON audit_logs(uid);

-- +migrate Down
-- Rollback for: Create backup-related tables

-- Drop indexes
DROP INDEX IF EXISTS idx_audit_logs_uid;
DROP INDEX IF EXISTS idx_audit_logs_created_at;
DROP INDEX IF EXISTS idx_audit_logs_resource_type;
DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_user_id;
DROP INDEX IF EXISTS idx_audit_logs_team_id;
DROP INDEX IF EXISTS idx_backup_policies_uid;
DROP INDEX IF EXISTS idx_backup_policies_next_run_at;
DROP INDEX IF EXISTS idx_backup_policies_connection_id;
DROP INDEX IF EXISTS idx_backup_policies_team_id;
DROP INDEX IF EXISTS idx_table_permissions_uid;
DROP INDEX IF EXISTS idx_table_permissions_team_id;
DROP INDEX IF EXISTS idx_table_permissions_database_table_id;
DROP INDEX IF EXISTS idx_table_permissions_user_id;
DROP INDEX IF EXISTS idx_backup_files_retention_until;
DROP INDEX IF EXISTS idx_backup_files_uid;
DROP INDEX IF EXISTS idx_backup_files_storage_config_id;
DROP INDEX IF EXISTS idx_backup_files_job_id;
DROP INDEX IF EXISTS idx_backup_jobs_scheduled_at;
DROP INDEX IF EXISTS idx_backup_jobs_uid;
DROP INDEX IF EXISTS idx_backup_jobs_created_at;
DROP INDEX IF EXISTS idx_backup_jobs_status;
DROP INDEX IF EXISTS idx_backup_jobs_connection_id;
DROP INDEX IF EXISTS idx_backup_jobs_team_id;

-- Drop tables in reverse order
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS backup_policies;
DROP TABLE IF EXISTS table_permissions;
DROP TABLE IF EXISTS backup_files;
DROP TABLE IF EXISTS backup_jobs;