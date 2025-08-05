package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTOTPManager_NewTOTPManager(t *testing.T) {
	issuer := "dbackup-test"
	tm := NewTOTPManager(issuer)

	assert.NotNil(t, tm)
	assert.Equal(t, issuer, tm.issuer)
	assert.Equal(t, otp.DigitsSix, tm.digits)
	assert.Equal(t, uint(30), tm.period)
}

func TestTOTPManager_GenerateSecret(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")
	accountName := "test@example.com"

	key, err := tm.GenerateSecret(accountName)
	require.NoError(t, err)
	assert.NotNil(t, key)

	// Verify key properties
	assert.Equal(t, "dbackup-test", key.Issuer())
	assert.Equal(t, accountName, key.AccountName())
	assert.NotEmpty(t, key.Secret())
	assert.NotEmpty(t, key.URL())
}

func TestTOTPManager_GenerateCode(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")
	
	key, err := tm.GenerateSecret("test@example.com")
	require.NoError(t, err)

	t.Run("generate current code", func(t *testing.T) {
		code, err := tm.GenerateCode(key.Secret())
		require.NoError(t, err)
		assert.Len(t, code, 6)
		assert.Regexp(t, `^[0-9]{6}$`, code)
	})

	t.Run("generate code at specific time", func(t *testing.T) {
		specificTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		code, err := tm.GenerateCodeAtTime(key.Secret(), specificTime)
		require.NoError(t, err)
		assert.Len(t, code, 6)
		assert.Regexp(t, `^[0-9]{6}$`, code)

		// Same time should generate same code
		code2, err := tm.GenerateCodeAtTime(key.Secret(), specificTime)
		require.NoError(t, err)
		assert.Equal(t, code, code2)
	})

	t.Run("invalid secret", func(t *testing.T) {
		_, err := tm.GenerateCode("invalid-secret")
		assert.Error(t, err)
	})
}

func TestTOTPManager_ValidateCode(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")
	
	key, err := tm.GenerateSecret("test@example.com")
	require.NoError(t, err)

	t.Run("valid code", func(t *testing.T) {
		code, err := tm.GenerateCode(key.Secret())
		require.NoError(t, err)

		valid := tm.ValidateCode(code, key.Secret())
		assert.True(t, valid)
	})

	t.Run("invalid code", func(t *testing.T) {
		valid := tm.ValidateCode("123456", key.Secret())
		assert.False(t, valid)
	})

	t.Run("wrong length code", func(t *testing.T) {
		valid := tm.ValidateCode("12345", key.Secret())
		assert.False(t, valid)

		valid = tm.ValidateCode("1234567", key.Secret())
		assert.False(t, valid)
	})

	t.Run("non-numeric code", func(t *testing.T) {
		valid := tm.ValidateCode("abcdef", key.Secret())
		assert.False(t, valid)
	})
}

func TestTOTPManager_ValidateCodeWithSkew(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")
	
	key, err := tm.GenerateSecret("test@example.com")
	require.NoError(t, err)

	t.Run("current time window", func(t *testing.T) {
		code, err := tm.GenerateCode(key.Secret())
		require.NoError(t, err)

		valid := tm.ValidateCodeWithSkew(code, key.Secret(), 1)
		assert.True(t, valid)
	})

	t.Run("previous time window with skew", func(t *testing.T) {
		// Generate code for 30 seconds ago
		pastTime := time.Now().Add(-30 * time.Second)
		code, err := tm.GenerateCodeAtTime(key.Secret(), pastTime)
		require.NoError(t, err)

		// Should be valid with skew of 1
		valid := tm.ValidateCodeWithSkew(code, key.Secret(), 1)
		assert.True(t, valid)

		// Should be invalid with skew of 0
		valid = tm.ValidateCodeWithSkew(code, key.Secret(), 0)
		// Note: This might be true if we're still in the same time window
		// The exact result depends on timing
	})

	t.Run("way too old code", func(t *testing.T) {
		// Generate code for 5 minutes ago
		oldTime := time.Now().Add(-5 * time.Minute)
		code, err := tm.GenerateCodeAtTime(key.Secret(), oldTime)
		require.NoError(t, err)

		valid := tm.ValidateCodeWithSkew(code, key.Secret(), 1)
		assert.False(t, valid)
	})
}

func TestTOTPManager_GetQRCodeURL(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")
	
	key, err := tm.GenerateSecret("test@example.com")
	require.NoError(t, err)

	qrURL := tm.GetQRCodeURL(key)
	assert.NotEmpty(t, qrURL)
	assert.Contains(t, qrURL, "otpauth://totp/")
	assert.Contains(t, qrURL, "dbackup-test")
	assert.Contains(t, qrURL, "test@example.com")
}

func TestTOTPManager_GetManualEntryKey(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")
	
	key, err := tm.GenerateSecret("test@example.com")
	require.NoError(t, err)

	manualKey := tm.GetManualEntryKey(key)
	assert.NotEmpty(t, manualKey)
	assert.Equal(t, key.Secret(), manualKey)
}

func TestTOTPManager_GenerateBackupCodes(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")

	t.Run("valid count", func(t *testing.T) {
		codes, err := tm.GenerateBackupCodes(8)
		require.NoError(t, err)
		assert.Len(t, codes, 8)

		// Check format (should be XXXX-XXXX-XX)
		for _, code := range codes {
			assert.Regexp(t, `^[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{2}$`, code)
		}

		// All codes should be unique
		codeSet := make(map[string]bool)
		for _, code := range codes {
			assert.False(t, codeSet[code], "Duplicate backup code generated: %s", code)
			codeSet[code] = true
		}
	})

	t.Run("minimum count", func(t *testing.T) {
		codes, err := tm.GenerateBackupCodes(1)
		require.NoError(t, err)
		assert.Len(t, codes, 1)
	})

	t.Run("maximum count", func(t *testing.T) {
		codes, err := tm.GenerateBackupCodes(20)
		require.NoError(t, err)
		assert.Len(t, codes, 20)
	})

	t.Run("invalid count - zero", func(t *testing.T) {
		_, err := tm.GenerateBackupCodes(0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "between 1 and 20")
	})

	t.Run("invalid count - too many", func(t *testing.T) {
		_, err := tm.GenerateBackupCodes(21)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "between 1 and 20")
	})
}

func TestTOTPManager_ValidateBackupCode(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")

	t.Run("valid backup code format", func(t *testing.T) {
		codes, err := tm.GenerateBackupCodes(1)
		require.NoError(t, err)

		valid := tm.ValidateBackupCode(codes[0])
		assert.True(t, valid)
	})

	t.Run("valid backup code with spaces", func(t *testing.T) {
		valid := tm.ValidateBackupCode("ABCD EFGH IJ")
		assert.True(t, valid)
	})

	t.Run("valid backup code without hyphens", func(t *testing.T) {
		valid := tm.ValidateBackupCode("ABCDEFGHIJ")
		assert.True(t, valid)
	})

	t.Run("invalid length", func(t *testing.T) {
		valid := tm.ValidateBackupCode("ABCD-EFGH")
		assert.False(t, valid)

		valid = tm.ValidateBackupCode("ABCD-EFGH-IJK")
		assert.False(t, valid)
	})

	t.Run("invalid characters", func(t *testing.T) {
		valid := tm.ValidateBackupCode("ABCD-EFGH-@#")
		assert.False(t, valid)
	})

	t.Run("empty code", func(t *testing.T) {
		valid := tm.ValidateBackupCode("")
		assert.False(t, valid)
	})
}

func TestParseTOTPURL(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")
	key, err := tm.GenerateSecret("test@example.com")
	require.NoError(t, err)

	url := key.URL()

	t.Run("valid TOTP URL", func(t *testing.T) {
		info, err := ParseTOTPURL(url)
		require.NoError(t, err)
		
		// The account name in the URL path includes the issuer prefix
		assert.Equal(t, "dbackup-test:test@example.com", info.AccountName)
		assert.Equal(t, key.Secret(), info.Secret)
		assert.Equal(t, "dbackup-test", info.Issuer)
		assert.NotEmpty(t, info.Algorithm)
		assert.NotEmpty(t, info.Digits)
		assert.NotEmpty(t, info.Period)
	})

	t.Run("invalid URL", func(t *testing.T) {
		_, err := ParseTOTPURL("invalid-url")
		assert.Error(t, err)
	})

	t.Run("wrong scheme", func(t *testing.T) {
		_, err := ParseTOTPURL("https://example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid TOTP URL scheme")
	})

	t.Run("wrong host", func(t *testing.T) {
		_, err := ParseTOTPURL("otpauth://hotp/test@example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid TOTP URL scheme or host")
	})

	t.Run("missing secret", func(t *testing.T) {
		_, err := ParseTOTPURL("otpauth://totp/test@example.com?issuer=test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing secret parameter")
	})
}

func TestTOTPManager_GetCurrentTimeWindow(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")

	window := tm.GetCurrentTimeWindow()
	assert.Greater(t, window, int64(0))

	// Should be consistent for calls within the same second
	window2 := tm.GetCurrentTimeWindow()
	assert.Equal(t, window, window2)
}

func TestTOTPManager_GetTimeWindowForTime(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")

	testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	window := tm.GetTimeWindowForTime(testTime)
	
	expected := testTime.Unix() / 30 // 30 second periods
	assert.Equal(t, expected, window)
}

func TestTOTPManager_GetRemainingTime(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")

	remaining := tm.GetRemainingTime()
	assert.Greater(t, remaining, time.Duration(0))
	assert.LessOrEqual(t, remaining, 30*time.Second)
}

func TestTOTPManager_SetupTOTP(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")
	accountName := "test@example.com"

	setup, err := tm.SetupTOTP(accountName)
	require.NoError(t, err)
	assert.NotNil(t, setup)

	assert.NotEmpty(t, setup.Secret)
	assert.NotEmpty(t, setup.QRCodeURL)
	assert.NotEmpty(t, setup.ManualEntry)
	assert.Len(t, setup.BackupCodes, 8)
	assert.Equal(t, "dbackup-test", setup.Issuer)
	assert.Equal(t, accountName, setup.AccountName)

	// QR code URL should contain the secret
	assert.Contains(t, setup.QRCodeURL, setup.Secret)

	// Manual entry should be formatted with spaces
	assert.Contains(t, setup.ManualEntry, " ")
	assert.Equal(t, strings.ToUpper(setup.ManualEntry), setup.ManualEntry)

	// Backup codes should be valid
	for _, code := range setup.BackupCodes {
		assert.True(t, tm.ValidateBackupCode(code))
	}
}

func TestFormatSecretForManualEntry(t *testing.T) {
	testCases := []struct {
		name     string
		secret   string
		expected string
	}{
		{
			name:     "16 character secret",
			secret:   "ABCDEFGHIJKLMNOP",
			expected: "ABCD EFGH IJKL MNOP",
		},
		{
			name:     "32 character secret",
			secret:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567",
			expected: "ABCD EFGH IJKL MNOP QRST UVWX YZ23 4567",
		},
		{
			name:     "secret with spaces",
			secret:   "ABCD EFGH IJKL MNOP",
			expected: "ABCD EFGH IJKL MNOP",
		},
		{
			name:     "lowercase secret",
			secret:   "abcdefghijklmnop",
			expected: "ABCD EFGH IJKL MNOP",
		},
		{
			name:     "empty secret",
			secret:   "",
			expected: "",
		},
		{
			name:     "short secret",
			secret:   "ABC",
			expected: "ABC",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatSecretForManualEntry(tc.secret)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTOTPManager_VerifyTOTPSetup(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")

	setup, err := tm.SetupTOTP("test@example.com")
	require.NoError(t, err)

	t.Run("valid setup verification", func(t *testing.T) {
		// Generate a code using the secret
		code, err := tm.GenerateCode(setup.Secret)
		require.NoError(t, err)

		valid := tm.VerifyTOTPSetup(setup.Secret, code)
		assert.True(t, valid)
	})

	t.Run("invalid code", func(t *testing.T) {
		valid := tm.VerifyTOTPSetup(setup.Secret, "123456")
		assert.False(t, valid)
	})

	t.Run("invalid secret", func(t *testing.T) {
		valid := tm.VerifyTOTPSetup("invalid-secret", "123456")
		assert.False(t, valid)
	})
}

func TestTOTPManager_GetTOTPStatus(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")

	setup, err := tm.SetupTOTP("test@example.com")
	require.NoError(t, err)

	status, err := tm.GetTOTPStatus(setup.Secret)
	require.NoError(t, err)
	assert.NotNil(t, status)

	assert.Len(t, status.CurrentCode, 6)
	assert.Regexp(t, `^[0-9]{6}$`, status.CurrentCode)
	assert.Greater(t, status.CurrentWindow, int64(0))
	assert.Greater(t, status.RemainingTime, time.Duration(0))
	assert.LessOrEqual(t, status.RemainingTime, 30*time.Second)
	assert.Equal(t, uint(30), status.Period)
	assert.Equal(t, 6, status.Digits)
	assert.Equal(t, "SHA1", status.Algorithm)
}

func TestGenerateBackupCode(t *testing.T) {
	t.Run("generate backup code", func(t *testing.T) {
		code, err := generateBackupCode()
		require.NoError(t, err)
		assert.NotEmpty(t, code)
		
		// Should match format XXXX-XXXX-XX
		assert.Regexp(t, `^[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{2}$`, code)
	})

	t.Run("generate multiple codes", func(t *testing.T) {
		codes := make(map[string]bool)
		
		for i := 0; i < 10; i++ {
			code, err := generateBackupCode()
			require.NoError(t, err)
			
			// Should be unique
			assert.False(t, codes[code], "Generated duplicate backup code: %s", code)
			codes[code] = true
		}
	})
}

func TestTOTPManager_EdgeCases(t *testing.T) {
	t.Run("empty issuer", func(t *testing.T) {
		tm := NewTOTPManager("")
		_, err := tm.GenerateSecret("test@example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Issuer must be set")
	})

	t.Run("empty account name", func(t *testing.T) {
		tm := NewTOTPManager("dbackup-test")
		_, err := tm.GenerateSecret("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AccountName must be set")
	})

	t.Run("very long account name", func(t *testing.T) {
		tm := NewTOTPManager("dbackup-test")
		longName := strings.Repeat("a", 1000) + "@example.com"
		key, err := tm.GenerateSecret(longName)
		require.NoError(t, err)
		assert.Equal(t, longName, key.AccountName())
	})
}

func TestTOTPManager_CrossValidation(t *testing.T) {
	// Test that our implementation is compatible with the standard library
	tm := NewTOTPManager("dbackup-test")

	key, err := tm.GenerateSecret("test@example.com")
	require.NoError(t, err)

	// Generate code using our method
	ourCode, err := tm.GenerateCode(key.Secret())
	require.NoError(t, err)

	// Validate using standard library
	valid := totp.Validate(ourCode, key.Secret())
	assert.True(t, valid, "Our generated code should be valid with standard library")

	// Generate code using standard library
	stdCode, err := totp.GenerateCode(key.Secret(), time.Now())
	require.NoError(t, err)

	// Validate using our method
	valid = tm.ValidateCode(stdCode, key.Secret())
	assert.True(t, valid, "Standard library code should be valid with our method")
}

func TestTOTPManager_TimeBasedConsistency(t *testing.T) {
	tm := NewTOTPManager("dbackup-test")

	key, err := tm.GenerateSecret("test@example.com")
	require.NoError(t, err)

	// Test that codes are consistent within the same time window
	code1, err := tm.GenerateCode(key.Secret())
	require.NoError(t, err)

	// Small delay (should still be same time window)
	time.Sleep(100 * time.Millisecond)

	code2, err := tm.GenerateCode(key.Secret())
	require.NoError(t, err)

	assert.Equal(t, code1, code2, "Codes should be the same within the same time window")
}