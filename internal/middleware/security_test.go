package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name            string
		useTLS          bool
		expectedHeaders map[string]string
	}{
		{
			name:   "HTTP request",
			useTLS: false,
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
				"X-XSS-Protection":       "1; mode=block",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
			},
		},
		{
			name:   "HTTPS request",
			useTLS: true,
			expectedHeaders: map[string]string{
				"X-Content-Type-Options":    "nosniff",
				"X-Frame-Options":           "DENY",
				"X-XSS-Protection":          "1; mode=block",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"Permissions-Policy":        "geolocation=(), microphone=(), camera=()",
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.useTLS {
				req.TLS = &tls.ConnectionState{}
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Apply SecurityHeaders middleware
			handler := SecurityHeaders()(func(c echo.Context) error {
				return c.String(http.StatusOK, "OK")
			})

			// Execute handler
			err := handler(c)
			require.NoError(t, err)

			// Check response
			assert.Equal(t, http.StatusOK, rec.Code)

			// Check expected headers
			for header, value := range tt.expectedHeaders {
				assert.Equal(t, value, rec.Header().Get(header), "Header %s mismatch", header)
			}

			// Check Content-Security-Policy header
			csp := rec.Header().Get("Content-Security-Policy")
			assert.Contains(t, csp, "default-src 'self'")
			assert.Contains(t, csp, "script-src 'self' 'unsafe-inline' 'unsafe-eval'")
			assert.Contains(t, csp, "style-src 'self' 'unsafe-inline'")
			assert.Contains(t, csp, "img-src 'self' data: https:")
			assert.Contains(t, csp, "font-src 'self' data:")
			assert.Contains(t, csp, "connect-src 'self' https://api.stripe.com")
			assert.Contains(t, csp, "frame-ancestors 'none'")

			// Ensure HSTS is only set for HTTPS
			if !tt.useTLS {
				assert.Empty(t, rec.Header().Get("Strict-Transport-Security"))
			}
		})
	}
}

func TestSecurityHeadersMiddlewareChain(t *testing.T) {
	// Test that middleware properly calls the next handler
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := SecurityHeaders()(func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "Handler executed")
	})

	err := handler(c)
	require.NoError(t, err)
	assert.True(t, handlerCalled, "Next handler should be called")
	assert.Equal(t, "Handler executed", rec.Body.String())
}