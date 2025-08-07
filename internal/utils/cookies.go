package utils

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// CookieSettings holds cookie configuration
type CookieSettings struct {
	Name     string
	Domain   string
	Path     string
	MaxAge   int
	HTTPOnly bool
	Secure   bool
	SameSite http.SameSite
}

// DefaultAccessTokenCookieSettings returns default settings for access token cookie
func DefaultAccessTokenCookieSettings() CookieSettings {
	return CookieSettings{
		Name:     "access_token",
		Domain:   "",
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60, // 7 days in seconds
		HTTPOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}
}

// DefaultRefreshTokenCookieSettings returns default settings for refresh token cookie
func DefaultRefreshTokenCookieSettings() CookieSettings {
	return CookieSettings{
		Name:     "refresh_token",
		Domain:   "",
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days in seconds
		HTTPOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}
}

// SetTokenCookies sets both access and refresh token cookies
func SetTokenCookies(c echo.Context, accessToken, refreshToken string) {
	SetTokenCookie(c, accessToken, DefaultAccessTokenCookieSettings())
	SetTokenCookie(c, refreshToken, DefaultRefreshTokenCookieSettings())
}

// SetTokenCookie sets a single token cookie
func SetTokenCookie(c echo.Context, token string, settings CookieSettings) {
	cookie := &http.Cookie{
		Name:     settings.Name,
		Value:    token,
		Domain:   settings.Domain,
		Path:     settings.Path,
		MaxAge:   settings.MaxAge,
		HttpOnly: settings.HTTPOnly,
		Secure:   settings.Secure,
		SameSite: settings.SameSite,
	}
	c.SetCookie(cookie)
}

// ClearTokenCookies clears both access and refresh token cookies
func ClearTokenCookies(c echo.Context) {
	ClearTokenCookie(c, "access_token")
	ClearTokenCookie(c, "refresh_token")
}

// ClearTokenCookie clears a single token cookie
func ClearTokenCookie(c echo.Context, name string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Expires:  time.Now().Add(-24 * time.Hour),
	}
	c.SetCookie(cookie)
}

// GetTokenFromCookie retrieves token from cookie
func GetTokenFromCookie(c echo.Context, name string) (string, error) {
	cookie, err := c.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// GetAccessToken retrieves access token from cookie
func GetAccessToken(c echo.Context) (string, error) {
	return GetTokenFromCookie(c, "access_token")
}

// GetRefreshToken retrieves refresh token from cookie
func GetRefreshToken(c echo.Context) (string, error) {
	return GetTokenFromCookie(c, "refresh_token")
}