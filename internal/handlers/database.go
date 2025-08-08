package handlers

import (
	"net/http"
	"strconv"

	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/encryption"
	"github.com/dbackup/backend-go/internal/middleware"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/dbackup/backend-go/internal/responses"
	"github.com/dbackup/backend-go/internal/services"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// DatabaseHandler handles database connection management
type DatabaseHandler struct {
	db         *gorm.DB
	dbService  *services.DatabaseService
	encService *encryption.Service
}

// NewDatabaseHandler creates a new database handler
func NewDatabaseHandler(db *gorm.DB, encService *encryption.Service) *DatabaseHandler {
	return &DatabaseHandler{
		db:         db,
		dbService:  services.NewDatabaseService(db, encService),
		encService: encService,
	}
}

// ListDatabaseConnections handles GET /api/databases
func (h *DatabaseHandler) ListDatabaseConnections(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	// Parse query parameters
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	search := c.QueryParam("search")
	dbType := c.QueryParam("type")
	isActive := c.QueryParam("active")

	// Build query
	query := h.db.Model(&models.DatabaseConnection{}).Where("user_id = ?", user.ID)

	// Apply filters
	if search != "" {
		// Use LIKE for broader database compatibility (SQLite doesn't support ILIKE)
		query = query.Where("LOWER(name) LIKE LOWER(?) OR LOWER(host) LIKE LOWER(?) OR LOWER(database) LIKE LOWER(?)",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if dbType != "" {
		query = query.Where("type = ?", dbType)
	}

	if isActive != "" {
		active, _ := strconv.ParseBool(isActive)
		query = query.Where("is_active = ?", active)
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get connections with pagination
	var connections []models.DatabaseConnection
	err := query.Preload("Tags").
		Offset((page - 1) * limit).
		Limit(limit).
		Order("created_at DESC").
		Find(&connections).Error

	if err != nil {
		return responses.InternalError(c, "Failed to fetch database connections")
	}

	// Convert to public format
	publicConnections := make([]*models.DatabaseConnectionPublic, len(connections))
	for i, conn := range connections {
		publicConnections[i] = conn.ToPublic()
	}

	paginationMeta := map[string]interface{}{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": (total + int64(limit) - 1) / int64(limit),
	}

	return responses.SuccessWithMeta(c, "Database connections retrieved successfully", publicConnections, paginationMeta)
}

// CreateDatabaseConnection handles POST /api/databases
func (h *DatabaseHandler) CreateDatabaseConnection(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	// Parse request
	var req models.ConnectionRequest
	if err := c.Bind(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, err.Error())
	}

	// Convert to model
	conn := req.ToModel()
	conn.UserID = user.ID

	// Set default values
	conn.SetDefaultValues()

	// Validate connection configuration
	if err := conn.ValidateConnection(); err != nil {
		return responses.Error(c, http.StatusBadRequest, err.Error())
	}

	// Encrypt credentials
	if err := conn.EncryptCredentials(h.encService); err != nil {
		return responses.InternalError(c, "Failed to encrypt credentials")
	}

	// Save to database
	if err := h.db.Create(conn).Error; err != nil {
		return responses.InternalError(c, "Failed to create database connection")
	}

	// Load tags if specified
	if len(req.TagIDs) > 0 {
		var tags []models.DatabaseTag
		h.db.Where("id IN ?", req.TagIDs).Find(&tags)
		conn.Tags = tags
		h.db.Save(conn)
	}

	return responses.Created(c, "Database connection created successfully", conn.ToPublic())
}

// GetDatabaseConnection handles GET /api/databases/:uid
func (h *DatabaseHandler) GetDatabaseConnection(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	uid := c.Param("uid")
	if uid == "" {
		return responses.Error(c, http.StatusBadRequest, "Connection UID is required")
	}

	var conn models.DatabaseConnection
	err := h.db.Preload("Tags").Preload("Tables").
		Where("uid = ? AND user_id = ?", uid, user.ID).
		First(&conn).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return responses.NotFound(c, "Database connection not found")
		}
		return responses.InternalError(c, "Failed to fetch database connection")
	}

	return responses.Success(c, "Database connection retrieved successfully", conn.ToPublic())
}

// UpdateDatabaseConnection handles PUT /api/databases/:uid
func (h *DatabaseHandler) UpdateDatabaseConnection(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	uid := c.Param("uid")
	if uid == "" {
		return responses.Error(c, http.StatusBadRequest, "Connection UID is required")
	}

	// Parse request
	var req models.ConnectionRequest
	if err := c.Bind(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, err.Error())
	}

	// Find existing connection
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return responses.NotFound(c, "Database connection not found")
		}
		return responses.InternalError(c, "Failed to fetch database connection")
	}

	// Update fields
	updatedConn := req.ToModel()
	updatedConn.ID = conn.ID
	updatedConn.UID = conn.UID
	updatedConn.UserID = conn.UserID
	updatedConn.CreatedAt = conn.CreatedAt

	// Validate updated connection
	if err := updatedConn.ValidateConnection(); err != nil {
		return responses.Error(c, http.StatusBadRequest, err.Error())
	}

	// Encrypt credentials
	if err := updatedConn.EncryptCredentials(h.encService); err != nil {
		return responses.InternalError(c, "Failed to encrypt credentials")
	}

	// Save to database
	if err := h.db.Save(updatedConn).Error; err != nil {
		return responses.InternalError(c, "Failed to update database connection")
	}

	// Update tags if specified
	if len(req.TagIDs) > 0 {
		var tags []models.DatabaseTag
		h.db.Where("id IN ?", req.TagIDs).Find(&tags)
		updatedConn.Tags = tags
		h.db.Save(updatedConn)
	}

	return responses.Success(c, "Database connection updated successfully", updatedConn.ToPublic())
}

// DeleteDatabaseConnection handles DELETE /api/databases/:uid
func (h *DatabaseHandler) DeleteDatabaseConnection(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	uid := c.Param("uid")
	if uid == "" {
		return responses.Error(c, http.StatusBadRequest, "Connection UID is required")
	}

	// Check if connection exists and belongs to user
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return responses.NotFound(c, "Database connection not found")
		}
		return responses.InternalError(c, "Failed to fetch database connection")
	}

	// Soft delete the connection
	if err := h.db.Delete(&conn).Error; err != nil {
		return responses.InternalError(c, "Failed to delete database connection")
	}

	return responses.Success(c, "Database connection deleted successfully", nil)
}

// TestDatabaseConnection handles POST /api/databases/:uid/test
func (h *DatabaseHandler) TestDatabaseConnection(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	uid := c.Param("uid")
	if uid == "" {
		return responses.Error(c, http.StatusBadRequest, "Connection UID is required")
	}

	// Find connection
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return responses.NotFound(c, "Database connection not found")
		}
		return responses.InternalError(c, "Failed to fetch database connection")
	}

	// Test connection
	result, err := h.dbService.TestConnection(c.Request().Context(), &conn)
	if err != nil {
		return responses.InternalError(c, "Failed to test connection")
	}

	// Update connection test result
	if err := h.dbService.UpdateConnectionTestResult(c.Request().Context(), conn.ID, result); err != nil {
		// Log error but don't fail the request
		c.Logger().Errorf("Failed to update connection test result: %v", err)
	}

	return responses.Success(c, "Database connection test completed", result)
}

// DiscoverTables handles POST /api/databases/:uid/discover
func (h *DatabaseHandler) DiscoverTables(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	uid := c.Param("uid")
	if uid == "" {
		return responses.Error(c, http.StatusBadRequest, "Connection UID is required")
	}

	// Parse request
	var req models.TableDiscoveryRequest
	if err := c.Bind(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return responses.Error(c, http.StatusBadRequest, err.Error())
	}

	// Find connection
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return responses.NotFound(c, "Database connection not found")
		}
		return responses.InternalError(c, "Failed to fetch database connection")
	}

	// Discover tables
	tables, err := h.dbService.DiscoverTables(c.Request().Context(), &conn, &req)
	if err != nil {
		return responses.InternalError(c, "Failed to discover tables: "+err.Error())
	}

	// Save discovered tables
	if err := h.dbService.SaveDiscoveredTables(c.Request().Context(), tables); err != nil {
		return responses.InternalError(c, "Failed to save discovered tables")
	}

	// Convert to public format
	publicTables := make([]*models.DatabaseTablePublic, len(tables))
	for i, table := range tables {
		publicTables[i] = table.ToPublic()
	}

	response := &models.TableDiscoveryResponse{
		Tables: publicTables,
		Count:  len(tables),
	}

	return responses.Success(c, "Tables discovered successfully", response)
}

// ListTables handles GET /api/databases/:uid/tables
func (h *DatabaseHandler) ListTables(c echo.Context) error {
	// Get authenticated user (guaranteed to exist after auth middleware)
	user := middleware.GetUserModel(c)

	uid := c.Param("uid")
	if uid == "" {
		return responses.Error(c, http.StatusBadRequest, "Connection UID is required")
	}

	// Parse query parameters for filtering
	includeViews, _ := strconv.ParseBool(c.QueryParam("include_views"))
	includeSystem, _ := strconv.ParseBool(c.QueryParam("include_system"))
	schemaPattern := c.QueryParam("schema_pattern")
	tablePattern := c.QueryParam("table_pattern")

	// Build discovery request
	req := &models.TableDiscoveryRequest{
		IncludeViews:  includeViews,
		IncludeSystem: includeSystem,
	}

	if schemaPattern != "" {
		req.SchemaPattern = &schemaPattern
	}
	if tablePattern != "" {
		req.TablePattern = &tablePattern
	}

	// Find connection
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return responses.NotFound(c, "Database connection not found")
		}
		return responses.InternalError(c, "Failed to fetch database connection")
	}

	// List tables dynamically (without saving to database)
	tables, err := h.dbService.DiscoverTables(c.Request().Context(), &conn, req)
	if err != nil {
		return responses.InternalError(c, "Failed to list tables: "+err.Error())
	}

	// Convert to public format
	publicTables := make([]*models.DatabaseTablePublic, len(tables))
	for i, table := range tables {
		publicTables[i] = table.ToPublic()
	}

	response := &models.TableListResponse{
		Tables: publicTables,
		Count:  len(tables),
		Filters: models.TableListFilters{
			IncludeViews:  includeViews,
			IncludeSystem: includeSystem,
			SchemaPattern: schemaPattern,
			TablePattern:  tablePattern,
		},
	}

	return responses.Success(c, "Tables listed successfully", response)
}

// DatabaseStats returns database connection statistics
func DatabaseStats(c echo.Context) error {
	stats, err := database.GetStats()
	if err != nil {
		return responses.InternalError(c, "Failed to get database statistics")
	}

	return responses.Success(c, "Database statistics retrieved successfully", stats)
}
