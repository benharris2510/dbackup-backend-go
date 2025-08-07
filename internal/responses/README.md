# API Responses Package

This package provides standardized API response utilities for the dbackup backend.

## Usage Examples

### Success Responses

```go
import "github.com/dbackup/backend-go/internal/responses"

// Simple success
return responses.Success(c, "User updated successfully", &user)

// Created response (201)
return responses.Created(c, "Database connection created", &dbConnection)

// Success with metadata
return responses.SuccessWithMeta(c, "Users retrieved", users, map[string]interface{}{
    "pagination": paginationInfo,
})
```

### Error Responses

```go
// Simple error
return responses.Error(c, http.StatusBadRequest, "Invalid input")

// Common shortcuts
return responses.Unauthorized(c, "Authentication required")
return responses.NotFound(c, "Resource not found")
return responses.InternalError(c, "Database connection failed")

// Validation errors
return responses.ValidationError(c, "Validation failed", map[string]string{
    "email": "Email is required",
    "password": "Password must be at least 8 characters",
})

// Error with additional data
return responses.ErrorWithData(c, http.StatusConflict, "Duplicate entry", map[string]interface{}{
    "existing_id": existingRecord.ID,
})
```

### Automatic Model Serialization

Models that implement the `APISerializable` interface will be automatically serialized:

```go
type User struct {
    ID       uint   `json:"id"`
    Email    string `json:"email"`
    Password string `json:"-"` // Hidden from API
}

func (u *User) SerializeForAPI() map[string]interface{} {
    return map[string]interface{}{
        "id":         u.ID,
        "email":      u.Email,
        "full_name":  u.GetFullName(),
        "created_at": u.CreatedAt.Format("2006-01-02T15:04:05Z"),
    }
}

// In handler
user := &User{ID: 1, Email: "user@example.com"}
return responses.Success(c, "User retrieved", user)
// Automatically calls user.SerializeForAPI()
```

## Response Format

All responses follow this standardized format:

```json
{
  "status": "success|error",
  "message": "Human readable message",
  "data": {}, // Optional - serialized data
  "meta": {}  // Optional - metadata like pagination
}
```