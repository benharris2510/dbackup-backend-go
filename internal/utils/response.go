package utils

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// APIResponse represents the standardized API response format
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// SuccessResponse creates a success response
func SuccessResponse(c echo.Context, message string, data interface{}) error {
	return c.JSON(http.StatusOK, APIResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

// SuccessResponseWithMeta creates a success response with metadata
func SuccessResponseWithMeta(c echo.Context, message string, data interface{}, meta interface{}) error {
	return c.JSON(http.StatusOK, APIResponse{
		Status:  "success",
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// ErrorResponse creates an error response
func ErrorResponse(c echo.Context, statusCode int, message string) error {
	return c.JSON(statusCode, APIResponse{
		Status:  "error",
		Message: message,
	})
}

// ErrorResponseWithData creates an error response with additional data
func ErrorResponseWithData(c echo.Context, statusCode int, message string, data interface{}) error {
	return c.JSON(statusCode, APIResponse{
		Status:  "error",
		Message: message,
		Data:    data,
	})
}

// ValidationErrorResponse creates a validation error response
func ValidationErrorResponse(c echo.Context, message string, errors map[string]string) error {
	return c.JSON(http.StatusBadRequest, APIResponse{
		Status:  "error",
		Message: message,
		Data: map[string]interface{}{
			"errors": errors,
		},
	})
}

// CreatedResponse creates a 201 Created response
func CreatedResponse(c echo.Context, message string, data interface{}) error {
	return c.JSON(http.StatusCreated, APIResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

// NoContentResponse creates a 204 No Content response
func NoContentResponse(c echo.Context, message string) error {
	return c.JSON(http.StatusNoContent, APIResponse{
		Status:  "success",
		Message: message,
	})
}

// UnauthorizedResponse creates a 401 Unauthorized response
func UnauthorizedResponse(c echo.Context, message string) error {
	return ErrorResponse(c, http.StatusUnauthorized, message)
}

// ForbiddenResponse creates a 403 Forbidden response
func ForbiddenResponse(c echo.Context, message string) error {
	return ErrorResponse(c, http.StatusForbidden, message)
}

// NotFoundResponse creates a 404 Not Found response
func NotFoundResponse(c echo.Context, message string) error {
	return ErrorResponse(c, http.StatusNotFound, message)
}

// ConflictResponse creates a 409 Conflict response
func ConflictResponse(c echo.Context, message string) error {
	return ErrorResponse(c, http.StatusConflict, message)
}

// InternalServerErrorResponse creates a 500 Internal Server Error response
func InternalServerErrorResponse(c echo.Context, message string) error {
	return ErrorResponse(c, http.StatusInternalServerError, message)
}