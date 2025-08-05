package models

import (
	"testing"
	"time"

	"github.com/dbackup/backend-go/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableType_IsValid(t *testing.T) {
	tests := []struct {
		tableType models.TableType
		expected  bool
	}{
		{models.TableTypeTable, true},
		{models.TableTypeView, true},
		{models.TableTypeMaterialized, true},
		{models.TableTypeSequence, true},
		{models.TableTypeIndex, true},
		{models.TableTypePartition, true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.tableType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.tableType.IsValid())
		})
	}
}

func TestTableType_GetDisplayName(t *testing.T) {
	tests := []struct {
		tableType models.TableType
		expected  string
	}{
		{models.TableTypeTable, "Table"},
		{models.TableTypeView, "View"},
		{models.TableTypeMaterialized, "Materialized View"},
		{models.TableTypeSequence, "Sequence"},
		{models.TableTypeIndex, "Index"},
		{models.TableTypePartition, "Partition"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.tableType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.tableType.GetDisplayName())
		})
	}
}

func TestIndexType_IsValid(t *testing.T) {
	tests := []struct {
		indexType models.IndexType
		expected  bool
	}{
		{models.IndexTypeBtree, true},
		{models.IndexTypeHash, true},
		{models.IndexTypeGin, true},
		{models.IndexTypeGist, true},
		{models.IndexTypeBrin, true},
		{models.IndexTypeSpgist, true},
		{models.IndexTypeFulltext, true},
		{models.IndexTypeSpatial, true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.indexType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.indexType.IsValid())
		})
	}
}

func TestIndexType_GetDisplayName(t *testing.T) {
	tests := []struct {
		indexType models.IndexType
		expected  string
	}{
		{models.IndexTypeBtree, "B-Tree"},
		{models.IndexTypeHash, "Hash"},
		{models.IndexTypeGin, "GIN"},
		{models.IndexTypeGist, "GiST"},
		{models.IndexTypeBrin, "BRIN"},
		{models.IndexTypeSpgist, "SP-GiST"},
		{models.IndexTypeFulltext, "Full Text"},
		{models.IndexTypeSpatial, "Spatial"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.indexType), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.indexType.GetDisplayName())
		})
	}
}

func TestTableAccessLevel_IsValid(t *testing.T) {
	tests := []struct {
		accessLevel models.TableAccessLevel
		expected    bool
	}{
		{models.TableAccessNone, true},
		{models.TableAccessRead, true},
		{models.TableAccessWrite, true},
		{models.TableAccessAdmin, true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.accessLevel), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.accessLevel.IsValid())
		})
	}
}

func TestTableAccessLevel_GetDisplayName(t *testing.T) {
	tests := []struct {
		accessLevel models.TableAccessLevel
		expected    string
	}{
		{models.TableAccessNone, "No Access"},
		{models.TableAccessRead, "Read Only"},
		{models.TableAccessWrite, "Read/Write"},
		{models.TableAccessAdmin, "Administrator"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.accessLevel), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.accessLevel.GetDisplayName())
		})
	}
}

func TestDatabaseTable_GetFullName(t *testing.T) {
	tests := []struct {
		name     string
		table    *models.DatabaseTable
		expected string
	}{
		{
			name: "with schema",
			table: &models.DatabaseTable{
				Name:   "users",
				Schema: "app",
			},
			expected: "app.users",
		},
		{
			name: "with public schema",
			table: &models.DatabaseTable{
				Name:   "users",
				Schema: "public",
			},
			expected: "users",
		},
		{
			name: "without schema",
			table: &models.DatabaseTable{
				Name:   "users",
				Schema: "",
			},
			expected: "users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.table.GetFullName())
		})
	}
}

func TestDatabaseTable_GetSizeInBytes(t *testing.T) {
	tests := []struct {
		name     string
		table    *models.DatabaseTable
		expected int64
	}{
		{
			name: "with both data and index length",
			table: &models.DatabaseTable{
				DataLength:  int64Ptr(1024),
				IndexLength: int64Ptr(512),
			},
			expected: 1536,
		},
		{
			name: "with only data length",
			table: &models.DatabaseTable{
				DataLength:  int64Ptr(1024),
				IndexLength: nil,
			},
			expected: 1024,
		},
		{
			name: "with only index length",
			table: &models.DatabaseTable{
				DataLength:  nil,
				IndexLength: int64Ptr(512),
			},
			expected: 512,
		},
		{
			name: "with no lengths",
			table: &models.DatabaseTable{
				DataLength:  nil,
				IndexLength: nil,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.table.GetSizeInBytes())
		})
	}
}

func TestDatabaseTable_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		table    *models.DatabaseTable
		expected bool
	}{
		{
			name: "empty table",
			table: &models.DatabaseTable{
				RowCount: int64Ptr(0),
			},
			expected: true,
		},
		{
			name: "non-empty table",
			table: &models.DatabaseTable{
				RowCount: int64Ptr(100),
			},
			expected: false,
		},
		{
			name: "nil row count",
			table: &models.DatabaseTable{
				RowCount: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.table.IsEmpty())
		})
	}
}

func TestDatabaseTable_HasStructureChanged(t *testing.T) {
	tests := []struct {
		name     string
		table    *models.DatabaseTable
		newHash  string
		expected bool
	}{
		{
			name: "no existing hash",
			table: &models.DatabaseTable{
				StructureHash: nil,
			},
			newHash:  "new_hash",
			expected: true,
		},
		{
			name: "same hash",
			table: &models.DatabaseTable{
				StructureHash: stringPtr("existing_hash"),
			},
			newHash:  "existing_hash",
			expected: false,
		},
		{
			name: "different hash",
			table: &models.DatabaseTable{
				StructureHash: stringPtr("existing_hash"),
			},
			newHash:  "new_hash",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.table.HasStructureChanged(tt.newHash))
		})
	}
}

func TestDatabaseTable_UpdateStructureHash(t *testing.T) {
	table := &models.DatabaseTable{}
	newHash := "test_hash"

	table.UpdateStructureHash(newHash)

	require.NotNil(t, table.StructureHash)
	assert.Equal(t, newHash, *table.StructureHash)
}

func TestDatabaseTable_CanBackup(t *testing.T) {
	tests := []struct {
		name     string
		table    *models.DatabaseTable
		expected bool
	}{
		{
			name: "can backup",
			table: &models.DatabaseTable{
				Type:              models.TableTypeTable,
				IsBackupEnabled:   true,
				ExcludeFromBackup: false,
				HasSelectAccess:   true,
			},
			expected: true,
		},
		{
			name: "backup disabled",
			table: &models.DatabaseTable{
				Type:              models.TableTypeTable,
				IsBackupEnabled:   false,
				ExcludeFromBackup: false,
				HasSelectAccess:   true,
			},
			expected: false,
		},
		{
			name: "excluded from backup",
			table: &models.DatabaseTable{
				Type:              models.TableTypeTable,
				IsBackupEnabled:   true,
				ExcludeFromBackup: true,
				HasSelectAccess:   true,
			},
			expected: false,
		},
		{
			name: "no select access",
			table: &models.DatabaseTable{
				Type:              models.TableTypeTable,
				IsBackupEnabled:   true,
				ExcludeFromBackup: false,
				HasSelectAccess:   false,
			},
			expected: false,
		},
		{
			name: "is view",
			table: &models.DatabaseTable{
				Type:              models.TableTypeView,
				IsBackupEnabled:   true,
				ExcludeFromBackup: false,
				HasSelectAccess:   true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.table.CanBackup())
		})
	}
}

func TestDatabaseTable_GetBackupPriorityLevel(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		expected string
	}{
		{"critical", 10, "critical"},
		{"high", 40, "high"},
		{"medium", 60, "medium"},
		{"low", 100, "low"},
		{"low default", 200, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := &models.DatabaseTable{
				BackupPriority: tt.priority,
			}
			assert.Equal(t, tt.expected, table.GetBackupPriorityLevel())
		})
	}
}

func TestDatabaseTable_GetActiveBackupJobsCount(t *testing.T) {
	table := &models.DatabaseTable{
		BackupJobs: []models.BackupJob{
			{Status: models.BackupStatusPending},
			{Status: models.BackupStatusRunning},
			{Status: models.BackupStatusCompleted},
			{Status: models.BackupStatusFailed},
			{Status: models.BackupStatusPending},
		},
	}

	count := table.GetActiveBackupJobsCount()
	assert.Equal(t, 3, count) // 2 pending + 1 running
}

func TestDatabaseTable_HasActiveBackups(t *testing.T) {
	tests := []struct {
		name     string
		table    *models.DatabaseTable
		expected bool
	}{
		{
			name: "has active backups",
			table: &models.DatabaseTable{
				BackupJobs: []models.BackupJob{
					{Status: models.BackupStatusPending},
				},
			},
			expected: true,
		},
		{
			name: "no active backups",
			table: &models.DatabaseTable{
				BackupJobs: []models.BackupJob{
					{Status: models.BackupStatusCompleted},
				},
			},
			expected: false,
		},
		{
			name: "no backup jobs",
			table: &models.DatabaseTable{
				BackupJobs: []models.BackupJob{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.table.HasActiveBackups())
		})
	}
}

func TestDatabaseTable_GetCounts(t *testing.T) {
	table := &models.DatabaseTable{
		Columns: []models.DatabaseColumn{
			{Name: "id"},
			{Name: "name"},
		},
		Indexes: []models.DatabaseIndex{
			{Name: "idx_name"},
		},
		ForeignKeys: []models.DatabaseForeignKey{
			{Name: "fk_user_id"},
		},
	}

	assert.Equal(t, 2, table.GetColumnCount())
	assert.Equal(t, 1, table.GetIndexCount())
	assert.Equal(t, 1, table.GetForeignKeyCount())
}

func TestDatabaseTable_GetPrimaryKeyColumns(t *testing.T) {
	table := &models.DatabaseTable{
		Columns: []models.DatabaseColumn{
			{Name: "id", IsPrimaryKey: true},
			{Name: "name", IsPrimaryKey: false},
			{Name: "org_id", IsPrimaryKey: true},
		},
	}

	pkColumns := table.GetPrimaryKeyColumns()
	assert.Len(t, pkColumns, 2)
	assert.Equal(t, "id", pkColumns[0].Name)
	assert.Equal(t, "org_id", pkColumns[1].Name)
}

func TestDatabaseTable_HasPrimaryKey(t *testing.T) {
	tests := []struct {
		name     string
		table    *models.DatabaseTable
		expected bool
	}{
		{
			name: "has primary key",
			table: &models.DatabaseTable{
				Columns: []models.DatabaseColumn{
					{Name: "id", IsPrimaryKey: true},
					{Name: "name", IsPrimaryKey: false},
				},
			},
			expected: true,
		},
		{
			name: "no primary key",
			table: &models.DatabaseTable{
				Columns: []models.DatabaseColumn{
					{Name: "name", IsPrimaryKey: false},
				},
			},
			expected: false,
		},
		{
			name: "no columns",
			table: &models.DatabaseTable{
				Columns: []models.DatabaseColumn{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.table.HasPrimaryKey())
		})
	}
}

func TestDatabaseTable_GetByName(t *testing.T) {
	table := &models.DatabaseTable{
		Columns: []models.DatabaseColumn{
			{Name: "id"},
			{Name: "name"},
		},
		Indexes: []models.DatabaseIndex{
			{Name: "idx_id"},
			{Name: "idx_name"},
		},
	}

	t.Run("get column by name - found", func(t *testing.T) {
		column := table.GetColumnByName("name")
		require.NotNil(t, column)
		assert.Equal(t, "name", column.Name)
	})

	t.Run("get column by name - not found", func(t *testing.T) {
		column := table.GetColumnByName("nonexistent")
		assert.Nil(t, column)
	})

	t.Run("get index by name - found", func(t *testing.T) {
		index := table.GetIndexByName("idx_name")
		require.NotNil(t, index)
		assert.Equal(t, "idx_name", index.Name)
	})

	t.Run("get index by name - not found", func(t *testing.T) {
		index := table.GetIndexByName("nonexistent")
		assert.Nil(t, index)
	})
}

func TestDatabaseTable_IsAccessible(t *testing.T) {
	tests := []struct {
		name          string
		accessLevel   models.TableAccessLevel
		requiredLevel models.TableAccessLevel
		expected      bool
	}{
		{"none required, none access", models.TableAccessNone, models.TableAccessNone, true},
		{"read required, none access", models.TableAccessNone, models.TableAccessRead, false},
		{"read required, read access", models.TableAccessRead, models.TableAccessRead, true},
		{"read required, write access", models.TableAccessWrite, models.TableAccessRead, true},
		{"read required, admin access", models.TableAccessAdmin, models.TableAccessRead, true},
		{"write required, read access", models.TableAccessRead, models.TableAccessWrite, false},
		{"write required, write access", models.TableAccessWrite, models.TableAccessWrite, true},
		{"write required, admin access", models.TableAccessAdmin, models.TableAccessWrite, true},
		{"admin required, write access", models.TableAccessWrite, models.TableAccessAdmin, false},
		{"admin required, admin access", models.TableAccessAdmin, models.TableAccessAdmin, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := &models.DatabaseTable{
				AccessLevel: tt.accessLevel,
			}
			assert.Equal(t, tt.expected, table.IsAccessible(tt.requiredLevel))
		})
	}
}

func TestDatabaseTable_IsView(t *testing.T) {
	tests := []struct {
		name      string
		tableType models.TableType
		expected  bool
	}{
		{"table", models.TableTypeTable, false},
		{"view", models.TableTypeView, true},
		{"materialized view", models.TableTypeMaterialized, true},
		{"sequence", models.TableTypeSequence, false},
		{"index", models.TableTypeIndex, false},
		{"partition", models.TableTypePartition, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := &models.DatabaseTable{
				Type: tt.tableType,
			}
			assert.Equal(t, tt.expected, table.IsView())
		})
	}
}

func TestDatabaseTable_IsStale(t *testing.T) {
	tests := []struct {
		name             string
		lastDiscoveredAt time.Time
		expected         bool
	}{
		{
			name:             "recent discovery",
			lastDiscoveredAt: time.Now().Add(-1 * time.Hour),
			expected:         false,
		},
		{
			name:             "stale discovery",
			lastDiscoveredAt: time.Now().Add(-25 * time.Hour),
			expected:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := &models.DatabaseTable{
				LastDiscoveredAt: tt.lastDiscoveredAt,
			}
			assert.Equal(t, tt.expected, table.IsStale())
		})
	}
}

func TestDatabaseTable_UpdateStatistics(t *testing.T) {
	table := &models.DatabaseTable{}
	
	rowCount := int64(1000)
	dataLength := int64(2048)
	indexLength := int64(512)

	beforeUpdate := time.Now()
	table.UpdateStatistics(rowCount, dataLength, indexLength)
	afterUpdate := time.Now()

	require.NotNil(t, table.RowCount)
	assert.Equal(t, rowCount, *table.RowCount)
	require.NotNil(t, table.DataLength)
	assert.Equal(t, dataLength, *table.DataLength)
	require.NotNil(t, table.IndexLength)
	assert.Equal(t, indexLength, *table.IndexLength)
	
	assert.True(t, table.LastDiscoveredAt.After(beforeUpdate))
	assert.True(t, table.LastDiscoveredAt.Before(afterUpdate))
}

func TestDatabaseTable_ToPublic(t *testing.T) {
	now := time.Now()
	comment := "Test table"
	engine := "InnoDB"
	collation := "utf8_general_ci"
	schedule := "0 2 * * *"
	discoveryError := "Connection timeout"

	table := &models.DatabaseTable{
		ID:                1,
		UID:               "test-uid",
		Name:              "test_table",
		Schema:            "test_schema",
		Type:              models.TableTypeTable,
		Comment:           &comment,
		Engine:            &engine,
		Collation:         &collation,
		RowCount:          int64Ptr(1000),
		DataLength:        int64Ptr(2048),
		IndexLength:       int64Ptr(512),
		AutoIncrement:     int64Ptr(1001),
		LastDiscoveredAt:  now,
		DiscoveryError:    &discoveryError,
		IsBackupEnabled:   true,
		BackupPriority:    50,
		ExcludeFromBackup: false,
		BackupSchedule:    &schedule,
		HasSelectAccess:   true,
		AccessLevel:       models.TableAccessRead,
		Columns: []models.DatabaseColumn{
			{Name: "id", IsPrimaryKey: true},
			{Name: "name"},
		},
		Indexes: []models.DatabaseIndex{
			{Name: "idx_name"},
		},
		ForeignKeys: []models.DatabaseForeignKey{
			{Name: "fk_user_id"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	public := table.ToPublic()

	assert.Equal(t, table.ID, public.ID)
	assert.Equal(t, table.UID, public.UID)
	assert.Equal(t, table.Name, public.Name)
	assert.Equal(t, table.Schema, public.Schema)
	assert.Equal(t, table.Type, public.Type)
	assert.Equal(t, comment, public.Comment)
	assert.Equal(t, engine, public.Engine)
	assert.Equal(t, collation, public.Collation)
	assert.Equal(t, table.RowCount, public.RowCount)
	assert.Equal(t, table.DataLength, public.DataLength)
	assert.Equal(t, table.IndexLength, public.IndexLength)
	assert.Equal(t, table.AutoIncrement, public.AutoIncrement)
	assert.Equal(t, table.LastDiscoveredAt, public.LastDiscoveredAt)
	assert.Equal(t, discoveryError, public.DiscoveryError)
	assert.Equal(t, table.IsBackupEnabled, public.IsBackupEnabled)
	assert.Equal(t, table.BackupPriority, public.BackupPriority)
	assert.Equal(t, table.ExcludeFromBackup, public.ExcludeFromBackup)
	assert.Equal(t, schedule, public.BackupSchedule)
	assert.Equal(t, table.HasSelectAccess, public.HasSelectAccess)
	assert.Equal(t, table.AccessLevel, public.AccessLevel)
	assert.Equal(t, "test_schema.test_table", public.FullName)
	assert.Equal(t, int64(2560), public.SizeInBytes) // 2048 + 512
	assert.NotEmpty(t, public.FormattedSize)
	assert.False(t, public.IsEmpty)
	assert.True(t, public.CanBackup)
	assert.Equal(t, "high", public.BackupPriorityLevel)
	assert.False(t, public.HasActiveBackups)
	assert.Equal(t, 2, public.ColumnCount)
	assert.Equal(t, 1, public.IndexCount)
	assert.Equal(t, 1, public.ForeignKeyCount)
	assert.True(t, public.HasPrimaryKey)
	assert.False(t, public.IsView)
	assert.Equal(t, table.CreatedAt, public.CreatedAt)
	assert.Equal(t, table.UpdatedAt, public.UpdatedAt)
}

func TestDatabaseTable_ApplyUpdate(t *testing.T) {
	table := &models.DatabaseTable{
		IsBackupEnabled:   false,
		BackupPriority:    100,
		ExcludeFromBackup: false,
		BackupSchedule:    stringPtr("old_schedule"),
	}

	update := &models.TableUpdateRequest{
		IsBackupEnabled:   boolPtr(true),
		BackupPriority:    intPtr(50),
		ExcludeFromBackup: boolPtr(true),
		BackupSchedule:    stringPtr("0 2 * * *"),
	}

	table.ApplyUpdate(update)

	assert.True(t, table.IsBackupEnabled)
	assert.Equal(t, 50, table.BackupPriority)
	assert.True(t, table.ExcludeFromBackup)
	require.NotNil(t, table.BackupSchedule)
	assert.Equal(t, "0 2 * * *", *table.BackupSchedule)

	t.Run("clear schedule", func(t *testing.T) {
		emptyUpdate := &models.TableUpdateRequest{
			BackupSchedule: stringPtr(""),
		}
		
		table.ApplyUpdate(emptyUpdate)
		assert.Nil(t, table.BackupSchedule)
	})
}

func TestDatabaseColumn_GetDataTypeCategory(t *testing.T) {
	tests := []struct {
		dataType string
		expected string
	}{
		{"int", "integer"},
		{"bigint", "integer"},
		{"serial", "integer"},
		{"decimal", "decimal"},
		{"numeric", "decimal"},
		{"float", "decimal"},
		{"varchar", "string"},
		{"text", "string"},
		{"character varying", "string"},
		{"date", "datetime"},
		{"timestamp", "datetime"},
		{"timestamptz", "datetime"},
		{"boolean", "boolean"},
		{"bool", "boolean"},
		{"bit", "boolean"},
		{"json", "json"},
		{"jsonb", "json"},
		{"uuid", "uuid"},
		{"bytea", "binary"},
		{"blob", "binary"},
		{"varbinary", "binary"},
		{"unknown_type", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			column := &models.DatabaseColumn{
				DataType: tt.dataType,
			}
			assert.Equal(t, tt.expected, column.GetDataTypeCategory())
		})
	}
}

func TestDatabaseColumn_TypeChecks(t *testing.T) {
	tests := []struct {
		name       string
		dataType   string
		isNumeric  bool
		isString   bool
		isDateTime bool
	}{
		{"integer", "int", true, false, false},
		{"decimal", "decimal", true, false, false},
		{"string", "varchar", false, true, false},
		{"datetime", "timestamp", false, false, true},
		{"boolean", "boolean", false, false, false},
		{"json", "json", false, false, false},
		{"uuid", "uuid", false, false, false},
		{"binary", "bytea", false, false, false},
		{"other", "unknown", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := &models.DatabaseColumn{
				DataType: tt.dataType,
			}
			assert.Equal(t, tt.isNumeric, column.IsNumeric())
			assert.Equal(t, tt.isString, column.IsString())
			assert.Equal(t, tt.isDateTime, column.IsDateTime())
		})
	}
}

// Helper functions
func int64Ptr(i int64) *int64 {
	return &i
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}