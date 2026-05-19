package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime settings for both assignment parts.
type Config struct {
	Addr                   string
	MaxRequestsPerMinute   int
	Window                 time.Duration
	MaxURLsPerRequest      int
	MaxURLLength           int
	DefaultLimit           int
	MaxLimit               int
}

// Load reads configuration from environment with sensible defaults.
func Load() Config {
	return Config{
		Addr:                 env("ADDR", ":8080"),
		MaxRequestsPerMinute: envInt("MAX_REQUESTS_PER_MINUTE", 5),
		Window:               time.Duration(envInt("WINDOW_MILLIS", 60000)) * time.Millisecond,
		MaxURLsPerRequest:    envInt("MAX_URLS_PER_REQUEST", 20),
		MaxURLLength:         envInt("MAX_URL_LENGTH", 2048),
		DefaultLimit:         envInt("DEFAULT_LIMIT", 20),
		MaxLimit:             envInt("MAX_LIMIT", 100),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
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
