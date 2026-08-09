// Package config loads and validates gateway configuration from environment
// variables. All other packages receive a *Config — none read env vars directly.
package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the gateway.
// Add new fields here as sub-packages are built.
type Config struct {
	// Server
	Addr string

	// Redis
	RedisAddr string
	PoolSize  int

	// Auth
	JWTSecret []byte

	// Limiter
	LimiterMode string        // "static" | "adaptive"
	BaseLimit   int           // requests allowed per window at risk score 0
	Window      time.Duration // token bucket refill window

	// Proxy
	BackendURL string // mock backend e.g. "http://localhost:8081"

	// Replay prevention
	NonceWindowSec int // timestamp window in seconds e.g. 300
}

// Load reads configuration from environment variables and returns a validated
// Config. It returns an error if any required field is missing or invalid.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:           getEnv("ADDR", ":8080"),
		RedisAddr:      getEnv("REDIS_URL", "localhost:6379"),
		PoolSize:       getEnvInt("REDIS_POOL_SIZE", 10),
		JWTSecret:      []byte(getEnv("JWT_SECRET", "")),
		LimiterMode:    getEnv("LIMITER_MODE", "static"),
		BaseLimit:      getEnvInt("BASE_LIMIT", 100),
		Window:         time.Duration(getEnvInt("WINDOW_MS", 1000)) * time.Millisecond,
		BackendURL:     getEnv("BACKEND_URL", "http://localhost:8081"),
		NonceWindowSec: getEnvInt("NONCE_WINDOW_SEC", 300),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks that all required fields are present and valid.
func (c *Config) validate() error {
	if len(c.JWTSecret) == 0 {
		return errors.New("config: JWT_SECRET must not be empty")
	}
	if c.LimiterMode != "static" && c.LimiterMode != "adaptive" {
		return errors.New("config: LIMITER_MODE must be 'static' or 'adaptive'")
	}
	if c.BaseLimit <= 0 {
		return errors.New("config: BASE_LIMIT must be greater than 0")
	}
	if c.BackendURL == "" {
		return errors.New("config: BACKEND_URL must not be empty")
	}
	return nil
}

// --- helpers ---

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
