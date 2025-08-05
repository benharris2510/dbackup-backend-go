package models

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// generateUID generates a unique identifier
func generateUID() string {
	return uuid.New().String()
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	
	// Convert to hex string
	return fmt.Sprintf("%x", bytes), nil
}

// generateBackupCode generates a backup code for 2FA
func generateBackupCode() (string, error) {
	// Generate 8 groups of 4 characters each
	groups := make([]string, 8)
	for i := 0; i < 8; i++ {
		bytes := make([]byte, 2)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		groups[i] = fmt.Sprintf("%04X", (int(bytes[0])<<8)|int(bytes[1]))
	}
	
	return strings.Join(groups, "-"), nil
}

// validateEmail performs basic email validation
func validateEmail(email string) bool {
	if len(email) < 5 { // minimum: a@b.c
		return false
	}
	
	// Must contain exactly one @
	atCount := strings.Count(email, "@")
	if atCount != 1 {
		return false
	}
	
	// Split on @ to get local and domain parts
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	
	local, domain := parts[0], parts[1]
	
	// Local part cannot be empty
	if len(local) == 0 {
		return false
	}
	
	// Domain part must contain at least one dot and cannot be empty
	if len(domain) == 0 || !strings.Contains(domain, ".") {
		return false
	}
	
	// Domain cannot start or end with a dot
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	
	return true
}

// normalizeEmail normalizes an email address
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}