package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORS(t *testing.T) {
	tests := []struct {
		name               string
		envVars            map[string]string
		origin             string
		method             string
		expectedAllowed    bool
		expectedHeaders    map[string]string
	}{
		{
			name: "Default configuration allows localhost:3000",
			envVars: map[string]string{},
			origin: "http://localhost:3000",
			method: http.MethodGet,
			expectedAllowed: true,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "http://localhost:3000",
			},
		},
		{
			name: "Custom allowed origins",
			envVars: map[string]string{
				"CORS_ALLOWED_ORIGINS": "http://example.com,https://app.example.com",
			},
			origin: "http://example.com",
			method: http.MethodPost,
			expectedAllowed: true,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "http://example.com",
			},
		},
		{
			name: "Disallowed origin",
			envVars: map[string]string{
				"CORS_ALLOWED_ORIGINS": "http://example.com",
			},
			origin: "http://notallowed.com",
			method: http.MethodGet,
			expectedAllowed: false,
			expectedHeaders: map[string]string{},
		},
		{
			name: "Preflight request",
			envVars: map[string]string{
				"CORS_ALLOWED_ORIGINS": "http://localhost:3000",
				"CORS_ALLOWED_METHODS": "GET,POST,PUT,DELETE",
				"CORS_ALLOWED_HEADERS": "Content-Type,Authorization",
			},
			origin: "http://localhost:3000",
			method: http.MethodOptions,
			expectedAllowed: true,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "http://localhost:3000",
				"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE",
			},
		},
		{
			name: "With credentials",
			envVars: map[string]string{
				"CORS_ALLOWED_ORIGINS": "http://localhost:3000",
				"CORS_ALLOW_CREDENTIALS": "true",
			},
			origin: "http://localhost:3000",
			method: http.MethodGet,
			expectedAllowed: true,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "http://localhost:3000",
				"Access-Control-Allow-Credentials": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Setup Echo
			e := echo.New()
			req := httptest.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			if tt.method == http.MethodOptions {
				req.Header.Set("Access-Control-Request-Method", "POST")
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Apply CORS middleware
			handler := CORS()(func(c echo.Context) error {
				return c.String(http.StatusOK, "OK")
			})

			// Execute handler
			err := handler(c)
			require.NoError(t, err)

			// Check response
			if tt.expectedAllowed {
				if tt.method == http.MethodOptions {
					assert.Equal(t, http.StatusNoContent, rec.Code)
				} else {
					assert.Equal(t, http.StatusOK, rec.Code)
				}
				
				// Check expected headers
				for header, value := range tt.expectedHeaders {
					assert.Equal(t, value, rec.Header().Get(header))
				}
			} else {
				// When origin is not allowed, check that CORS headers are not set
				assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSWithEmptyEnvironment(t *testing.T) {
	// Clear all CORS environment variables
	os.Unsetenv("CORS_ALLOWED_ORIGINS")
	os.Unsetenv("CORS_ALLOWED_METHODS")
	os.Unsetenv("CORS_ALLOWED_HEADERS")
	os.Unsetenv("CORS_ALLOW_CREDENTIALS")

	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Apply CORS middleware
	handler := CORS()(func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Execute
	err := handler(c)
	require.NoError(t, err)

	// Assert default behavior
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
}