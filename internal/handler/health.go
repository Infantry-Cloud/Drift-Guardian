package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// HealthResponse represents the JSON response for health endpoints
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
}

// ReadinessResponse represents the JSON response for readiness endpoints
type ReadinessResponse struct {
	Status       string                 `json:"status"`
	Timestamp    time.Time              `json:"timestamp"`
	Service      string                 `json:"service"`
	Dependencies map[string]interface{} `json:"dependencies"`
}

// HealthHandler handles health check endpoints
type HealthHandler struct{}

// NewHealthHandler creates a new health handler instance
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// HandleHealth handles the /health endpoint for Kubernetes liveness probes
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method not allowed\n"))
		return
	}

	// Create health response
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Service:   "drift-guardian",
		Version:   "0.1.2",
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error\n"))
		return
	}
}

// HandleReady handles the /ready endpoint for Kubernetes readiness probes
func (h *HealthHandler) HandleReady(w http.ResponseWriter, r *http.Request, rdb *redis.Client, ctx context.Context) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method not allowed\n"))
		return
	}

	// Check Redis connectivity with timeout
	redisStatus := h.checkRedisConnectivity(rdb, ctx)

	// Determine overall readiness status
	overallStatus := "ready"
	statusCode := http.StatusOK

	if !redisStatus["healthy"].(bool) {
		overallStatus = "not ready"
		statusCode = http.StatusServiceUnavailable
	}

	// Create readiness response
	response := ReadinessResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Service:   "drift-guardian",
		Dependencies: map[string]interface{}{
			"redis": redisStatus,
		},
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error\n"))
		return
	}
}

// checkRedisConnectivity checks Redis connectivity with 5-second timeout
func (h *HealthHandler) checkRedisConnectivity(rdb *redis.Client, ctx context.Context) map[string]interface{} {
	// Create context with 5-second timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Attempt Redis PING
	start := time.Now()
	err := rdb.Ping(timeoutCtx).Err()
	duration := time.Since(start)

	if err != nil {
		return map[string]interface{}{
			"healthy":          false,
			"error":            err.Error(),
			"response_time_ms": duration.Milliseconds(),
		}
	}

	return map[string]interface{}{
		"healthy":          true,
		"status":           "connected",
		"response_time_ms": duration.Milliseconds(),
	}
}
