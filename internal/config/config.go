package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration
type Config struct {
	// Logging configuration
	LogLevel string

	// Authentication configuration
	EnableAuthentication bool
	BearerToken          string

	// Redis configuration
	RedisURL string

	// GitLab configuration
	GitLabToken   string
	GitLabBaseURL string
	GitLabSkipTLS bool

	// Application configuration
	ComparisonBranch string
	DriftThreshold   int

	// Server configuration
	Port string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		// Logging
		LogLevel: getEnvString("LOG_LEVEL", "info"),

		// Authentication
		EnableAuthentication: getEnvBool("ENABLE_AUTHENTICATION", false),
		BearerToken:          getEnvString("BEARER_TOKEN", ""),

		// Redis
		RedisURL: getEnvString("REDIS_URL", ""),

		// GitLab (maintaining backward compatibility)
		GitLabToken:   getEnvString("GITLAB_API_TOKEN", ""),                        // Keep existing name
		GitLabBaseURL: getEnvString("GITLAB_API_URL", "https://gitlab.com/api/v4"), // Use existing env var name with default
		GitLabSkipTLS: getEnvBool("GITLAB_SKIP_TLS_VERIFY", false),

		// Application (maintaining backward compatibility)
		ComparisonBranch: getEnvString("COMPARISION_BRANCH", "main"), // Keep existing typo for compatibility
		DriftThreshold:   getEnvInt("DEFAULT_DRIFT_THRESHOLD", 1),    // Keep existing name

		// Server
		Port: getEnvString("PORT", "8080"),
	}
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	// Check required fields
	if c.RedisURL == "" {
		return &ConfigError{Field: "REDIS_URL", Message: "Redis URL is required"}
	}

	if c.EnableAuthentication && c.BearerToken == "" {
		return &ConfigError{Field: "BEARER_TOKEN", Message: "Bearer token is required when authentication is enabled"}
	}

	return nil
}

// GetLogLevel returns the slog.Level for the configured log level
func (c *Config) GetLogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "Configuration error for " + e.Field + ": " + e.Message
}

// Helper functions for environment variable parsing

func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true"
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
