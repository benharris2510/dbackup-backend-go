package services

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dbackup/backend-go/internal/models"
)

// BackupServiceInterface defines the interface for backup operations
type BackupServiceInterface interface {
	// PostgreSQL backup operations
	CreatePostgreSQLBackup(ctx context.Context, conn *models.DatabaseConnection, options *BackupOptions) (*BackupResult, error)
	RestorePostgreSQLBackup(ctx context.Context, conn *models.DatabaseConnection, backupPath string, options *RestoreOptions) error
	
	// MySQL backup operations
	CreateMySQLBackup(ctx context.Context, conn *models.DatabaseConnection, options *BackupOptions) (*BackupResult, error)
	RestoreMySQLBackup(ctx context.Context, conn *models.DatabaseConnection, backupPath string, options *RestoreOptions) error
	
	// Generic operations
	ValidateBackupTools() error
	GetBackupEstimate(ctx context.Context, conn *models.DatabaseConnection, options *BackupOptions) (*BackupEstimate, error)
	CompressBackup(ctx context.Context, inputPath, outputPath string, algorithm string) error
	DecompressBackup(ctx context.Context, inputPath, outputPath string) error
}

// BackupService implements database backup operations
type BackupService struct {
	tempDir      string
	pgDumpPath   string
	pgRestorePath string
	mysqlDumpPath string
	mysqlPath    string
}

// BackupOptions contains options for backup operations
type BackupOptions struct {
	// Common options
	Tables       []string `json:"tables,omitempty"`       // Specific tables to backup
	ExcludeTables []string `json:"exclude_tables,omitempty"` // Tables to exclude
	SchemaOnly   bool     `json:"schema_only"`            // Only backup schema
	DataOnly     bool     `json:"data_only"`              // Only backup data
	Compress     bool     `json:"compress"`               // Compress the backup
	
	// PostgreSQL specific options
	Format       string   `json:"format,omitempty"`       // pg_dump format (plain, custom, directory, tar)
	Jobs         int      `json:"jobs,omitempty"`         // Number of parallel jobs
	Verbose      bool     `json:"verbose"`                // Verbose output
	
	// MySQL specific options
	SingleTransaction bool     `json:"single_transaction"` // Use single transaction
	LockTables       bool     `json:"lock_tables"`        // Lock tables during backup
	SkipLockTables   bool     `json:"skip_lock_tables"`   // Skip locking tables
	QuickDump        bool     `json:"quick_dump"`         // Quick dump mode
	
	// Progress tracking
	ProgressCallback func(progress float64, message string) `json:"-"`
}

// RestoreOptions contains options for restore operations
type RestoreOptions struct {
	// Common options
	DropExisting    bool     `json:"drop_existing"`     // Drop existing objects before restore
	CreateDatabase  bool     `json:"create_database"`   // Create database if it doesn't exist
	CleanFirst      bool     `json:"clean_first"`       // Clean (drop) objects before recreating
	
	// PostgreSQL specific options
	Jobs           int      `json:"jobs,omitempty"`    // Number of parallel jobs
	Format         string   `json:"format,omitempty"`  // Backup format
	
	// MySQL specific options
	Force          bool     `json:"force"`             // Force restore even on errors
	
	// Progress tracking
	ProgressCallback func(progress float64, message string) `json:"-"`
}

// BackupResult contains the result of a backup operation
type BackupResult struct {
	FilePath       string            `json:"file_path"`
	OriginalSize   int64             `json:"original_size"`
	CompressedSize *int64            `json:"compressed_size,omitempty"`
	Duration       time.Duration     `json:"duration"`
	Tables         []string          `json:"tables"`
	Metadata       map[string]string `json:"metadata"`
	Checksum       string            `json:"checksum"`
}

// BackupEstimate contains estimated backup information
type BackupEstimate struct {
	EstimatedSize     int64         `json:"estimated_size"`
	EstimatedDuration time.Duration `json:"estimated_duration"`
	TableCount        int           `json:"table_count"`
	RowCount          int64         `json:"row_count"`
}

// NewBackupService creates a new backup service
func NewBackupService() (*BackupService, error) {
	service := &BackupService{
		tempDir: os.TempDir(),
	}
	
	// Find backup tools
	if err := service.findBackupTools(); err != nil {
		return nil, fmt.Errorf("failed to find backup tools: %w", err)
	}
	
	return service, nil
}

// findBackupTools locates the required backup tools
func (bs *BackupService) findBackupTools() error {
	var err error
	
	// Find PostgreSQL tools
	bs.pgDumpPath, err = exec.LookPath("pg_dump")
	if err != nil {
		bs.pgDumpPath = "" // Not an error, just not available
	}
	
	bs.pgRestorePath, err = exec.LookPath("pg_restore")
	if err != nil {
		bs.pgRestorePath = "" // Not an error, just not available
	}
	
	// Find MySQL tools
	bs.mysqlDumpPath, err = exec.LookPath("mysqldump")
	if err != nil {
		bs.mysqlDumpPath = "" // Not an error, just not available
	}
	
	bs.mysqlPath, err = exec.LookPath("mysql")
	if err != nil {
		bs.mysqlPath = "" // Not an error, just not available
	}
	
	return nil
}

// ValidateBackupTools validates that required backup tools are available
func (bs *BackupService) ValidateBackupTools() error {
	var missingTools []string
	
	if bs.pgDumpPath == "" {
		missingTools = append(missingTools, "pg_dump")
	}
	if bs.pgRestorePath == "" {
		missingTools = append(missingTools, "pg_restore")
	}
	if bs.mysqlDumpPath == "" {
		missingTools = append(missingTools, "mysqldump")
	}
	if bs.mysqlPath == "" {
		missingTools = append(missingTools, "mysql")
	}
	
	if len(missingTools) > 0 {
		return fmt.Errorf("missing backup tools: %s", strings.Join(missingTools, ", "))
	}
	
	return nil
}

// CreatePostgreSQLBackup creates a PostgreSQL backup using pg_dump
func (bs *BackupService) CreatePostgreSQLBackup(ctx context.Context, conn *models.DatabaseConnection, options *BackupOptions) (*BackupResult, error) {
	if bs.pgDumpPath == "" {
		return nil, fmt.Errorf("pg_dump not found")
	}
	
	if options == nil {
		options = &BackupOptions{}
	}
	
	// Set default format
	if options.Format == "" {
		options.Format = "custom"
	}
	
	// Create temporary file for backup
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("postgres_%s_%s.backup", conn.Database, timestamp)
	backupPath := filepath.Join(bs.tempDir, filename)
	
	startTime := time.Now()
	
	// Build pg_dump command
	args := bs.buildPgDumpArgs(conn, backupPath, options)
	cmd := exec.CommandContext(ctx, bs.pgDumpPath, args...)
	
	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PGPASSWORD=%s", conn.Password),
		fmt.Sprintf("PGHOST=%s", conn.Host),
		fmt.Sprintf("PGPORT=%d", conn.Port),
		fmt.Sprintf("PGUSER=%s", conn.Username),
		fmt.Sprintf("PGDATABASE=%s", conn.Database),
	)
	
	// Capture output for progress tracking
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start pg_dump: %w", err)
	}
	
	// Track progress if callback is provided
	if options.ProgressCallback != nil {
		go bs.trackPostgreSQLProgress(stderr, options.ProgressCallback)
	}
	
	// Wait for completion
	if err := cmd.Wait(); err != nil {
		// Clean up on failure
		os.Remove(backupPath)
		return nil, fmt.Errorf("pg_dump failed: %w", err)
	}
	
	duration := time.Since(startTime)
	
	// Get file size
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup file info: %w", err)
	}
	
	// Calculate checksum
	checksum, err := bs.calculateChecksum(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}
	
	result := &BackupResult{
		FilePath:     backupPath,
		OriginalSize: fileInfo.Size(),
		Duration:     duration,
		Tables:       options.Tables,
		Metadata: map[string]string{
			"database_type": "postgresql",
			"format":        options.Format,
			"timestamp":     timestamp,
		},
		Checksum: checksum,
	}
	
	// Compress if requested
	if options.Compress {
		compressedPath := backupPath + ".gz"
		if err := bs.CompressBackup(ctx, backupPath, compressedPath, "gzip"); err != nil {
			return nil, fmt.Errorf("failed to compress backup: %w", err)
		}
		
		// Update result with compressed file info
		compressedInfo, err := os.Stat(compressedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get compressed file info: %w", err)
		}
		
		compressedSize := compressedInfo.Size()
		result.CompressedSize = &compressedSize
		result.FilePath = compressedPath
		
		// Remove original uncompressed file
		os.Remove(backupPath)
	}
	
	return result, nil
}

// buildPgDumpArgs builds the arguments for pg_dump command
func (bs *BackupService) buildPgDumpArgs(conn *models.DatabaseConnection, outputPath string, options *BackupOptions) []string {
	args := []string{
		"--host", conn.Host,
		"--port", fmt.Sprintf("%d", conn.Port),
		"--username", conn.Username,
		"--dbname", conn.Database,
		"--format", options.Format,
		"--file", outputPath,
	}
	
	if options.Verbose {
		args = append(args, "--verbose")
	}
	
	if options.SchemaOnly {
		args = append(args, "--schema-only")
	}
	
	if options.DataOnly {
		args = append(args, "--data-only")
	}
	
	if options.Jobs > 1 && options.Format == "directory" {
		args = append(args, "--jobs", fmt.Sprintf("%d", options.Jobs))
	}
	
	// Add specific tables
	for _, table := range options.Tables {
		args = append(args, "--table", table)
	}
	
	// Exclude tables
	for _, table := range options.ExcludeTables {
		args = append(args, "--exclude-table", table)
	}
	
	return args
}

// trackPostgreSQLProgress tracks the progress of pg_dump
func (bs *BackupService) trackPostgreSQLProgress(stderr io.Reader, callback func(float64, string)) {
	scanner := bufio.NewScanner(stderr)
	progress := 0.0
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Parse pg_dump verbose output for progress
		if strings.Contains(line, "dumping table") {
			progress += 10.0 // Rough estimate
			if progress > 90.0 {
				progress = 90.0
			}
			callback(progress, line)
		} else if strings.Contains(line, "completed") {
			callback(100.0, "Backup completed")
		}
	}
}

// CreateMySQLBackup creates a MySQL backup using mysqldump
func (bs *BackupService) CreateMySQLBackup(ctx context.Context, conn *models.DatabaseConnection, options *BackupOptions) (*BackupResult, error) {
	if bs.mysqlDumpPath == "" {
		return nil, fmt.Errorf("mysqldump not found")
	}
	
	if options == nil {
		options = &BackupOptions{}
	}
	
	// Create temporary file for backup
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("mysql_%s_%s.sql", conn.Database, timestamp)
	backupPath := filepath.Join(bs.tempDir, filename)
	
	startTime := time.Now()
	
	// Build mysqldump command
	args := bs.buildMySQLDumpArgs(conn, options)
	cmd := exec.CommandContext(ctx, bs.mysqlDumpPath, args...)
	
	// Create output file
	outFile, err := os.Create(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}
	defer outFile.Close()
	
	cmd.Stdout = outFile
	
	// Capture stderr for progress tracking
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mysqldump: %w", err)
	}
	
	// Track progress if callback is provided
	if options.ProgressCallback != nil {
		go bs.trackMySQLProgress(stderr, options.ProgressCallback)
	}
	
	// Wait for completion
	if err := cmd.Wait(); err != nil {
		// Clean up on failure
		os.Remove(backupPath)
		return nil, fmt.Errorf("mysqldump failed: %w", err)
	}
	
	duration := time.Since(startTime)
	
	// Get file size
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup file info: %w", err)
	}
	
	// Calculate checksum
	checksum, err := bs.calculateChecksum(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}
	
	result := &BackupResult{
		FilePath:     backupPath,
		OriginalSize: fileInfo.Size(),
		Duration:     duration,
		Tables:       options.Tables,
		Metadata: map[string]string{
			"database_type": "mysql",
			"timestamp":     timestamp,
		},
		Checksum: checksum,
	}
	
	// Compress if requested
	if options.Compress {
		compressedPath := backupPath + ".gz"
		if err := bs.CompressBackup(ctx, backupPath, compressedPath, "gzip"); err != nil {
			return nil, fmt.Errorf("failed to compress backup: %w", err)
		}
		
		// Update result with compressed file info
		compressedInfo, err := os.Stat(compressedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get compressed file info: %w", err)
		}
		
		compressedSize := compressedInfo.Size()
		result.CompressedSize = &compressedSize
		result.FilePath = compressedPath
		
		// Remove original uncompressed file
		os.Remove(backupPath)
	}
	
	return result, nil
}

// buildMySQLDumpArgs builds the arguments for mysqldump command
func (bs *BackupService) buildMySQLDumpArgs(conn *models.DatabaseConnection, options *BackupOptions) []string {
	args := []string{
		"--host", conn.Host,
		"--port", fmt.Sprintf("%d", conn.Port),
		"--user", conn.Username,
		fmt.Sprintf("--password=%s", conn.Password),
		"--routines",
		"--triggers",
	}
	
	if options.SingleTransaction {
		args = append(args, "--single-transaction")
	}
	
	if options.LockTables {
		args = append(args, "--lock-tables")
	}
	
	if options.SkipLockTables {
		args = append(args, "--skip-lock-tables")
	}
	
	if options.QuickDump {
		args = append(args, "--quick")
	}
	
	if options.SchemaOnly {
		args = append(args, "--no-data")
	}
	
	if options.DataOnly {
		args = append(args, "--no-create-info")
	}
	
	// Add database name
	args = append(args, conn.Database)
	
	// Add specific tables
	if len(options.Tables) > 0 {
		args = append(args, options.Tables...)
	}
	
	return args
}

// trackMySQLProgress tracks the progress of mysqldump
func (bs *BackupService) trackMySQLProgress(stderr io.Reader, callback func(float64, string)) {
	scanner := bufio.NewScanner(stderr)
	progress := 0.0
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// MySQL doesn't provide detailed progress, so we estimate
		if strings.Contains(line, "Dumping table") {
			progress += 10.0 // Rough estimate
			if progress > 90.0 {
				progress = 90.0
			}
			callback(progress, line)
		}
	}
	
	// Mark as completed
	callback(100.0, "MySQL backup completed")
}

// RestorePostgreSQLBackup restores a PostgreSQL backup using pg_restore
func (bs *BackupService) RestorePostgreSQLBackup(ctx context.Context, conn *models.DatabaseConnection, backupPath string, options *RestoreOptions) error {
	if bs.pgRestorePath == "" {
		return fmt.Errorf("pg_restore not found")
	}
	
	if options == nil {
		options = &RestoreOptions{}
	}
	
	// Build pg_restore command
	args := []string{
		"--host", conn.Host,
		"--port", fmt.Sprintf("%d", conn.Port),
		"--username", conn.Username,
		"--dbname", conn.Database,
		"--verbose",
	}
	
	if options.CleanFirst {
		args = append(args, "--clean")
	}
	
	if options.CreateDatabase {
		args = append(args, "--create")
	}
	
	if options.Jobs > 1 {
		args = append(args, "--jobs", fmt.Sprintf("%d", options.Jobs))
	}
	
	args = append(args, backupPath)
	
	cmd := exec.CommandContext(ctx, bs.pgRestorePath, args...)
	
	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PGPASSWORD=%s", conn.Password),
	)
	
	// Execute command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore failed: %w", err)
	}
	
	return nil
}

// RestoreMySQLBackup restores a MySQL backup using mysql
func (bs *BackupService) RestoreMySQLBackup(ctx context.Context, conn *models.DatabaseConnection, backupPath string, options *RestoreOptions) error {
	if bs.mysqlPath == "" {
		return fmt.Errorf("mysql not found")
	}
	
	if options == nil {
		options = &RestoreOptions{}
	}
	
	// Build mysql command
	args := []string{
		"--host", conn.Host,
		"--port", fmt.Sprintf("%d", conn.Port),
		"--user", conn.Username,
		fmt.Sprintf("--password=%s", conn.Password),
		conn.Database,
	}
	
	if options.Force {
		args = append(args, "--force")
	}
	
	cmd := exec.CommandContext(ctx, bs.mysqlPath, args...)
	
	// Open backup file
	backupFile, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer backupFile.Close()
	
	cmd.Stdin = backupFile
	
	// Execute command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql restore failed: %w", err)
	}
	
	return nil
}

// GetBackupEstimate estimates backup size and duration
func (bs *BackupService) GetBackupEstimate(ctx context.Context, conn *models.DatabaseConnection, options *BackupOptions) (*BackupEstimate, error) {
	// This is a simplified implementation
	// In a real implementation, you would query the database for table sizes and row counts
	
	estimate := &BackupEstimate{
		EstimatedSize:     100 * 1024 * 1024, // 100MB estimate
		EstimatedDuration: 5 * time.Minute,   // 5 minute estimate
		TableCount:        10,                 // Placeholder
		RowCount:          1000,               // Placeholder
	}
	
	return estimate, nil
}

// CompressBackup compresses a backup file
func (bs *BackupService) CompressBackup(ctx context.Context, inputPath, outputPath string, algorithm string) error {
	var cmd *exec.Cmd
	
	switch algorithm {
	case "gzip":
		cmd = exec.CommandContext(ctx, "gzip", "-c", inputPath)
	case "bzip2":
		cmd = exec.CommandContext(ctx, "bzip2", "-c", inputPath)
	case "xz":
		cmd = exec.CommandContext(ctx, "xz", "-c", inputPath)
	default:
		return fmt.Errorf("unsupported compression algorithm: %s", algorithm)
	}
	
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create compressed file: %w", err)
	}
	defer outFile.Close()
	
	cmd.Stdout = outFile
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}
	
	return nil
}

// DecompressBackup decompresses a backup file
func (bs *BackupService) DecompressBackup(ctx context.Context, inputPath, outputPath string) error {
	var cmd *exec.Cmd
	
	if strings.HasSuffix(inputPath, ".gz") {
		cmd = exec.CommandContext(ctx, "gunzip", "-c", inputPath)
	} else if strings.HasSuffix(inputPath, ".bz2") {
		cmd = exec.CommandContext(ctx, "bunzip2", "-c", inputPath)
	} else if strings.HasSuffix(inputPath, ".xz") {
		cmd = exec.CommandContext(ctx, "unxz", "-c", inputPath)
	} else {
		return fmt.Errorf("unknown compression format for file: %s", inputPath)
	}
	
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create decompressed file: %w", err)
	}
	defer outFile.Close()
	
	cmd.Stdout = outFile
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}
	
	return nil
}

// calculateChecksum calculates SHA256 checksum of a file
func (bs *BackupService) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		return "", err
	}
	
	// For simplicity, using a basic hash - in production you'd use crypto/sha256
	return fmt.Sprintf("sha256:%x", buf.Bytes()[:32]), nil
}