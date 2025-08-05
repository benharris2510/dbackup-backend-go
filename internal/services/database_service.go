package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dbackup/backend-go/internal/encryption"
	"github.com/dbackup/backend-go/internal/models"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
)

// DatabaseService provides database connection testing and management
type DatabaseService struct {
	db               *gorm.DB
	encryptionService *encryption.Service
}

// NewDatabaseService creates a new database service instance
func NewDatabaseService(db *gorm.DB, encService *encryption.Service) *DatabaseService {
	return &DatabaseService{
		db:               db,
		encryptionService: encService,
	}
}

// TestConnection tests a database connection and returns detailed results
func (ds *DatabaseService) TestConnection(ctx context.Context, conn *models.DatabaseConnection) (*models.TestConnectionResult, error) {
	startTime := time.Now()
	result := &models.TestConnectionResult{
		Success:      false,
		ResponseTime: 0,
	}

	// Get decrypted password
	password, err := conn.GetDecryptedPassword(ds.encryptionService)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to decrypt credentials: %v", err)
		result.Message = "Invalid connection credentials"
		result.ResponseTime = time.Since(startTime)
		return result, nil
	}

	// Test connection based on database type
	switch conn.Type {
	case models.DatabaseTypePostgreSQL:
		return ds.testPostgreSQLConnection(ctx, conn, password, startTime)
	case models.DatabaseTypeMySQL:
		return ds.testMySQLConnection(ctx, conn, password, startTime)
	case models.DatabaseTypeSQLite:
		return ds.testSQLiteConnection(ctx, conn, startTime)
	default:
		result.Error = fmt.Sprintf("Unsupported database type: %s", conn.Type)
		result.Message = "Database type not supported"
		result.ResponseTime = time.Since(startTime)
		return result, nil
	}
}

// testPostgreSQLConnection tests a PostgreSQL connection
func (ds *DatabaseService) testPostgreSQLConnection(ctx context.Context, conn *models.DatabaseConnection, password string, startTime time.Time) (*models.TestConnectionResult, error) {
	result := &models.TestConnectionResult{
		Success: false,
	}

	// Build connection string
	connStr := ds.buildPostgreSQLConnectionString(conn, password)

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, conn.ConnectionTimeout)
	defer cancel()

	// Open connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to open connection: %v", err)
		result.Message = "Connection configuration error"
		result.ResponseTime = time.Since(startTime)
		return result, nil
	}
	defer db.Close()

	// Set connection limits
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(conn.ConnectionTimeout)

	// Test the connection
	if err := db.PingContext(testCtx); err != nil {
		result.Error = fmt.Sprintf("Connection test failed: %v", err)
		if strings.Contains(err.Error(), "timeout") {
			result.Message = "Connection timeout - check network connectivity"
		} else if strings.Contains(err.Error(), "authentication") {
			result.Message = "Authentication failed - check username and password"
		} else if strings.Contains(err.Error(), "does not exist") {
			result.Message = "Database does not exist"
		} else {
			result.Message = "Connection failed"
		}
		result.ResponseTime = time.Since(startTime)
		return result, nil
	}

	// Get database information
	dbInfo, err := ds.getPostgreSQLInfo(testCtx, db, conn.Database)
	if err != nil {
		// Connection works but we can't get info - still success
		result.Success = true
		result.Message = "Connection successful (limited database information)"
		result.ResponseTime = time.Since(startTime)
		return result, nil
	}

	result.Success = true
	result.Message = "Connection successful"
	result.DatabaseInfo = dbInfo
	result.ResponseTime = time.Since(startTime)
	return result, nil
}

// testMySQLConnection tests a MySQL connection
func (ds *DatabaseService) testMySQLConnection(ctx context.Context, conn *models.DatabaseConnection, password string, startTime time.Time) (*models.TestConnectionResult, error) {
	result := &models.TestConnectionResult{
		Success: false,
	}

	// Build connection string
	connStr := ds.buildMySQLConnectionString(conn, password)

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, conn.ConnectionTimeout)
	defer cancel()

	// Open connection
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to open connection: %v", err)
		result.Message = "Connection configuration error"
		result.ResponseTime = time.Since(startTime)
		return result, nil
	}
	defer db.Close()

	// Set connection limits
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(conn.ConnectionTimeout)

	// Test the connection
	if err := db.PingContext(testCtx); err != nil {
		result.Error = fmt.Sprintf("Connection test failed: %v", err)
		if strings.Contains(err.Error(), "timeout") {
			result.Message = "Connection timeout - check network connectivity"
		} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "Access denied") {
			result.Message = "Authentication failed - check username and password"
		} else if strings.Contains(err.Error(), "Unknown database") {
			result.Message = "Database does not exist"
		} else {
			result.Message = "Connection failed"
		}
		result.ResponseTime = time.Since(startTime)
		return result, nil
	}

	// Get database information
	dbInfo, err := ds.getMySQLInfo(testCtx, db, conn.Database)
	if err != nil {
		// Connection works but we can't get info - still success
		result.Success = true
		result.Message = "Connection successful (limited database information)"
		result.ResponseTime = time.Since(startTime)
		return result, nil
	}

	result.Success = true
	result.Message = "Connection successful"
	result.DatabaseInfo = dbInfo
	result.ResponseTime = time.Since(startTime)
	return result, nil
}

// testSQLiteConnection tests a SQLite connection (placeholder for future implementation)
func (ds *DatabaseService) testSQLiteConnection(ctx context.Context, conn *models.DatabaseConnection, startTime time.Time) (*models.TestConnectionResult, error) {
	result := &models.TestConnectionResult{
		Success:      false,
		Error:        "SQLite support not yet implemented",
		Message:      "SQLite connections are not supported in this version",
		ResponseTime: time.Since(startTime),
	}
	return result, nil
}

// buildPostgreSQLConnectionString builds a PostgreSQL connection string
func (ds *DatabaseService) buildPostgreSQLConnectionString(conn *models.DatabaseConnection, password string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("host=%s", conn.Host))
	parts = append(parts, fmt.Sprintf("port=%d", conn.Port))
	parts = append(parts, fmt.Sprintf("user=%s", conn.Username))
	parts = append(parts, fmt.Sprintf("password=%s", password))
	parts = append(parts, fmt.Sprintf("dbname=%s", conn.Database))

	// SSL configuration
	if conn.SSLEnabled {
		if conn.SSLMode != nil && *conn.SSLMode != "" {
			parts = append(parts, fmt.Sprintf("sslmode=%s", *conn.SSLMode))
		} else {
			parts = append(parts, "sslmode=require")
		}

		// Add SSL certificates if provided
		if conn.SSLCert != nil && *conn.SSLCert != "" {
			// For actual implementation, we'd need to write cert files to temp location
			// For now, just note that SSL cert is configured
		}
	} else {
		parts = append(parts, "sslmode=disable")
	}

	// Connection timeout
	timeoutSeconds := int(conn.ConnectionTimeout.Seconds())
	if timeoutSeconds > 0 {
		parts = append(parts, fmt.Sprintf("connect_timeout=%d", timeoutSeconds))
	}

	return strings.Join(parts, " ")
}

// buildMySQLConnectionString builds a MySQL connection string
func (ds *DatabaseService) buildMySQLConnectionString(conn *models.DatabaseConnection, password string) string {
	// Format: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
	connStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", 
		conn.Username, password, conn.Host, conn.Port, conn.Database)

	var params []string

	// SSL configuration
	if conn.SSLEnabled {
		params = append(params, "tls=true")
	} else {
		params = append(params, "tls=false")
	}

	// Connection timeout
	timeoutSeconds := int(conn.ConnectionTimeout.Seconds())
	if timeoutSeconds > 0 {
		params = append(params, fmt.Sprintf("timeout=%ds", timeoutSeconds))
	}

	// Parse time for better datetime handling
	params = append(params, "parseTime=true")

	// Character set
	params = append(params, "charset=utf8mb4")

	if len(params) > 0 {
		connStr += "?" + strings.Join(params, "&")
	}

	return connStr
}

// getPostgreSQLInfo retrieves PostgreSQL database information
func (ds *DatabaseService) getPostgreSQLInfo(ctx context.Context, db *sql.DB, dbName string) (*models.DatabaseInfoResult, error) {
	info := &models.DatabaseInfoResult{}

	// Get PostgreSQL version
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}
	
	// Extract version number from full version string
	if strings.Contains(version, "PostgreSQL") {
		parts := strings.Fields(version)
		if len(parts) >= 2 {
			info.Version = parts[1]
		} else {
			info.Version = version
		}
	} else {
		info.Version = version
	}

	// Get database size
	var sizeBytes int64
	err = db.QueryRowContext(ctx, 
		"SELECT pg_database_size($1)", dbName).Scan(&sizeBytes)
	if err == nil {
		info.Size = formatBytes(sizeBytes)
	}

	// Get database encoding/charset
	var encoding string
	err = db.QueryRowContext(ctx,
		"SELECT pg_encoding_to_char(encoding) FROM pg_database WHERE datname = $1", 
		dbName).Scan(&encoding)
	if err == nil {
		info.Charset = encoding
	}

	// Get collation
	var collation string
	err = db.QueryRowContext(ctx,
		"SELECT datcollate FROM pg_database WHERE datname = $1", 
		dbName).Scan(&collation)
	if err == nil {
		info.Collation = collation
	}

	return info, nil
}

// getMySQLInfo retrieves MySQL database information
func (ds *DatabaseService) getMySQLInfo(ctx context.Context, db *sql.DB, dbName string) (*models.DatabaseInfoResult, error) {
	info := &models.DatabaseInfoResult{}

	// Get MySQL version
	var version string
	err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}
	
	// Extract version number from full version string
	if strings.Contains(version, "-") {
		parts := strings.Split(version, "-")
		info.Version = parts[0]
	} else {
		info.Version = version
	}

	// Get database size
	var sizeBytes sql.NullInt64
	err = db.QueryRowContext(ctx, `
		SELECT ROUND(SUM(data_length + index_length))
		FROM information_schema.tables
		WHERE table_schema = ?`, dbName).Scan(&sizeBytes)
	if err == nil && sizeBytes.Valid {
		info.Size = formatBytes(sizeBytes.Int64)
	}

	// Get default character set
	var charset sql.NullString
	err = db.QueryRowContext(ctx, `
		SELECT default_character_set_name
		FROM information_schema.schemata
		WHERE schema_name = ?`, dbName).Scan(&charset)
	if err == nil && charset.Valid {
		info.Charset = charset.String
	}

	// Get default collation
	var collation sql.NullString
	err = db.QueryRowContext(ctx, `
		SELECT default_collation_name
		FROM information_schema.schemata
		WHERE schema_name = ?`, dbName).Scan(&collation)
	if err == nil && collation.Valid {
		info.Collation = collation.String
	}

	return info, nil
}

// DiscoverTables discovers tables in a database connection
func (ds *DatabaseService) DiscoverTables(ctx context.Context, conn *models.DatabaseConnection, req *models.TableDiscoveryRequest) ([]models.DatabaseTable, error) {
	// Get decrypted password
	password, err := conn.GetDecryptedPassword(ds.encryptionService)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	switch conn.Type {
	case models.DatabaseTypePostgreSQL:
		return ds.discoverPostgreSQLTables(ctx, conn, password, req)
	case models.DatabaseTypeMySQL:
		return ds.discoverMySQLTables(ctx, conn, password, req)
	case models.DatabaseTypeSQLite:
		return nil, fmt.Errorf("SQLite table discovery not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", conn.Type)
	}
}

// discoverPostgreSQLTables discovers tables in a PostgreSQL database
func (ds *DatabaseService) discoverPostgreSQLTables(ctx context.Context, conn *models.DatabaseConnection, password string, req *models.TableDiscoveryRequest) ([]models.DatabaseTable, error) {
	// Build connection string
	connStr := ds.buildPostgreSQLConnectionString(conn, password)

	// Create context with timeout
	discoverCtx, cancel := context.WithTimeout(ctx, conn.QueryTimeout)
	defer cancel()

	// Open connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	// Set connection limits
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(conn.ConnectionTimeout)

	// Test connection
	if err := db.PingContext(discoverCtx); err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	// Build query for table discovery
	query := ds.buildPostgreSQLTablesQuery(req)
	args := ds.buildPostgreSQLTablesArgs(req)

	// Execute query
	rows, err := db.QueryContext(discoverCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to discover tables: %w", err)
	}
	defer rows.Close()

	var tables []models.DatabaseTable
	now := time.Now()

	for rows.Next() {
		var table models.DatabaseTable
		var comment sql.NullString
		var rowCount, dataLength, indexLength sql.NullInt64

		err := rows.Scan(
			&table.Name,
			&table.Schema,
			&table.Type,
			&comment,
			&rowCount,
			&dataLength,
			&indexLength,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}

		// Set optional fields
		if comment.Valid {
			table.Comment = &comment.String
		}
		if rowCount.Valid {
			table.RowCount = &rowCount.Int64
		}
		if dataLength.Valid {
			table.DataLength = &dataLength.Int64
		}
		if indexLength.Valid {
			table.IndexLength = &indexLength.Int64
		}

		// Set database connection
		table.DatabaseConnectionID = conn.ID
		table.LastDiscoveredAt = now
		
		// Set defaults
		table.IsBackupEnabled = false
		table.BackupPriority = 100
		table.ExcludeFromBackup = false
		table.HasSelectAccess = false
		table.AccessLevel = models.TableAccessNone

		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating table rows: %w", err)
	}

	return tables, nil
}

// discoverMySQLTables discovers tables in a MySQL database
func (ds *DatabaseService) discoverMySQLTables(ctx context.Context, conn *models.DatabaseConnection, password string, req *models.TableDiscoveryRequest) ([]models.DatabaseTable, error) {
	// Build connection string
	connStr := ds.buildMySQLConnectionString(conn, password)

	// Create context with timeout
	discoverCtx, cancel := context.WithTimeout(ctx, conn.QueryTimeout)
	defer cancel()

	// Open connection
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	// Set connection limits
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(conn.ConnectionTimeout)

	// Test connection
	if err := db.PingContext(discoverCtx); err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	// Build query for table discovery
	query := ds.buildMySQLTablesQuery(req)
	args := ds.buildMySQLTablesArgs(conn.Database, req)

	// Execute query
	rows, err := db.QueryContext(discoverCtx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to discover tables: %w", err)
	}
	defer rows.Close()

	var tables []models.DatabaseTable
	now := time.Now()

	for rows.Next() {
		var table models.DatabaseTable
		var comment, engine, collation sql.NullString
		var rowCount, dataLength, indexLength sql.NullInt64

		err := rows.Scan(
			&table.Name,
			&table.Schema,
			&table.Type,
			&comment,
			&engine,
			&collation,
			&rowCount,
			&dataLength,
			&indexLength,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}

		// Set optional fields
		if comment.Valid {
			table.Comment = &comment.String
		}
		if engine.Valid {
			table.Engine = &engine.String
		}
		if collation.Valid {
			table.Collation = &collation.String
		}
		if rowCount.Valid {
			table.RowCount = &rowCount.Int64
		}
		if dataLength.Valid {
			table.DataLength = &dataLength.Int64
		}
		if indexLength.Valid {
			table.IndexLength = &indexLength.Int64
		}

		// Set database connection
		table.DatabaseConnectionID = conn.ID
		table.LastDiscoveredAt = now
		
		// Set defaults
		table.IsBackupEnabled = false
		table.BackupPriority = 100
		table.ExcludeFromBackup = false
		table.HasSelectAccess = false
		table.AccessLevel = models.TableAccessNone

		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating table rows: %w", err)
	}

	return tables, nil
}

// buildPostgreSQLTablesQuery builds the PostgreSQL table discovery query
func (ds *DatabaseService) buildPostgreSQLTablesQuery(req *models.TableDiscoveryRequest) string {
	query := `
		SELECT 
			t.table_name,
			t.table_schema,
			CASE 
				WHEN t.table_type = 'BASE TABLE' THEN 'table'
				WHEN t.table_type = 'VIEW' THEN 'view'
				ELSE LOWER(t.table_type)
			END as table_type,
			obj_description(c.oid) as comment,
			s.n_tup_ins + s.n_tup_upd + s.n_tup_del as row_count,
			pg_total_relation_size(c.oid) as data_length,
			pg_indexes_size(c.oid) as index_length
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		LEFT JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.table_schema
		LEFT JOIN pg_stat_user_tables s ON s.relname = t.table_name AND s.schemaname = t.table_schema
		WHERE t.table_catalog = current_database()`

	var conditions []string
	argIndex := 1

	// Schema pattern filter
	if req.SchemaPattern != nil && *req.SchemaPattern != "" {
		conditions = append(conditions, fmt.Sprintf("t.table_schema LIKE $%d", argIndex))
		argIndex++
	} else {
		// Default: exclude system schemas
		if !req.IncludeSystem {
			conditions = append(conditions, "t.table_schema NOT IN ('information_schema', 'pg_catalog', 'pg_toast', 'pg_temp_1')")
		}
	}

	// Table pattern filter
	if req.TablePattern != nil && *req.TablePattern != "" {
		conditions = append(conditions, fmt.Sprintf("t.table_name LIKE $%d", argIndex))
		argIndex++
	}

	// View inclusion filter
	if !req.IncludeViews {
		conditions = append(conditions, "t.table_type = 'BASE TABLE'")
	}

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY t.table_schema, t.table_name"

	return query
}

// buildPostgreSQLTablesArgs builds the arguments for the PostgreSQL table discovery query
func (ds *DatabaseService) buildPostgreSQLTablesArgs(req *models.TableDiscoveryRequest) []interface{} {
	var args []interface{}

	if req.SchemaPattern != nil && *req.SchemaPattern != "" {
		args = append(args, *req.SchemaPattern)
	}

	if req.TablePattern != nil && *req.TablePattern != "" {
		args = append(args, *req.TablePattern)
	}

	if args == nil {
		return []interface{}{}
	}

	return args
}

// buildMySQLTablesQuery builds the MySQL table discovery query
func (ds *DatabaseService) buildMySQLTablesQuery(req *models.TableDiscoveryRequest) string {
	query := `
		SELECT 
			t.table_name,
			t.table_schema,
			CASE 
				WHEN t.table_type = 'BASE TABLE' THEN 'table'
				WHEN t.table_type = 'VIEW' THEN 'view'
				ELSE LOWER(t.table_type)
			END as table_type,
			t.table_comment,
			t.engine,
			t.table_collation,
			t.table_rows,
			t.data_length,
			t.index_length
		FROM information_schema.tables t
		WHERE t.table_schema = ?`

	var conditions []string
	argIndex := 2

	// Schema pattern filter (MySQL doesn't really use schema patterns like PostgreSQL)
	if req.SchemaPattern != nil && *req.SchemaPattern != "" {
		conditions = append(conditions, fmt.Sprintf("t.table_schema LIKE ?"))
		argIndex++
	}

	// Table pattern filter
	if req.TablePattern != nil && *req.TablePattern != "" {
		conditions = append(conditions, fmt.Sprintf("t.table_name LIKE ?"))
		argIndex++
	}

	// View inclusion filter
	if !req.IncludeViews {
		conditions = append(conditions, "t.table_type = 'BASE TABLE'")
	}

	// System schema exclusion (MySQL doesn't have as many system schemas as PostgreSQL)
	if !req.IncludeSystem {
		conditions = append(conditions, "t.table_schema NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')")
	}

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY t.table_schema, t.table_name"

	return query
}

// buildMySQLTablesArgs builds the arguments for the MySQL table discovery query
func (ds *DatabaseService) buildMySQLTablesArgs(dbName string, req *models.TableDiscoveryRequest) []interface{} {
	var args []interface{}

	// Always start with database name
	args = append(args, dbName)

	if req.SchemaPattern != nil && *req.SchemaPattern != "" {
		args = append(args, *req.SchemaPattern)
	}

	if req.TablePattern != nil && *req.TablePattern != "" {
		args = append(args, *req.TablePattern)
	}

	return args
}

// UpdateConnectionTestResult updates the test result for a database connection
func (ds *DatabaseService) UpdateConnectionTestResult(ctx context.Context, connID uint, result *models.TestConnectionResult) error {
	updates := map[string]interface{}{
		"last_tested_at": time.Now(),
	}

	if result.Success {
		updates["last_test_error"] = nil
	} else {
		updates["last_test_error"] = result.Error
		if result.Message != "" {
			updates["last_test_error"] = result.Message + ": " + result.Error
		}
	}

	return ds.db.WithContext(ctx).Model(&models.DatabaseConnection{}).
		Where("id = ?", connID).
		Updates(updates).Error
}

// SaveDiscoveredTables saves discovered tables to the database
func (ds *DatabaseService) SaveDiscoveredTables(ctx context.Context, tables []models.DatabaseTable) error {
	if len(tables) == 0 {
		return nil
	}

	// Use transaction for bulk insert/update
	return ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range tables {
			// Check if table already exists
			var existing models.DatabaseTable
			err := tx.Where("database_connection_id = ? AND schema = ? AND name = ?", 
				table.DatabaseConnectionID, table.Schema, table.Name).
				First(&existing).Error

			if err == gorm.ErrRecordNotFound {
				// Create new table
				if err := tx.Create(&table).Error; err != nil {
					return fmt.Errorf("failed to create table %s.%s: %w", table.Schema, table.Name, err)
				}
			} else if err == nil {
				// Update existing table statistics
				updates := map[string]interface{}{
					"type":               table.Type,
					"comment":            table.Comment,
					"row_count":          table.RowCount,
					"data_length":        table.DataLength,
					"index_length":       table.IndexLength,
					"last_discovered_at": table.LastDiscoveredAt,
					"discovery_error":    nil, // Clear any previous errors
				}

				if err := tx.Model(&existing).Updates(updates).Error; err != nil {
					return fmt.Errorf("failed to update table %s.%s: %w", table.Schema, table.Name, err)
				}
			} else {
				return fmt.Errorf("failed to check existing table %s.%s: %w", table.Schema, table.Name, err)
			}
		}

		return nil
	})
}

// formatBytes formats bytes to human readable string
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	} else if bytes < 1024*1024*1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	} else {
		return fmt.Sprintf("%.1f TB", float64(bytes)/(1024*1024*1024*1024))
	}
}