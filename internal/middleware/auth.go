package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"drift-guardian/internal/config"
)

// AuthenticationMiddleware creates middleware for bearer token authentication
func AuthenticationMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if authentication is enabled
			if !cfg.EnableAuthentication {
				slog.Debug("Authentication disabled, allowing request")
				next.ServeHTTP(w, r)
				return
			}

			// Extract bearer token from Authorization header
			token := extractBearerToken(r)
			if token == "" {
				slog.Warn("Request missing bearer token",
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				http.Error(w, "Unauthorized: Bearer token required", http.StatusUnauthorized)
				return
			}

			// Validate token
			if !validateToken(token, cfg.BearerToken) {
				slog.Warn("Invalid bearer token provided",
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"token_prefix", "***",
				)
				http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
				return
			}

			slog.Debug("Authentication successful",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)

			// Authentication successful, proceed to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken extracts the bearer token from the Authorization header
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Check if header starts with "Bearer "
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}

	// Extract token part
	token := strings.TrimSpace(authHeader[len(bearerPrefix):])
	return token
}

// validateToken validates the bearer token against the configured token
func validateToken(token, expectedToken string) bool {
	if expectedToken == "" {
		// If no token is configured, reject all authentication attempts
		return false
	}

	// Simple string comparison for bearer token validation
	return token == expectedToken
}
