package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"

	"drift-guardian/internal/client"
	"drift-guardian/internal/config"
	"drift-guardian/internal/handler"
	"drift-guardian/internal/middleware"
	"drift-guardian/internal/repository"
	"drift-guardian/internal/service"
)

// Initialises Redis, sets up HTTP handlers, and starts the HTTP server.
func main() {
	// Load configuration
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		panic("Configuration validation failed: " + err.Error())
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.GetLogLevel(),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String("time", a.Value.Time().Format("2006-01-02 15:04:05"))
			}
			return a
		},
	})))

	slog.Info("Drift Guardian starting", "version", "0.2.1")

	// Log configuration (sanitized)
	slog.Info("Configuration loaded",
		"log_level", cfg.LogLevel,
		"authentication_enabled", cfg.EnableAuthentication,
		"comparison_branch", cfg.ComparisonBranch,
		"drift_threshold", cfg.DriftThreshold,
		"port", cfg.Port,
	)

	// Initialize Redis/Valkey client
	slog.Info("Initializing Redis connection...")
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("Failed to parse Redis URL", "error", err)
		panic(err) // Exit if Redis URL is invalid
	}

	// Create context and Redis client
	ctx := context.Background()
	rdb := redis.NewClient(opt)

	// Initialize service layer dependencies
	slog.Debug("Initializing service layer dependencies")
	redisRepo := repository.NewRedisRepository(rdb)
	gitlabClient := client.NewGitLabClient(cfg)
	thresholdManager := service.NewThresholdManager(redisRepo, cfg)
	driftService := service.NewDriftService(redisRepo, gitlabClient, thresholdManager, cfg)
	slog.Info("Service layer dependencies initialized successfully")

	// Initialize handler layer
	responseWriter := handler.NewResponseWriter()
	environmentHandler := handler.NewEnvironmentHandler(driftService, responseWriter)
	healthHandler := handler.NewHealthHandler()

	// Create HTTP router with middleware
	mux := http.NewServeMux()

	// Health endpoints (no authentication) - Kubernetes probes with security headers
	healthWithSecurity := middleware.SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthHandler.HandleHealth(w, r)
	}))
	readyWithSecurity := middleware.SecurityHeadersMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthHandler.HandleReady(w, r, rdb, ctx)
	}))

	mux.Handle("/health", healthWithSecurity)
	mux.Handle("/ready", readyWithSecurity)

	// Environment endpoint with authentication, logging, and security middleware
	envHandler := middleware.SecurityHeadersMiddleware()(
		middleware.AuthenticationMiddleware(cfg)(
			middleware.LoggingMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				environmentHandler.HandleEnvironments(w, r, ctx)
			})),
		),
	)
	mux.Handle("/environments", envHandler)

	// Start the HTTP server (blocking call)
	serverAddr := ":" + cfg.Port
	slog.Info("Server listening", "address", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		slog.Error("HTTP server error", "error", err)
	}
}
