package middleware

import (
	"net/http"
)

// SecurityHeadersMiddleware adds essential security headers to responses
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add essential security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")

			// Process request
			next.ServeHTTP(w, r)
		})
	}
}
