package responses

import (
	"net/http"
	"reflect"
	"time"

	"github.com/labstack/echo/v4"
)

// APISerializable interface for models that can customize their API representation
type APISerializable interface {
	SerializeForAPI() map[string]interface{}
}

// APIResponse represents the standardized API response format
type StandardAPIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// SerializeData automatically serializes data for API responses
func SerializeData(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	// If the model implements APISerializable, use that
	if serializable, ok := data.(APISerializable); ok {
		return serializable.SerializeForAPI()
	}

	// Handle slices of models
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Slice {
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			element := v.Index(i).Interface()
			// Recursively serialize each element (this will use SerializeForAPI if available)
			result[i] = SerializeData(element)
		}
		return result
	}

	// Handle pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		return SerializeData(v.Elem().Interface())
	}

	// Auto-serialize structs using reflection and json tags
	if v.Kind() == reflect.Struct {
		return autoSerializeStruct(data)
	}

	// Return primitive types as-is
	return data
}

// autoSerializeStruct automatically serializes a struct based on json tags and visibility
func autoSerializeStruct(data interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	v := reflect.ValueOf(data)
	t := reflect.TypeOf(data)

	// Handle pointers
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return result
		}
		v = v.Elem()
		t = t.Elem()
	}

	if v.Kind() != reflect.Struct {
		return result
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get JSON tag
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "-" {
			continue // Skip fields with json:"-"
		}

		// Determine field name
		fieldName := fieldType.Name
		if jsonTag != "" {
			// Extract field name from json tag (before comma)
			if commaIdx := findCommaIndex(jsonTag); commaIdx > 0 {
				fieldName = jsonTag[:commaIdx]
			} else if jsonTag != "" {
				fieldName = jsonTag
			}
		}

		// Convert field name to snake_case if it's not already specified in json tag
		if jsonTag == "" {
			fieldName = toSnakeCase(fieldName)
		}

		// Get field value
		fieldValue := field.Interface()

		// Handle time.Time specially
		if t, ok := fieldValue.(time.Time); ok {
			fieldValue = t.Format("2006-01-02T15:04:05Z")
		}

		// Handle pointers to time.Time
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			if t, ok := field.Elem().Interface().(time.Time); ok {
				fieldValue = t.Format("2006-01-02T15:04:05Z")
			} else {
				fieldValue = field.Elem().Interface()
			}
		}

		// Handle nil pointers
		if field.Kind() == reflect.Ptr && field.IsNil() {
			// Check if json tag has omitempty
			if hasOmitEmpty(jsonTag) {
				continue
			}
			fieldValue = nil
		}

		result[fieldName] = fieldValue
	}

	return result
}

// Helper functions
func findCommaIndex(s string) int {
	for i, r := range s {
		if r == ',' {
			return i
		}
	}
	return -1
}

func hasOmitEmpty(jsonTag string) bool {
	return jsonTag != "" && (jsonTag == "omitempty" || 
		(findCommaIndex(jsonTag) > 0 && jsonTag[findCommaIndex(jsonTag)+1:] == "omitempty"))
}

func toSnakeCase(s string) string {
	if len(s) == 0 {
		return s
	}
	
	result := make([]rune, 0, len(s))
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		if r >= 'A' && r <= 'Z' {
			result = append(result, r-'A'+'a')
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// Updated response functions that automatically serialize data
func ApiResponse(c echo.Context, statusCode int, status, message string, data interface{}, meta interface{}) error {
	response := StandardAPIResponse{
		Status:  status,
		Message: message,
		Data:    SerializeData(data),
	}
	
	if meta != nil {
		response.Meta = SerializeData(meta)
	}
	
	return c.JSON(statusCode, response)
}

// Convenience functions
func Success(c echo.Context, message string, data interface{}) error {
	return ApiResponse(c, http.StatusOK, "success", message, data, nil)
}

func SuccessWithMeta(c echo.Context, message string, data interface{}, meta interface{}) error {
	return ApiResponse(c, http.StatusOK, "success", message, data, meta)
}

func Created(c echo.Context, message string, data interface{}) error {
	return ApiResponse(c, http.StatusCreated, "success", message, data, nil)
}

func Error(c echo.Context, statusCode int, message string) error {
	return ApiResponse(c, statusCode, "error", message, nil, nil)
}

func ErrorWithData(c echo.Context, statusCode int, message string, data interface{}) error {
	return ApiResponse(c, statusCode, "error", message, data, nil)
}

func ValidationError(c echo.Context, message string, errors map[string]string) error {
	return ErrorWithData(c, http.StatusBadRequest, message, map[string]interface{}{
		"errors": errors,
	})
}

func Unauthorized(c echo.Context, message string) error {
	return Error(c, http.StatusUnauthorized, message)
}

func NotFound(c echo.Context, message string) error {
	return Error(c, http.StatusNotFound, message)
}

func InternalError(c echo.Context, message string) error {
	return Error(c, http.StatusInternalServerError, message)
}