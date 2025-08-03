package handler

import (
	"context"
	"net/http"
)

// EnvironmentHandler defines the interface for HTTP request handling
type EnvironmentHandler interface {
	// HandleEnvironments processes HTTP requests to the /environments endpoint
	HandleEnvironments(w http.ResponseWriter, r *http.Request, ctx context.Context)
}

// ResponseWriter wraps HTTP response writing functionality
type ResponseWriter interface {
	// WriteSuccess writes a successful response with headers and body
	WriteSuccess(w http.ResponseWriter, payload interface{}, headers map[string]string) error

	// WriteError writes an error response with appropriate status code
	WriteError(w http.ResponseWriter, message string, statusCode int) error
}
