package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordHasher_NewPasswordHasher(t *testing.T) {
	ph := NewPasswordHasher()
	
	assert.NotNil(t, ph)
	assert.Equal(t, uint32(64*1024), ph.memory)      // 64 MB
	assert.Equal(t, uint32(3), ph.iterations)
	assert.Equal(t, uint8(2), ph.parallelism)
	assert.Equal(t, uint32(16), ph.saltLength)
	assert.Equal(t, uint32(32), ph.keyLength)
}

func TestPasswordHasher_NewCustomPasswordHasher(t *testing.T) {
	memory := uint32(32 * 1024)
	iterations := uint32(4)
	parallelism := uint8(1)
	saltLength := uint32(8)
	keyLength := uint32(16)

	ph := NewCustomPasswordHasher(memory, iterations, parallelism, saltLength, keyLength)
	
	assert.Equal(t, memory, ph.memory)
	assert.Equal(t, iterations, ph.iterations)
	assert.Equal(t, parallelism, ph.parallelism)
	assert.Equal(t, saltLength, ph.saltLength)
	assert.Equal(t, keyLength, ph.keyLength)
}

func TestPasswordHasher_HashPassword(t *testing.T) {
	ph := NewPasswordHasher()
	password := "TestPassword123!"

	hash, err := ph.HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify hash format: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
	parts := strings.Split(hash, "$")
	assert.Len(t, parts, 6)
	assert.Equal(t, "", parts[0]) // Empty first element due to leading $
	assert.Equal(t, "argon2id", parts[1])
	assert.Equal(t, "v=19", parts[2])
	assert.Equal(t, "m=65536,t=3,p=2", parts[3])
	assert.NotEmpty(t, parts[4]) // salt
	assert.NotEmpty(t, parts[5]) // hash
}

func TestPasswordHasher_VerifyPassword(t *testing.T) {
	ph := NewPasswordHasher()
	password := "TestPassword123!"

	t.Run("correct password", func(t *testing.T) {
		hash, err := ph.HashPassword(password)
		require.NoError(t, err)

		valid, err := ph.VerifyPassword(password, hash)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("incorrect password", func(t *testing.T) {
		hash, err := ph.HashPassword(password)
		require.NoError(t, err)

		valid, err := ph.VerifyPassword("WrongPassword", hash)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("empty password", func(t *testing.T) {
		hash, err := ph.HashPassword("")
		require.NoError(t, err)

		valid, err := ph.VerifyPassword("", hash)
		require.NoError(t, err)
		assert.True(t, valid)

		valid, err = ph.VerifyPassword("something", hash)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("invalid hash format", func(t *testing.T) {
		_, err := ph.VerifyPassword(password, "invalid-hash")
		assert.ErrorIs(t, err, ErrInvalidHash)
	})

	t.Run("incompatible version", func(t *testing.T) {
		// Create a hash with wrong version
		invalidHash := "$argon2id$v=18$m=65536,t=3,p=2$c2FsdA$aGFzaA"
		_, err := ph.VerifyPassword(password, invalidHash)
		assert.ErrorIs(t, err, ErrIncompatibleVersion)
	})
}

func TestPasswordHasher_PasswordHashing_Consistency(t *testing.T) {
	ph := NewPasswordHasher()
	password := "ConsistencyTest123!"

	// Hash the same password multiple times
	hash1, err := ph.HashPassword(password)
	require.NoError(t, err)

	hash2, err := ph.HashPassword(password)
	require.NoError(t, err)

	// Hashes should be different (different salts)
	assert.NotEqual(t, hash1, hash2)

	// But both should verify successfully
	valid1, err := ph.VerifyPassword(password, hash1)
	require.NoError(t, err)
	assert.True(t, valid1)

	valid2, err := ph.VerifyPassword(password, hash2)
	require.NoError(t, err)
	assert.True(t, valid2)
}

func TestValidatePasswordStrength(t *testing.T) {
	testCases := []struct {
		name        string
		password    string
		expectValid bool
		expectedErr string
	}{
		{
			name:        "valid strong password",
			password:    "StrongPass123!",
			expectValid: true,
		},
		{
			name:        "too short",
			password:    "Short1!",
			expectValid: false,
			expectedErr: "at least 8 characters",
		},
		{
			name:        "too long",
			password:    strings.Repeat("a", 129) + "A1!",
			expectValid: false,
			expectedErr: "no more than 128 characters",
		},
		{
			name:        "no uppercase",
			password:    "lowercase123!",
			expectValid: false,
			expectedErr: "uppercase letter",
		},
		{
			name:        "no lowercase",
			password:    "UPPERCASE123!",
			expectValid: false,
			expectedErr: "lowercase letter",
		},
		{
			name:        "no number",
			password:    "NoNumbers!",
			expectValid: false,
			expectedErr: "number",
		},
		{
			name:        "no special character",
			password:    "NoSpecial123",
			expectValid: false,
			expectedErr: "special character",
		},
		{
			name:        "all requirements met - minimum length",
			password:    "Pass123!",
			expectValid: true,
		},
		{
			name:        "all requirements met - with spaces",
			password:    "Pass Word 123!",
			expectValid: true,
		},
		{
			name:        "unicode characters",
			password:    "Pässwörd123!",
			expectValid: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tc.password)

			if tc.expectValid {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
			}
		})
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	t.Run("valid length", func(t *testing.T) {
		password, err := GenerateRandomPassword(12)
		require.NoError(t, err)
		assert.Len(t, password, 12)

		// Should meet strength requirements
		err = ValidatePasswordStrength(password)
		assert.NoError(t, err)
	})

	t.Run("minimum length", func(t *testing.T) {
		password, err := GenerateRandomPassword(8)
		require.NoError(t, err)
		assert.Len(t, password, 8)
		
		err = ValidatePasswordStrength(password)
		assert.NoError(t, err)
	})

	t.Run("maximum length", func(t *testing.T) {
		password, err := GenerateRandomPassword(128)
		require.NoError(t, err)
		assert.Len(t, password, 128)
		
		err = ValidatePasswordStrength(password)
		assert.NoError(t, err)
	})

	t.Run("too short", func(t *testing.T) {
		_, err := GenerateRandomPassword(7)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 8 characters")
	})

	t.Run("too long", func(t *testing.T) {
		_, err := GenerateRandomPassword(129)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no more than 128 characters")
	})

	t.Run("uniqueness", func(t *testing.T) {
		password1, err := GenerateRandomPassword(16)
		require.NoError(t, err)

		password2, err := GenerateRandomPassword(16)
		require.NoError(t, err)

		// Should be different
		assert.NotEqual(t, password1, password2)
	})
}

func TestEstimateHashTime(t *testing.T) {
	password := "TestPassword123!"
	memory := uint32(1024)      // Small memory for faster test
	iterations := uint32(1)     // Few iterations for faster test
	parallelism := uint8(1)
	keyLength := uint32(32)

	duration, err := EstimateHashTime(password, memory, iterations, parallelism, keyLength)
	require.NoError(t, err)
	assert.Greater(t, duration, time.Duration(0))
	assert.Less(t, duration, 10*time.Second) // Should be fast with low parameters
}

func TestGetHashParameters(t *testing.T) {
	ph := NewPasswordHasher()
	password := "TestPassword123!"

	hash, err := ph.HashPassword(password)
	require.NoError(t, err)

	memory, iterations, parallelism, err := GetHashParameters(hash)
	require.NoError(t, err)
	
	assert.Equal(t, ph.memory, memory)
	assert.Equal(t, ph.iterations, iterations)
	assert.Equal(t, ph.parallelism, parallelism)
}

func TestCompareHashParameters(t *testing.T) {
	ph1 := NewPasswordHasher()
	ph2 := NewCustomPasswordHasher(32*1024, 2, 1, 16, 32)

	password := "TestPassword123!"

	hash1, err := ph1.HashPassword(password)
	require.NoError(t, err)

	hash2, err := ph2.HashPassword(password)
	require.NoError(t, err)

	// Same hasher should have same parameters
	same, err := CompareHashParameters(hash1, hash1)
	require.NoError(t, err)
	assert.True(t, same)

	// Different hashers should have different parameters
	different, err := CompareHashParameters(hash1, hash2)
	require.NoError(t, err)
	assert.False(t, different)
}

func TestPasswordHasher_NeedsRehash(t *testing.T) {
	ph := NewPasswordHasher()
	password := "TestPassword123!"

	t.Run("same parameters - no rehash needed", func(t *testing.T) {
		hash, err := ph.HashPassword(password)
		require.NoError(t, err)

		needsRehash, err := ph.NeedsRehash(hash)
		require.NoError(t, err)
		assert.False(t, needsRehash)
	})

	t.Run("different parameters - rehash needed", func(t *testing.T) {
		differentPH := NewCustomPasswordHasher(32*1024, 2, 1, 16, 32)
		hash, err := differentPH.HashPassword(password)
		require.NoError(t, err)

		needsRehash, err := ph.NeedsRehash(hash)
		require.NoError(t, err)
		assert.True(t, needsRehash)
	})

	t.Run("invalid hash", func(t *testing.T) {
		_, err := ph.NeedsRehash("invalid-hash")
		assert.Error(t, err)
	})
}

func TestGenerateRandomBytes(t *testing.T) {
	t.Run("generate bytes", func(t *testing.T) {
		bytes, err := generateRandomBytes(16)
		require.NoError(t, err)
		assert.Len(t, bytes, 16)
	})

	t.Run("different calls generate different bytes", func(t *testing.T) {
		bytes1, err := generateRandomBytes(16)
		require.NoError(t, err)

		bytes2, err := generateRandomBytes(16)
		require.NoError(t, err)

		assert.NotEqual(t, bytes1, bytes2)
	})

	t.Run("zero length", func(t *testing.T) {
		bytes, err := generateRandomBytes(0)
		require.NoError(t, err)
		assert.Len(t, bytes, 0)
	})
}

func TestDecodeHash(t *testing.T) {
	ph := NewPasswordHasher()
	password := "TestPassword123!"

	hash, err := ph.HashPassword(password)
	require.NoError(t, err)

	memory, iterations, parallelism, salt, hashBytes, err := decodeHash(hash)
	require.NoError(t, err)

	assert.Equal(t, ph.memory, memory)
	assert.Equal(t, ph.iterations, iterations)
	assert.Equal(t, ph.parallelism, parallelism)
	assert.Len(t, salt, int(ph.saltLength))
	assert.Len(t, hashBytes, int(ph.keyLength))

	t.Run("invalid hash format", func(t *testing.T) {
		_, _, _, _, _, err := decodeHash("invalid")
		assert.ErrorIs(t, err, ErrInvalidHash)
	})

	t.Run("wrong number of parts", func(t *testing.T) {
		_, _, _, _, _, err := decodeHash("$argon2id$v=19$m=65536")
		assert.ErrorIs(t, err, ErrInvalidHash)
	})
}

func TestRandomInt(t *testing.T) {
	t.Run("generate within range", func(t *testing.T) {
		max := 100
		for i := 0; i < 10; i++ {
			val, err := randomInt(max)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, val, 0)
			assert.Less(t, val, max)
		}
	})

	t.Run("max of 1", func(t *testing.T) {
		val, err := randomInt(1)
		require.NoError(t, err)
		assert.Equal(t, 0, val)
	})

	t.Run("max of 2", func(t *testing.T) {
		// Run multiple times to test both possible values
		values := make(map[int]bool)
		for i := 0; i < 20; i++ {
			val, err := randomInt(2)
			require.NoError(t, err)
			values[val] = true
		}
		
		// Should only contain 0 and/or 1
		for val := range values {
			assert.Contains(t, []int{0, 1}, val)
		}
	})

	t.Run("invalid max", func(t *testing.T) {
		_, err := randomInt(0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")

		_, err = randomInt(-1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be positive")
	})
}

func TestPasswordStrength_EdgeCases(t *testing.T) {
	t.Run("exactly 8 characters", func(t *testing.T) {
		err := ValidatePasswordStrength("Pass123!")
		assert.NoError(t, err)
	})

	t.Run("exactly 128 characters", func(t *testing.T) {
		password := strings.Repeat("a", 124) + "A1!@" // 124 + 4 = 128
		err := ValidatePasswordStrength(password)
		assert.NoError(t, err)
	})

	t.Run("special characters variety", func(t *testing.T) {
		specialChars := "!@#$%^&*()_+-=[]{}|;:,.<>?"
		for _, char := range specialChars {
			password := "Pass123" + string(char)
			err := ValidatePasswordStrength(password)
			assert.NoError(t, err, "Failed for special character: %c", char)
		}
	})

	t.Run("non-printable characters", func(t *testing.T) {
		// Test with control characters (should fail)
		password := "Pass123\x01" // Control character
		err := ValidatePasswordStrength(password)
		assert.Error(t, err) // Should fail because \x01 is not a valid special character
	})
}

func TestPasswordHasher_CustomParameters(t *testing.T) {
	// Test with very light parameters for speed
	ph := NewCustomPasswordHasher(1024, 1, 1, 8, 16)
	password := "TestPassword123!"

	hash, err := ph.HashPassword(password)
	require.NoError(t, err)

	valid, err := ph.VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, valid)

	// Verify the parameters are correctly encoded
	memory, iterations, parallelism, err := GetHashParameters(hash)
	require.NoError(t, err)
	assert.Equal(t, uint32(1024), memory)
	assert.Equal(t, uint32(1), iterations)
	assert.Equal(t, uint8(1), parallelism)
}

func TestPasswordHasher_PerformanceConsistency(t *testing.T) {
	ph := NewPasswordHasher()
	password := "TestPassword123!"

	// Hash the same password multiple times and measure consistency
	durations := make([]time.Duration, 5)
	for i := 0; i < 5; i++ {
		start := time.Now()
		_, err := ph.HashPassword(password)
		durations[i] = time.Since(start)
		require.NoError(t, err)
	}

	// All durations should be within reasonable range of each other
	// (This is more of a smoke test than a strict requirement)
	for i := 1; i < len(durations); i++ {
		ratio := float64(durations[i]) / float64(durations[0])
		assert.True(t, ratio > 0.1 && ratio < 10.0, "Duration variance too high: %v vs %v", durations[0], durations[i])
	}
}