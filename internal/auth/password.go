package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash         = errors.New("invalid hash format")
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
)

// PasswordHasher provides password hashing and verification
type PasswordHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// NewPasswordHasher creates a new password hasher with default settings
func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{
		memory:      64 * 1024, // 64 MB
		iterations:  3,
		parallelism: 2,
		saltLength:  16,
		keyLength:   32,
	}
}

// NewCustomPasswordHasher creates a password hasher with custom settings
func NewCustomPasswordHasher(memory uint32, iterations uint32, parallelism uint8, saltLength uint32, keyLength uint32) *PasswordHasher {
	return &PasswordHasher{
		memory:      memory,
		iterations:  iterations,
		parallelism: parallelism,
		saltLength:  saltLength,
		keyLength:   keyLength,
	}
}

// HashPassword hashes a password using Argon2id
func (ph *PasswordHasher) HashPassword(password string) (string, error) {
	// Generate a random salt
	salt, err := generateRandomBytes(ph.saltLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Generate the hash using Argon2id
	hash := argon2.IDKey([]byte(password), salt, ph.iterations, ph.memory, ph.parallelism, ph.keyLength)

	// Encode the salt and hash to base64
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Return the encoded hash in the format:
	// $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, ph.memory, ph.iterations, ph.parallelism, b64Salt, b64Hash)

	return encodedHash, nil
}

// VerifyPassword verifies a password against a hash
func (ph *PasswordHasher) VerifyPassword(password, encodedHash string) (bool, error) {
	// Extract the parameters, salt and derived key from the encoded password hash
	memory, iterations, parallelism, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	// Derive the key from the other password using the same parameters
	otherHash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(hash)))

	// Check that the contents of the hashed passwords are identical
	if subtle.ConstantTimeCompare(hash, otherHash) == 1 {
		return true, nil
	}
	return false, nil
}

// generateRandomBytes generates random bytes of the specified length
func generateRandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// decodeHash decodes the encoded hash and extracts the parameters
func decodeHash(encodedHash string) (memory uint32, iterations uint32, parallelism uint8, salt, hash []byte, err error) {
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}

	var version int
	_, err = fmt.Sscanf(vals[2], "v=%d", &version)
	if err != nil {
		return 0, 0, 0, nil, nil, err
	}
	if version != argon2.Version {
		return 0, 0, 0, nil, nil, ErrIncompatibleVersion
	}

	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism)
	if err != nil {
		return 0, 0, 0, nil, nil, err
	}

	salt, err = base64.RawStdEncoding.DecodeString(vals[4])
	if err != nil {
		return 0, 0, 0, nil, nil, err
	}

	hash, err = base64.RawStdEncoding.DecodeString(vals[5])
	if err != nil {
		return 0, 0, 0, nil, nil, err
	}

	return memory, iterations, parallelism, salt, hash, nil
}

// ValidatePasswordStrength validates password strength requirements
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	if len(password) > 128 {
		return errors.New("password must be no more than 128 characters long")
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		case char >= 32 && char <= 126: // Printable ASCII characters
			if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')) {
				hasSpecial = true
			}
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}

	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}

	if !hasNumber {
		return errors.New("password must contain at least one number")
	}

	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

// GenerateRandomPassword generates a cryptographically secure random password
func GenerateRandomPassword(length int) (string, error) {
	if length < 8 {
		return "", errors.New("password length must be at least 8 characters")
	}

	if length > 128 {
		return "", errors.New("password length must be no more than 128 characters")
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;:,.<>?"
	
	password := make([]byte, length)
	for i := range password {
		randomIndex, err := randomInt(len(charset))
		if err != nil {
			return "", fmt.Errorf("failed to generate random password: %w", err)
		}
		password[i] = charset[randomIndex]
	}

	// Ensure the generated password meets strength requirements
	if err := ValidatePasswordStrength(string(password)); err != nil {
		// If it doesn't meet requirements, try again (recursive call with small chance of infinite loop)
		return GenerateRandomPassword(length)
	}

	return string(password), nil
}

// randomInt generates a cryptographically secure random integer in the range [0, max)
func randomInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("max must be positive")
	}

	// Calculate the number of bits needed to represent max-1
	bitLen := 0
	for n := max - 1; n > 0; n >>= 1 {
		bitLen++
	}

	// Calculate the number of bytes needed
	byteLen := (bitLen + 7) / 8

	// Generate random bytes
	randomBytes := make([]byte, byteLen)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return 0, err
	}

	// Convert bytes to integer
	var randomValue int
	for i, b := range randomBytes {
		randomValue |= int(b) << (8 * i)
	}

	// Mask to get only the bits we need
	mask := (1 << bitLen) - 1
	randomValue &= mask

	// If the result is >= max, try again
	if randomValue >= max {
		return randomInt(max)
	}

	return randomValue, nil
}

// EstimateHashTime estimates the time it takes to hash a password with given parameters
func EstimateHashTime(password string, memory uint32, iterations uint32, parallelism uint8, keyLength uint32) (time.Duration, error) {
	// Generate a small salt for testing
	salt, err := generateRandomBytes(16)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	_ = argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	duration := time.Since(start)

	return duration, nil
}

// GetHashParameters extracts the hashing parameters from an encoded hash
func GetHashParameters(encodedHash string) (memory uint32, iterations uint32, parallelism uint8, err error) {
	memory, iterations, parallelism, _, _, err = decodeHash(encodedHash)
	return
}

// CompareHashParameters compares two sets of hash parameters
func CompareHashParameters(encodedHash1, encodedHash2 string) (bool, error) {
	memory1, iterations1, parallelism1, err := GetHashParameters(encodedHash1)
	if err != nil {
		return false, err
	}

	memory2, iterations2, parallelism2, err := GetHashParameters(encodedHash2)
	if err != nil {
		return false, err
	}

	return memory1 == memory2 && iterations1 == iterations2 && parallelism1 == parallelism2, nil
}

// NeedsRehash checks if a password hash needs to be rehashed with updated parameters
func (ph *PasswordHasher) NeedsRehash(encodedHash string) (bool, error) {
	memory, iterations, parallelism, err := GetHashParameters(encodedHash)
	if err != nil {
		return false, err
	}

	return memory != ph.memory || iterations != ph.iterations || parallelism != ph.parallelism, nil
}