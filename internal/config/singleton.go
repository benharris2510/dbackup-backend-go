package config

import (
	"sync"
)

var (
	instance *Config
	once     sync.Once
	loadErr  error
)

// Get returns the singleton configuration instance
func Get() (*Config, error) {
	once.Do(func() {
		instance, loadErr = Load()
	})
	return instance, loadErr
}

// MustGet returns the singleton configuration instance or panics if there's an error
func MustGet() *Config {
	cfg, err := Get()
	if err != nil {
		panic(err)
	}
	return cfg
}

// Reset resets the singleton instance (useful for testing)
func Reset() {
	instance = nil
	once = sync.Once{}
	loadErr = nil
}