package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// CORS returns a configured CORS middleware
func CORS() echo.MiddlewareFunc {
	allowedOrigins := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	if len(allowedOrigins) == 0 || allowedOrigins[0] == "" {
		allowedOrigins = []string{"http://localhost:3000"}
	}

	allowedMethods := strings.Split(os.Getenv("CORS_ALLOWED_METHODS"), ",")
	if len(allowedMethods) == 0 || allowedMethods[0] == "" {
		allowedMethods = []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodPatch,
			http.MethodOptions,
		}
	}

	allowedHeaders := strings.Split(os.Getenv("CORS_ALLOWED_HEADERS"), ",")
	if len(allowedHeaders) == 0 || allowedHeaders[0] == "" {
		allowedHeaders = []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			echo.HeaderXRequestedWith,
			echo.HeaderXRequestID,
		}
	}

	allowCredentials := os.Getenv("CORS_ALLOW_CREDENTIALS") == "true"

	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     allowedMethods,
		AllowHeaders:     allowedHeaders,
		AllowCredentials: allowCredentials,
		ExposeHeaders: []string{
			echo.HeaderContentLength,
			echo.HeaderXRequestID,
		},
		MaxAge: 86400, // 24 hours
	})
}