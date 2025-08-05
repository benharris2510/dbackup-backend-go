package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidTokenType = errors.New("invalid token type")
	ErrTokenMalformed   = errors.New("token is malformed")
	ErrInvalidSignature = errors.New("invalid token signature")
)

// TokenType represents the type of JWT token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims represents the JWT claims structure
type Claims struct {
	UserID    uint      `json:"user_id"`
	Email     string    `json:"email"`
	TokenType TokenType `json:"token_type"`
	TeamID    *uint     `json:"team_id,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token operations
type JWTManager struct {
	secretKey            []byte
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey string, accessTokenDuration, refreshTokenDuration time.Duration) *JWTManager {
	return &JWTManager{
		secretKey:            []byte(secretKey),
		accessTokenDuration:  accessTokenDuration,
		refreshTokenDuration: refreshTokenDuration,
	}
}

// GenerateAccessToken generates an access token for a user
func (jm *JWTManager) GenerateAccessToken(userID uint, email string, teamID *uint) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		Email:     email,
		TokenType: TokenTypeAccess,
		TeamID:    teamID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // Add unique ID to ensure tokens are different
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jm.accessTokenDuration)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "dbackup-api",
			Subject:   fmt.Sprintf("user:%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jm.secretKey)
}

// GenerateRefreshToken generates a refresh token for a user
func (jm *JWTManager) GenerateRefreshToken(userID uint, email string, teamID *uint) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		Email:     email,
		TokenType: TokenTypeRefresh,
		TeamID:    teamID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // Add unique ID to ensure tokens are different
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jm.refreshTokenDuration)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "dbackup-api",
			Subject:   fmt.Sprintf("user:%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jm.secretKey)
}

// GenerateTokenPair generates both access and refresh tokens
func (jm *JWTManager) GenerateTokenPair(userID uint, email string, teamID *uint) (accessToken, refreshToken string, err error) {
	accessToken, err = jm.GenerateAccessToken(userID, email, teamID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err = jm.GenerateRefreshToken(userID, email, teamID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// ValidateToken validates a JWT token and returns the claims
func (jm *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jm.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrTokenMalformed
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrInvalidSignature
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ValidateAccessToken validates an access token specifically
func (jm *JWTManager) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims, err := jm.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != TokenTypeAccess {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token specifically
func (jm *JWTManager) ValidateRefreshToken(tokenString string) (*Claims, error) {
	claims, err := jm.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != TokenTypeRefresh {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token using a valid refresh token
func (jm *JWTManager) RefreshAccessToken(refreshTokenString string) (string, error) {
	claims, err := jm.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return "", err
	}

	// Generate new access token with the same user information
	return jm.GenerateAccessToken(claims.UserID, claims.Email, claims.TeamID)
}

// ExtractTokenFromHeader extracts JWT token from Authorization header
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header is required")
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) {
		return "", errors.New("invalid authorization header format")
	}

	if authHeader[:len(bearerPrefix)] != bearerPrefix {
		return "", errors.New("authorization header must start with 'Bearer '")
	}

	token := strings.TrimSpace(authHeader[len(bearerPrefix):])
	if token == "" {
		return "", errors.New("token is required")
	}

	return token, nil
}

// GetTokenDuration returns the duration for a specific token type
func (jm *JWTManager) GetTokenDuration(tokenType TokenType) time.Duration {
	switch tokenType {
	case TokenTypeAccess:
		return jm.accessTokenDuration
	case TokenTypeRefresh:
		return jm.refreshTokenDuration
	default:
		return 0
	}
}

// GetSecretKey returns the secret key (for creating new managers with different durations)
func (jm *JWTManager) GetSecretKey() []byte {
	return jm.secretKey
}

// IsTokenExpired checks if a token is expired without validating the signature
func IsTokenExpired(tokenString string) bool {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return true
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return true
	}

	return claims.ExpiresAt.Time.Before(time.Now())
}

// GetTokenClaims extracts claims from a token without validating the signature
func GetTokenClaims(tokenString string) (*Claims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims format")
	}

	return claims, nil
}

// TokenInfo represents information about a token
type TokenInfo struct {
	UserID    uint      `json:"user_id"`
	Email     string    `json:"email"`
	TokenType TokenType `json:"token_type"`
	TeamID    *uint     `json:"team_id,omitempty"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	IsExpired bool      `json:"is_expired"`
}

// GetTokenInfo returns detailed information about a token
func (jm *JWTManager) GetTokenInfo(tokenString string) (*TokenInfo, error) {
	claims, err := GetTokenClaims(tokenString)
	if err != nil {
		return nil, err
	}

	return &TokenInfo{
		UserID:    claims.UserID,
		Email:     claims.Email,
		TokenType: claims.TokenType,
		TeamID:    claims.TeamID,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
		Subject:   claims.Subject,
		Issuer:    claims.Issuer,
		IsExpired: claims.ExpiresAt.Time.Before(time.Now()),
	}, nil
}

// RevokedTokenStore interface for managing revoked tokens
type RevokedTokenStore interface {
	RevokeToken(tokenID string, expiresAt time.Time) error
	IsTokenRevoked(tokenID string) bool
	CleanupExpiredTokens() error
}

// TokenManager extends JWTManager with token revocation capabilities
type TokenManager struct {
	*JWTManager
	revokedStore RevokedTokenStore
}

// NewTokenManager creates a new token manager with revocation support
func NewTokenManager(jwtManager *JWTManager, revokedStore RevokedTokenStore) *TokenManager {
	return &TokenManager{
		JWTManager:   jwtManager,
		revokedStore: revokedStore,
	}
}

// ValidateTokenWithRevocation validates a token and checks if it's revoked
func (tm *TokenManager) ValidateTokenWithRevocation(tokenString string) (*Claims, error) {
	claims, err := tm.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if token is revoked
	tokenID := claims.ID
	if tokenID != "" && tm.revokedStore.IsTokenRevoked(tokenID) {
		return nil, errors.New("token has been revoked")
	}

	return claims, nil
}

// RevokeToken revokes a token by adding it to the revoked tokens store
func (tm *TokenManager) RevokeToken(tokenString string) error {
	claims, err := GetTokenClaims(tokenString)
	if err != nil {
		return fmt.Errorf("failed to get token claims: %w", err)
	}

	tokenID := claims.ID
	if tokenID == "" {
		return errors.New("token does not have an ID")
	}

	return tm.revokedStore.RevokeToken(tokenID, claims.ExpiresAt.Time)
}

// CleanupExpiredTokens removes expired tokens from the revocation store
func (tm *TokenManager) CleanupExpiredTokens() error {
	return tm.revokedStore.CleanupExpiredTokens()
}