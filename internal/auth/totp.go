package auth

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPManager handles TOTP (Time-based One-Time Password) operations
type TOTPManager struct {
	issuer string
	digits otp.Digits
	period uint
}

// NewTOTPManager creates a new TOTP manager
func NewTOTPManager(issuer string) *TOTPManager {
	return &TOTPManager{
		issuer: issuer,
		digits: otp.DigitsSix,
		period: 30, // 30 seconds
	}
}

// GenerateSecret generates a new TOTP secret for a user
func (tm *TOTPManager) GenerateSecret(accountName string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      tm.issuer,
		AccountName: accountName,
		Period:      tm.period,
		Digits:      tm.digits,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	return key, nil
}

// GenerateCode generates a TOTP code for the current time
func (tm *TOTPManager) GenerateCode(secret string) (string, error) {
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP code: %w", err)
	}
	return code, nil
}

// GenerateCodeAtTime generates a TOTP code for a specific time
func (tm *TOTPManager) GenerateCodeAtTime(secret string, t time.Time) (string, error) {
	code, err := totp.GenerateCode(secret, t)
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP code: %w", err)
	}
	return code, nil
}

// ValidateCode validates a TOTP code
func (tm *TOTPManager) ValidateCode(code, secret string) bool {
	return totp.Validate(code, secret)
}

// ValidateCodeWithSkew validates a TOTP code with time skew tolerance
func (tm *TOTPManager) ValidateCodeWithSkew(code, secret string, skew uint) bool {
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    tm.period,
		Skew:      skew,
		Digits:    tm.digits,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && valid
}

// GetQRCodeURL generates a QR code URL for easy setup in authenticator apps
func (tm *TOTPManager) GetQRCodeURL(key *otp.Key) string {
	return key.URL()
}

// GetManualEntryKey returns the secret key for manual entry
func (tm *TOTPManager) GetManualEntryKey(key *otp.Key) string {
	return key.Secret()
}

// GenerateBackupCodes generates backup codes for 2FA recovery
func (tm *TOTPManager) GenerateBackupCodes(count int) ([]string, error) {
	if count <= 0 || count > 20 {
		return nil, fmt.Errorf("backup code count must be between 1 and 20")
	}

	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := generateBackupCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %w", err)
		}
		codes[i] = code
	}

	return codes, nil
}

// generateBackupCode generates a single backup code
func generateBackupCode() (string, error) {
	// Generate 8 bytes of random data
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Convert to base32 and format as XXXX-XXXX
	encoded := base32.StdEncoding.EncodeToString(bytes)
	encoded = strings.ToUpper(encoded)
	
	// Remove padding characters
	encoded = strings.TrimRight(encoded, "=")
	
	// Format as XXXX-XXXX-XX (taking first 10 characters)
	if len(encoded) >= 10 {
		return fmt.Sprintf("%s-%s-%s", encoded[:4], encoded[4:8], encoded[8:10]), nil
	}
	
	// Fallback format
	return encoded, nil
}

// ValidateBackupCode validates a backup code format
func (tm *TOTPManager) ValidateBackupCode(code string) bool {
	// Remove any spaces or hyphens for validation
	cleaned := strings.ReplaceAll(strings.ReplaceAll(code, "-", ""), " ", "")
	
	// Should be 10 characters of base32
	if len(cleaned) != 10 {
		return false
	}

	// Check if it's valid base32
	_, err := base32.StdEncoding.DecodeString(cleaned + "======") // Add padding
	return err == nil
}

// ParseTOTPURL parses a TOTP URL and extracts the components
func ParseTOTPURL(otpURL string) (*TOTPInfo, error) {
	u, err := url.Parse(otpURL)
	if err != nil {
		return nil, fmt.Errorf("invalid TOTP URL: %w", err)
	}

	if u.Scheme != "otpauth" || u.Host != "totp" {
		return nil, fmt.Errorf("invalid TOTP URL scheme or host")
	}

	query := u.Query()
	
	info := &TOTPInfo{
		AccountName: strings.TrimPrefix(u.Path, "/"),
		Secret:      query.Get("secret"),
		Issuer:      query.Get("issuer"),
		Algorithm:   query.Get("algorithm"),
		Digits:      query.Get("digits"),
		Period:      query.Get("period"),
	}

	if info.Secret == "" {
		return nil, fmt.Errorf("TOTP URL missing secret parameter")
	}

	return info, nil
}

// TOTPInfo represents parsed TOTP information
type TOTPInfo struct {
	AccountName string `json:"account_name"`
	Secret      string `json:"secret"`
	Issuer      string `json:"issuer"`
	Algorithm   string `json:"algorithm"`
	Digits      string `json:"digits"`
	Period      string `json:"period"`
}

// GetCurrentTimeWindow returns the current time window for TOTP
func (tm *TOTPManager) GetCurrentTimeWindow() int64 {
	return time.Now().Unix() / int64(tm.period)
}

// GetTimeWindowForTime returns the time window for a specific time
func (tm *TOTPManager) GetTimeWindowForTime(t time.Time) int64 {
	return t.Unix() / int64(tm.period)
}

// GetRemainingTime returns the remaining time in the current window
func (tm *TOTPManager) GetRemainingTime() time.Duration {
	now := time.Now()
	currentWindow := tm.GetTimeWindowForTime(now)
	nextWindow := time.Unix((currentWindow+1)*int64(tm.period), 0)
	return nextWindow.Sub(now)
}

// TOTPSetup represents the setup information for TOTP
type TOTPSetup struct {
	Secret      string   `json:"secret"`
	QRCodeURL   string   `json:"qr_code_url"`
	ManualEntry string   `json:"manual_entry"`
	BackupCodes []string `json:"backup_codes"`
	Issuer      string   `json:"issuer"`
	AccountName string   `json:"account_name"`
}

// SetupTOTP creates a complete TOTP setup for a user
func (tm *TOTPManager) SetupTOTP(accountName string) (*TOTPSetup, error) {
	// Generate secret
	key, err := tm.GenerateSecret(accountName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	// Generate backup codes
	backupCodes, err := tm.GenerateBackupCodes(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	setup := &TOTPSetup{
		Secret:      key.Secret(),
		QRCodeURL:   key.URL(),
		ManualEntry: formatSecretForManualEntry(key.Secret()),
		BackupCodes: backupCodes,
		Issuer:      tm.issuer,
		AccountName: accountName,
	}

	return setup, nil
}

// formatSecretForManualEntry formats the secret for easier manual entry
func formatSecretForManualEntry(secret string) string {
	// Remove any existing spaces and convert to uppercase
	cleaned := strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	
	// Add spaces every 4 characters for readability
	var formatted strings.Builder
	for i, char := range cleaned {
		if i > 0 && i%4 == 0 {
			formatted.WriteString(" ")
		}
		formatted.WriteRune(char)
	}
	
	return formatted.String()
}

// VerifyTOTPSetup verifies that the TOTP setup is working correctly
func (tm *TOTPManager) VerifyTOTPSetup(secret, userCode string) bool {
	return tm.ValidateCodeWithSkew(userCode, secret, 1) // Allow 1 time step skew
}

// GetTOTPStatus returns the current status of TOTP for debugging
func (tm *TOTPManager) GetTOTPStatus(secret string) (*TOTPStatus, error) {
	currentCode, err := tm.GenerateCode(secret)
	if err != nil {
		return nil, err
	}

	status := &TOTPStatus{
		CurrentCode:     currentCode,
		CurrentWindow:   tm.GetCurrentTimeWindow(),
		RemainingTime:   tm.GetRemainingTime(),
		Period:          tm.period,
		Digits:          int(tm.digits),
		Algorithm:       "SHA1",
	}

	return status, nil
}

// TOTPStatus represents the current TOTP status
type TOTPStatus struct {
	CurrentCode   string        `json:"current_code"`
	CurrentWindow int64         `json:"current_window"`
	RemainingTime time.Duration `json:"remaining_time"`
	Period        uint          `json:"period"`
	Digits        int           `json:"digits"`
	Algorithm     string        `json:"algorithm"`
}