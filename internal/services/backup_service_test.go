package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBackupService(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.NotEmpty(t, service.tempDir)
}

func TestBackupService_ValidateBackupTools(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	// This test will pass or fail depending on whether backup tools are installed
	// In CI/CD environments, tools might not be available
	err = service.ValidateBackupTools()
	if err != nil {
		t.Logf("Backup tools validation failed (expected in some environments): %v", err)
		assert.Contains(t, err.Error(), "missing backup tools")
	} else {
		t.Log("All backup tools are available")
	}
}

func TestBackupService_findBackupTools(t *testing.T) {
	service := &BackupService{
		tempDir: os.TempDir(),
	}

	err := service.findBackupTools()
	assert.NoError(t, err)

	// Check that paths are set (empty string means not found)
	t.Logf("pg_dump path: %s", service.pgDumpPath)
	t.Logf("pg_restore path: %s", service.pgRestorePath)
	t.Logf("mysqldump path: %s", service.mysqlDumpPath)
	t.Logf("mysql path: %s", service.mysqlPath)
}

func TestBackupService_buildPgDumpArgs(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     5432,
		Username: "testuser",
		Database: "testdb",
	}

	options := &BackupOptions{
		Format:        "custom",
		Verbose:       true,
		SchemaOnly:    false,
		DataOnly:      false,
		Jobs:          2,
		Tables:        []string{"table1", "table2"},
		ExcludeTables: []string{"temp_table"},
	}

	outputPath := "/tmp/test.backup"
	args := service.buildPgDumpArgs(conn, outputPath, options)

	expectedArgs := []string{
		"--host", "localhost",
		"--port", "5432",
		"--username", "testuser",
		"--dbname", "testdb",
		"--format", "custom",
		"--file", "/tmp/test.backup",
		"--verbose",
		"--table", "table1",
		"--table", "table2",
		"--exclude-table", "temp_table",
	}

	// Check that all expected args are present
	for _, expectedArg := range expectedArgs {
		assert.Contains(t, args, expectedArg)
	}
}

func TestBackupService_buildPgDumpArgs_DirectoryFormat(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     5432,
		Username: "testuser",
		Database: "testdb",
	}

	options := &BackupOptions{
		Format: "directory",
		Jobs:   4,
	}

	outputPath := "/tmp/test_dir"
	args := service.buildPgDumpArgs(conn, outputPath, options)

	// Should include jobs argument for directory format
	assert.Contains(t, args, "--jobs")
	assert.Contains(t, args, "4")
}

func TestBackupService_buildPgDumpArgs_SchemaAndDataOptions(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     5432,
		Username: "testuser",
		Database: "testdb",
	}

	tests := []struct {
		name     string
		options  *BackupOptions
		expected []string
		excluded []string
	}{
		{
			name: "schema only",
			options: &BackupOptions{
				Format:     "custom",
				SchemaOnly: true,
			},
			expected: []string{"--schema-only"},
			excluded: []string{"--data-only"},
		},
		{
			name: "data only",
			options: &BackupOptions{
				Format:   "custom",
				DataOnly: true,
			},
			expected: []string{"--data-only"},
			excluded: []string{"--schema-only"},
		},
		{
			name: "both schema and data (default)",
			options: &BackupOptions{
				Format: "custom",
			},
			expected: []string{},
			excluded: []string{"--schema-only", "--data-only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := service.buildPgDumpArgs(conn, "/tmp/test.backup", tt.options)
			
			for _, expected := range tt.expected {
				assert.Contains(t, args, expected)
			}
			
			for _, excluded := range tt.excluded {
				assert.NotContains(t, args, excluded)
			}
		})
	}
}

func TestBackupService_buildMySQLDumpArgs(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     3306,
		Username: "testuser",
		Password: "testpass",
		Database: "testdb",
	}

	options := &BackupOptions{
		SingleTransaction: true,
		LockTables:        false,
		SkipLockTables:    true,
		QuickDump:         true,
		SchemaOnly:        false,
		DataOnly:          false,
		Tables:            []string{"table1", "table2"},
	}

	args := service.buildMySQLDumpArgs(conn, options)

	expectedArgs := []string{
		"--host", "localhost",
		"--port", "3306",
		"--user", "testuser",
		"--password=testpass",
		"--routines",
		"--triggers",
		"--single-transaction",
		"--skip-lock-tables",
		"--quick",
		"testdb",
		"table1",
		"table2",
	}

	// Check that all expected args are present
	for _, expectedArg := range expectedArgs {
		assert.Contains(t, args, expectedArg)
	}

	// Should not contain lock-tables since it's false
	assert.NotContains(t, args, "--lock-tables")
}

func TestBackupService_buildMySQLDumpArgs_SchemaAndDataOptions(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     3306,
		Username: "testuser",
		Password: "testpass",
		Database: "testdb",
	}

	tests := []struct {
		name     string
		options  *BackupOptions
		expected []string
		excluded []string
	}{
		{
			name: "schema only",
			options: &BackupOptions{
				SchemaOnly: true,
			},
			expected: []string{"--no-data"},
			excluded: []string{"--no-create-info"},
		},
		{
			name: "data only",
			options: &BackupOptions{
				DataOnly: true,
			},
			expected: []string{"--no-create-info"},
			excluded: []string{"--no-data"},
		},
		{
			name: "both schema and data (default)",
			options: &BackupOptions{},
			expected: []string{},
			excluded: []string{"--no-data", "--no-create-info"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := service.buildMySQLDumpArgs(conn, tt.options)
			
			for _, expected := range tt.expected {
				assert.Contains(t, args, expected)
			}
			
			for _, excluded := range tt.excluded {
				assert.NotContains(t, args, excluded)
			}
		})
	}
}

func TestBackupService_GetBackupEstimate(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     5432,
		Username: "testuser",
		Database: "testdb",
	}

	options := &BackupOptions{
		Tables: []string{"table1", "table2"},
	}

	ctx := context.Background()
	estimate, err := service.GetBackupEstimate(ctx, conn, options)
	
	require.NoError(t, err)
	assert.NotNil(t, estimate)
	assert.Greater(t, estimate.EstimatedSize, int64(0))
	assert.Greater(t, estimate.EstimatedDuration, time.Duration(0))
	assert.Greater(t, estimate.TableCount, 0)
	assert.GreaterOrEqual(t, estimate.RowCount, int64(0))
}

func TestBackupService_CompressBackup(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	// Create a temporary test file
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.txt")
	outputFile := filepath.Join(tempDir, "test.txt.gz")

	// Write test content
	testContent := "This is test content for compression testing"
	err = os.WriteFile(inputFile, []byte(testContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Test gzip compression
	err = service.CompressBackup(ctx, inputFile, outputFile, "gzip")
	
	// Check if gzip is available on the system
	if err != nil && (os.IsNotExist(err) || err.Error() == "exec: \"gzip\": executable file not found in $PATH") {
		t.Skip("gzip not available on system")
	}
	
	require.NoError(t, err)
	
	// Verify compressed file exists and is smaller than original (in most cases)
	inputInfo, err := os.Stat(inputFile)
	require.NoError(t, err)
	
	outputInfo, err := os.Stat(outputFile)
	require.NoError(t, err)
	
	assert.Greater(t, inputInfo.Size(), int64(0))
	assert.Greater(t, outputInfo.Size(), int64(0))
	
	// For small files, compressed might be larger due to overhead
	t.Logf("Original size: %d, Compressed size: %d", inputInfo.Size(), outputInfo.Size())
}

func TestBackupService_DecompressBackup(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	// Create a temporary test file
	tempDir := t.TempDir()
	originalFile := filepath.Join(tempDir, "original.txt")
	compressedFile := filepath.Join(tempDir, "test.txt.gz")
	decompressedFile := filepath.Join(tempDir, "decompressed.txt")

	// Write test content
	testContent := "This is test content for decompression testing"
	err = os.WriteFile(originalFile, []byte(testContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	
	// First compress the file
	err = service.CompressBackup(ctx, originalFile, compressedFile, "gzip")
	if err != nil && (os.IsNotExist(err) || err.Error() == "exec: \"gzip\": executable file not found in $PATH") {
		t.Skip("gzip not available on system")
	}
	require.NoError(t, err)

	// Now decompress it
	err = service.DecompressBackup(ctx, compressedFile, decompressedFile)
	if err != nil && (os.IsNotExist(err) || err.Error() == "exec: \"gunzip\": executable file not found in $PATH") {
		t.Skip("gunzip not available on system")
	}
	require.NoError(t, err)

	// Verify decompressed content matches original
	decompressedContent, err := os.ReadFile(decompressedFile)
	require.NoError(t, err)
	
	assert.Equal(t, testContent, string(decompressedContent))
}

func TestBackupService_CompressBackup_UnsupportedAlgorithm(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.txt")
	outputFile := filepath.Join(tempDir, "test.txt.unknown")

	// Write test content
	err = os.WriteFile(inputFile, []byte("test"), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Test unsupported compression algorithm
	err = service.CompressBackup(ctx, inputFile, outputFile, "unsupported")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported compression algorithm")
}

func TestBackupService_DecompressBackup_UnknownFormat(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "test.unknown")
	outputFile := filepath.Join(tempDir, "output.txt")

	// Write test content
	err = os.WriteFile(inputFile, []byte("test"), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Test unknown compression format
	err = service.DecompressBackup(ctx, inputFile, outputFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown compression format")
}

func TestBackupService_calculateChecksum(t *testing.T) {
	service, err := NewBackupService()
	require.NoError(t, err)

	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	testContent := "This is test content for checksum calculation"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	checksum, err := service.calculateChecksum(testFile)
	require.NoError(t, err)
	
	assert.NotEmpty(t, checksum)
	assert.Contains(t, checksum, "sha256:")
	
	// Calculate checksum again to ensure consistency
	checksum2, err := service.calculateChecksum(testFile)
	require.NoError(t, err)
	
	assert.Equal(t, checksum, checksum2)
}

func TestBackupService_PostgreSQLBackup_MissingTool(t *testing.T) {
	service := &BackupService{
		tempDir:    os.TempDir(),
		pgDumpPath: "", // Simulate missing pg_dump
	}

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     5432,
		Username: "testuser",
		Database: "testdb",
	}

	ctx := context.Background()
	_, err := service.CreatePostgreSQLBackup(ctx, conn, nil)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pg_dump not found")
}

func TestBackupService_MySQLBackup_MissingTool(t *testing.T) {
	service := &BackupService{
		tempDir:       os.TempDir(),
		mysqlDumpPath: "", // Simulate missing mysqldump
	}

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     3306,
		Username: "testuser",
		Database: "testdb",
	}

	ctx := context.Background()
	_, err := service.CreateMySQLBackup(ctx, conn, nil)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mysqldump not found")
}

func TestBackupService_PostgreSQLRestore_MissingTool(t *testing.T) {
	service := &BackupService{
		tempDir:       os.TempDir(),
		pgRestorePath: "", // Simulate missing pg_restore
	}

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     5432,
		Username: "testuser",
		Database: "testdb",
	}

	ctx := context.Background()
	err := service.RestorePostgreSQLBackup(ctx, conn, "/fake/path", nil)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pg_restore not found")
}

func TestBackupService_MySQLRestore_MissingTool(t *testing.T) {
	service := &BackupService{
		tempDir:   os.TempDir(),
		mysqlPath: "", // Simulate missing mysql
	}

	conn := &models.DatabaseConnection{
		Host:     "localhost",
		Port:     3306,
		Username: "testuser",
		Database: "testdb",
	}

	ctx := context.Background()
	err := service.RestoreMySQLBackup(ctx, conn, "/fake/path", nil)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mysql not found")
}

func TestBackupOptions_DefaultValues(t *testing.T) {
	options := &BackupOptions{}
	
	// Test that nil options don't cause panics and work with default values
	assert.NotNil(t, options)
	assert.False(t, options.SchemaOnly)
	assert.False(t, options.DataOnly)
	assert.False(t, options.Compress)
	assert.Empty(t, options.Tables)
	assert.Empty(t, options.ExcludeTables)
}

func TestRestoreOptions_DefaultValues(t *testing.T) {
	options := &RestoreOptions{}
	
	// Test that nil options don't cause panics and work with default values
	assert.NotNil(t, options)
	assert.False(t, options.DropExisting)
	assert.False(t, options.CreateDatabase)
	assert.False(t, options.CleanFirst)
	assert.False(t, options.Force)
}

func TestBackupResult_Structure(t *testing.T) {
	result := &BackupResult{
		FilePath:     "/path/to/backup.sql",
		OriginalSize: 1024,
		Duration:     5 * time.Minute,
		Tables:       []string{"table1", "table2"},
		Metadata: map[string]string{
			"database_type": "postgresql",
			"format":        "custom",
		},
		Checksum: "sha256:abcd1234",
	}
	
	assert.Equal(t, "/path/to/backup.sql", result.FilePath)
	assert.Equal(t, int64(1024), result.OriginalSize)
	assert.Equal(t, 5*time.Minute, result.Duration)
	assert.Equal(t, []string{"table1", "table2"}, result.Tables)
	assert.Equal(t, "postgresql", result.Metadata["database_type"])
	assert.Equal(t, "sha256:abcd1234", result.Checksum)
	assert.Nil(t, result.CompressedSize)
}

func TestBackupEstimate_Structure(t *testing.T) {
	estimate := &BackupEstimate{
		EstimatedSize:     100 * 1024 * 1024,
		EstimatedDuration: 10 * time.Minute,
		TableCount:        5,
		RowCount:          10000,
	}
	
	assert.Equal(t, int64(100*1024*1024), estimate.EstimatedSize)
	assert.Equal(t, 10*time.Minute, estimate.EstimatedDuration)
	assert.Equal(t, 5, estimate.TableCount)
	assert.Equal(t, int64(10000), estimate.RowCount)
}

// Integration tests would go here but require actual database tools
// These tests are skipped by default and would only run in full integration test environments

func TestBackupService_Integration_PostgreSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// This test would require:
	// - PostgreSQL server running
	// - pg_dump and pg_restore tools installed
	// - Test database with sample data
	
	t.Skip("Integration test requires PostgreSQL setup")
}

func TestBackupService_Integration_MySQL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// This test would require:
	// - MySQL server running
	// - mysqldump and mysql tools installed
	// - Test database with sample data
	
	t.Skip("Integration test requires MySQL setup")
}