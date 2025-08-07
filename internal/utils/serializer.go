package utils

import (
	"reflect"
	"time"
)

// APIField defines how a field should be serialized in API responses
type APIField struct {
	Name      string      `json:"name,omitempty"`      // JSON field name (if different from struct field)
	Omit      bool        `json:"omit,omitempty"`      // Whether to omit this field
	Transform string      `json:"transform,omitempty"` // Transform function to apply
	Format    string      `json:"format,omitempty"`    // Format for dates/times
}

// Serializable interface for models that can be serialized for API responses
type Serializable interface {
	GetAPIFields() map[string]APIField
}

// SerializeForAPI converts a model to a map for API responses using struct tags and configuration
func SerializeForAPI(model interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Use reflection to get struct fields
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	if v.Kind() != reflect.Struct {
		return result
	}
	
	t := v.Type()
	
	// Get field configuration if model implements Serializable
	var fieldConfig map[string]APIField
	if serializable, ok := model.(Serializable); ok {
		fieldConfig = serializable.GetAPIFields()
	}
	
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		
		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}
		
		// Get JSON tag name
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "-" {
			continue // Skip fields with json:"-"
		}
		
		fieldName := fieldType.Name
		if jsonTag != "" && jsonTag != "-" {
			// Extract field name from json tag (before comma)
			if idx := len(jsonTag); idx > 0 {
				if commaIdx := 0; commaIdx < len(jsonTag) {
					for j, r := range jsonTag {
						if r == ',' {
							commaIdx = j
							break
						}
					}
					if commaIdx > 0 {
						fieldName = jsonTag[:commaIdx]
					} else {
						fieldName = jsonTag
					}
				}
			}
		}
		
		// Check field configuration
		config, hasConfig := fieldConfig[fieldType.Name]
		if hasConfig && config.Omit {
			continue
		}
		
		// Use configured name if provided
		if hasConfig && config.Name != "" {
			fieldName = config.Name
		}
		
		// Get field value
		fieldValue := field.Interface()
		
		// Apply transformations
		if hasConfig {
			fieldValue = applyTransformations(fieldValue, config)
		}
		
		// Handle time formatting
		if t, ok := fieldValue.(time.Time); ok {
			if hasConfig && config.Format != "" {
				fieldValue = t.Format(config.Format)
			} else {
				fieldValue = t.Format("2006-01-02T15:04:05Z")
			}
		}
		
		// Handle pointers
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			fieldValue = field.Elem().Interface()
		}
		
		result[fieldName] = fieldValue
	}
	
	return result
}

// applyTransformations applies configured transformations to field values
func applyTransformations(value interface{}, config APIField) interface{} {
	switch config.Transform {
	case "full_name":
		// Assuming this is applied to a User struct with FirstName and LastName
		// This would need to be handled differently in a real implementation
		return value
	case "omit_if_empty":
		if isEmpty(value) {
			return nil
		}
		return value
	default:
		return value
	}
}

// isEmpty checks if a value is considered empty
func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}