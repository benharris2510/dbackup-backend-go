package handlers

import (
	"net/http"
	"strconv"

	"github.com/dbackup/backend-go/internal/database"
	"github.com/dbackup/backend-go/internal/encryption"
	"github.com/dbackup/backend-go/internal/models"
	"github.com/dbackup/backend-go/internal/services"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// DatabaseHandler handles database connection management
type DatabaseHandler struct {
	db            *gorm.DB
	dbService     *services.DatabaseService
	encService    *encryption.Service
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
	// Get user from context (set by auth middleware)
	user, ok := c.Get("user").(*models.User)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

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
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch database connections")
	}

	// Convert to public format
	publicConnections := make([]*models.DatabaseConnectionPublic, len(connections))
	for i, conn := range connections {
		publicConnections[i] = conn.ToPublic()
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"connections": publicConnections,
			"pagination": map[string]interface{}{
				"page":        page,
				"limit":       limit,
				"total":       total,
				"total_pages": (total + int64(limit) - 1) / int64(limit),
			},
		},
	})
}

// CreateDatabaseConnection handles POST /api/databases
func (h *DatabaseHandler) CreateDatabaseConnection(c echo.Context) error {
	// Get user from context
	user, ok := c.Get("user").(*models.User)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	// Parse request
	var req models.ConnectionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Convert to model
	conn := req.ToModel()
	conn.UserID = user.ID

	// Set default values
	conn.SetDefaultValues()

	// Validate connection configuration
	if err := conn.ValidateConnection(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Encrypt credentials
	if err := conn.EncryptCredentials(h.encService); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to encrypt credentials")
	}

	// Save to database
	if err := h.db.Create(conn).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create database connection")
	}

	// Load tags if specified
	if len(req.TagIDs) > 0 {
		var tags []models.DatabaseTag
		h.db.Where("id IN ?", req.TagIDs).Find(&tags)
		conn.Tags = tags
		h.db.Save(conn)
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"status": "success",
		"data":   conn.ToPublic(),
	})
}

// GetDatabaseConnection handles GET /api/databases/:uid
func (h *DatabaseHandler) GetDatabaseConnection(c echo.Context) error {
	user, ok := c.Get("user").(*models.User)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	uid := c.Param("uid")
	if uid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Connection UID is required")
	}

	var conn models.DatabaseConnection
	err := h.db.Preload("Tags").Preload("Tables").
		Where("uid = ? AND user_id = ?", uid, user.ID).
		First(&conn).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Database connection not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch database connection")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   conn.ToPublic(),
	})
}

// UpdateDatabaseConnection handles PUT /api/databases/:uid
func (h *DatabaseHandler) UpdateDatabaseConnection(c echo.Context) error {
	user, ok := c.Get("user").(*models.User)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	uid := c.Param("uid")
	if uid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Connection UID is required")
	}

	// Parse request
	var req models.ConnectionRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Find existing connection
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Database connection not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch database connection")
	}

	// Update fields
	updatedConn := req.ToModel()
	updatedConn.ID = conn.ID
	updatedConn.UID = conn.UID
	updatedConn.UserID = conn.UserID
	updatedConn.CreatedAt = conn.CreatedAt

	// Validate updated connection
	if err := updatedConn.ValidateConnection(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Encrypt credentials
	if err := updatedConn.EncryptCredentials(h.encService); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to encrypt credentials")
	}

	// Save to database
	if err := h.db.Save(updatedConn).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update database connection")
	}

	// Update tags if specified
	if len(req.TagIDs) > 0 {
		var tags []models.DatabaseTag
		h.db.Where("id IN ?", req.TagIDs).Find(&tags)
		updatedConn.Tags = tags
		h.db.Save(updatedConn)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   updatedConn.ToPublic(),
	})
}

// DeleteDatabaseConnection handles DELETE /api/databases/:uid
func (h *DatabaseHandler) DeleteDatabaseConnection(c echo.Context) error {
	user, ok := c.Get("user").(*models.User)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	uid := c.Param("uid")
	if uid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Connection UID is required")
	}

	// Check if connection exists and belongs to user
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Database connection not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch database connection")
	}

	// Soft delete the connection
	if err := h.db.Delete(&conn).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete database connection")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Database connection deleted successfully",
	})
}

// TestDatabaseConnection handles POST /api/databases/:uid/test
func (h *DatabaseHandler) TestDatabaseConnection(c echo.Context) error {
	user, ok := c.Get("user").(*models.User)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	uid := c.Param("uid")
	if uid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Connection UID is required")
	}

	// Find connection
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Database connection not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch database connection")
	}

	// Test connection
	result, err := h.dbService.TestConnection(c.Request().Context(), &conn)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to test connection")
	}

	// Update connection test result
	if err := h.dbService.UpdateConnectionTestResult(c.Request().Context(), conn.ID, result); err != nil {
		// Log error but don't fail the request
		c.Logger().Errorf("Failed to update connection test result: %v", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   result,
	})
}

// DiscoverTables handles POST /api/databases/:uid/discover
func (h *DatabaseHandler) DiscoverTables(c echo.Context) error {
	user, ok := c.Get("user").(*models.User)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not authenticated")
	}

	uid := c.Param("uid")
	if uid == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Connection UID is required")
	}

	// Parse request
	var req models.TableDiscoveryRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request format")
	}

	// Validate request
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Find connection
	var conn models.DatabaseConnection
	err := h.db.Where("uid = ? AND user_id = ?", uid, user.ID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Database connection not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch database connection")
	}

	// Discover tables
	tables, err := h.dbService.DiscoverTables(c.Request().Context(), &conn, &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to discover tables: "+err.Error())
	}

	// Save discovered tables
	if err := h.dbService.SaveDiscoveredTables(c.Request().Context(), tables); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save discovered tables")
	}

	// Convert to public format
	publicTables := make([]*models.DatabaseTablePublic, len(tables))
	for i, table := range tables {
		publicTables[i] = table.ToPublic()
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"tables": publicTables,
			"count":  len(tables),
		},
	})
}

// DatabaseStats returns database connection statistics
func DatabaseStats(c echo.Context) error {
	stats, err := database.GetStats()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "Failed to get database statistics",
			"details": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   stats,
	})
}