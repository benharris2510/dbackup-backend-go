package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// Service handles encryption and decryption of sensitive data
type Service struct {
	key []byte
}

// NewService creates a new encryption service with the provided key
func NewService(secretKey string) *Service {
	// Derive a 32-byte key from the secret using PBKDF2
	salt := []byte("dbackup-encryption-salt") // In production, use a random salt per app
	key := pbkdf2.Key([]byte(secretKey), salt, 10000, 32, sha256.New)
	
	return &Service{
		key: key,
	}
}

// Encrypt encrypts plaintext using AES-256-GCM
func (s *Service) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 for storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (s *Service) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check minimum length
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EncryptMap encrypts all values in a map
func (s *Service) EncryptMap(data map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	
	for key, value := range data {
		encrypted, err := s.Encrypt(value)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt key %s: %w", key, err)
		}
		result[key] = encrypted
	}
	
	return result, nil
}

// DecryptMap decrypts all values in a map
func (s *Service) DecryptMap(data map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	
	for key, value := range data {
		decrypted, err := s.Decrypt(value)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt key %s: %w", key, err)
		}
		result[key] = decrypted
	}
	
	return result, nil
}

// RotateKey creates a new service with a new key and provides methods for migration
func (s *Service) RotateKey(newSecretKey string) *Service {
	return NewService(newSecretKey)
}

// ReencryptWithNewKey re-encrypts data with a new key
func (s *Service) ReencryptWithNewKey(ciphertext string, newService *Service) (string, error) {
	// Decrypt with old key
	plaintext, err := s.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt with old key: %w", err)
	}
	
	// Encrypt with new key
	newCiphertext, err := newService.Encrypt(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt with new key: %w", err)
	}
	
	return newCiphertext, nil
}

// ValidateEncryption validates that encryption/decryption works correctly
func (s *Service) ValidateEncryption() error {
	testData := "test-encryption-validation-data"
	
	// Encrypt
	encrypted, err := s.Encrypt(testData)
	if err != nil {
		return fmt.Errorf("encryption validation failed: %w", err)
	}
	
	// Decrypt
	decrypted, err := s.Decrypt(encrypted)
	if err != nil {
		return fmt.Errorf("decryption validation failed: %w", err)
	}
	
	// Compare
	if decrypted != testData {
		return errors.New("encryption validation failed: decrypted data doesn't match original")
	}
	
	return nil
}

// HashSensitiveData creates a hash for sensitive data (for logging/auditing without exposing the actual data)
func (s *Service) HashSensitiveData(data string) string {
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("sha256:%x", hash[:8]) // First 8 bytes for brevity
}