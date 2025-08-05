package config

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	defer Reset()

	// First call should load configuration
	cfg1, err1 := Get()
	require.NoError(t, err1)
	require.NotNil(t, cfg1)

	// Second call should return the same instance
	cfg2, err2 := Get()
	require.NoError(t, err2)
	require.NotNil(t, cfg2)

	// Should be the same instance
	assert.Same(t, cfg1, cfg2)
}

func TestMustGet(t *testing.T) {
	defer Reset()

	// Should not panic with valid configuration
	assert.NotPanics(t, func() {
		cfg := MustGet()
		assert.NotNil(t, cfg)
	})
}

func TestMustGetPanic(t *testing.T) {
	defer Reset()

	// Set invalid configuration that will cause validation to fail
	os.Setenv("BACKUP_MAX_SIZE", "-1")
	defer os.Unsetenv("BACKUP_MAX_SIZE")

	// Should panic with invalid configuration
	assert.Panics(t, func() {
		MustGet()
	})
}

func TestReset(t *testing.T) {
	defer Reset()

	// Load configuration
	cfg1, err := Get()
	require.NoError(t, err)
	require.NotNil(t, cfg1)

	// Reset singleton
	Reset()

	// Set different environment variable
	os.Setenv("SERVER_PORT", "9999")
	defer os.Unsetenv("SERVER_PORT")

	// Load configuration again
	cfg2, err := Get()
	require.NoError(t, err)
	require.NotNil(t, cfg2)

	// Should be different instances
	assert.NotSame(t, cfg1, cfg2)
	assert.Equal(t, "9999", cfg2.Server.Port)
}

func TestConcurrentAccess(t *testing.T) {
	defer Reset()

	const numGoroutines = 100
	var wg sync.WaitGroup
	configs := make([]*Config, numGoroutines)

	// Launch multiple goroutines that call Get() concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			cfg, err := Get()
			require.NoError(t, err)
			configs[index] = cfg
		}(i)
	}

	wg.Wait()

	// All instances should be the same (singleton behavior)
	baseConfig := configs[0]
	for i := 1; i < numGoroutines; i++ {
		assert.Same(t, baseConfig, configs[i], "Config instance %d should be the same as instance 0", i)
	}
}

func TestSingletonWithError(t *testing.T) {
	defer Reset()

	// Set invalid configuration
	os.Setenv("BACKUP_MAX_SIZE", "-1")
	defer os.Unsetenv("BACKUP_MAX_SIZE")

	// First call should return error
	cfg1, err1 := Get()
	assert.Error(t, err1)
	assert.Nil(t, cfg1)

	// Second call should return the same error (cached)
	cfg2, err2 := Get()
	assert.Error(t, err2)
	assert.Nil(t, cfg2)
	assert.Equal(t, err1.Error(), err2.Error())
}

func TestResetClearsError(t *testing.T) {
	defer Reset()

	// Set invalid configuration
	os.Setenv("BACKUP_MAX_SIZE", "-1")
	defer os.Unsetenv("BACKUP_MAX_SIZE")

	// First call should return error
	cfg1, err1 := Get()
	assert.Error(t, err1)
	assert.Nil(t, cfg1)

	// Reset and fix configuration
	Reset()
	os.Unsetenv("BACKUP_MAX_SIZE")

	// Second call should succeed
	cfg2, err2 := Get()
	assert.NoError(t, err2)
	assert.NotNil(t, cfg2)
}