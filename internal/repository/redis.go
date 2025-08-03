package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/redis/go-redis/v9"
)

// RedisRepository implements StorageRepository interface for Redis operations
type RedisRepository struct {
	client *redis.Client
}

// NewRedisRepository creates a new Redis repository instance
func NewRedisRepository(client *redis.Client) *RedisRepository {
	return &RedisRepository{
		client: client,
	}
}

// InitializeEnvironment creates a new environment hash with default values
func (r *RedisRepository) InitializeEnvironment(ctx context.Context, key, tier, projectID, threshold string) (bool, error) {
	slog.Debug("Initializing environment in Redis",
		"key", key,
		"tier", tier,
		"project_id", projectID,
		"threshold", threshold,
	)

	// Check if hash exists
	slog.Debug("Checking if environment already exists", "key", key)
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		slog.Error("Failed to check if environment exists", "key", key)
		return false, fmt.Errorf("error checking hash existence: %w", err)
	}

	// If hash already exists, return false
	if exists > 0 {
		slog.Debug("Environment already exists, skipping initialization", "key", key)
		return false, nil
	}

	// Use provided threshold (service layer should provide default)
	if threshold == "" {
		threshold = os.Getenv("DEFAULT_DRIFT_THRESHOLD")
		if threshold == "" {
			threshold = "1"
		}
		slog.Debug("Using fallback threshold", "threshold", threshold)
	}

	// Initialize hash with default values
	fields := map[string]interface{}{
		"driftThreshold":  threshold,
		"environmentTier": tier,
		"projectID":       projectID,
		"driftIncrement":  "0",
	}

	slog.Debug("Creating environment hash in Redis", "key", key, "fields", fields)
	err = r.client.HMSet(ctx, key, fields).Err()
	if err != nil {
		slog.Error("Failed to initialize environment hash",
			"key", key,
			"tier", tier,
			"project_id", projectID,
		)
		return false, fmt.Errorf("error initializing environment hash: %w", err)
	}

	slog.Info("Environment initialized successfully",
		"key", key,
		"tier", tier,
		"project_id", projectID,
		"threshold", threshold,
	)

	return true, nil
}

// UpdateOperationLog records operation timestamp and type
func (r *RedisRepository) UpdateOperationLog(ctx context.Context, key, timestamp, operation string) error {
	slog.Debug("Updating operation log",
		"key", key,
		"timestamp", timestamp,
		"operation", operation,
	)

	logEntry := fmt.Sprintf(`{"timestamp": "%s", "operation": "%s"}`, timestamp, operation)
	err := r.client.HMSet(ctx, key, map[string]interface{}{
		"log": logEntry,
	}).Err()

	if err != nil {
		slog.Error("Failed to update operation log",
			"key", key,
			"operation", operation,
		)
		return fmt.Errorf("error updating operation log: %w", err)
	}

	slog.Debug("Operation log updated successfully", "key", key, "operation", operation)
	return nil
}

// IncrementDrift increases drift counter and returns new value
func (r *RedisRepository) IncrementDrift(ctx context.Context, key string) (int, error) {
	slog.Debug("Incrementing drift counter", "key", key)

	newValue, err := r.client.HIncrBy(ctx, key, "driftIncrement", 1).Result()
	if err != nil {
		slog.Error("Failed to increment drift counter", "key", key)
		return 0, fmt.Errorf("error incrementing drift: %w", err)
	}

	return int(newValue), nil
}

// ResetDrift sets drift counter to zero
func (r *RedisRepository) ResetDrift(ctx context.Context, key string) error {
	slog.Debug("Resetting drift counter", "key", key)

	err := r.client.HSet(ctx, key, "driftIncrement", "0").Err()
	if err != nil {
		slog.Error("Failed to reset drift counter", "key", key)
		return fmt.Errorf("error resetting drift: %w", err)
	}

	return nil
}

// GetEnvironmentData retrieves all environment data as map
func (r *RedisRepository) GetEnvironmentData(ctx context.Context, key string) (map[string]string, error) {
	slog.Debug("Retrieving environment data", "key", key)

	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		slog.Error("Failed to retrieve environment data", "key", key)
		return nil, fmt.Errorf("error retrieving environment data: %w", err)
	}

	if len(data) == 0 {
		slog.Warn("No environment data found", "key", key)
		return nil, fmt.Errorf("no data found for key: %s", key)
	}

	slog.Debug("Environment data retrieved successfully",
		"key", key,
		"field_count", len(data),
	)

	return data, nil
}

// SetField updates a specific field in the environment hash
func (r *RedisRepository) SetField(ctx context.Context, key, field, value string) error {
	slog.Debug("Setting field in environment hash",
		"key", key,
		"field", field,
		"value", value,
	)

	err := r.client.HSet(ctx, key, field, value).Err()
	if err != nil {
		slog.Error("Failed to set field",
			"key", key,
			"field", field,
		)
		return fmt.Errorf("error setting field %s: %w", field, err)
	}

	slog.Debug("Field set successfully", "key", key, "field", field)
	return nil
}

// GetField retrieves a specific field from the environment hash
func (r *RedisRepository) GetField(ctx context.Context, key, field string) (string, error) {
	slog.Debug("Getting field from environment hash", "key", key, "field", field)

	value, err := r.client.HGet(ctx, key, field).Result()
	if err != nil {
		if err == redis.Nil {
			slog.Debug("Field not found", "key", key, "field", field)
			return "", nil // Field doesn't exist, return empty string
		}
		slog.Error("Failed to get field", "key", key, "field", field)
		return "", fmt.Errorf("error getting field %s: %w", field, err)
	}

	slog.Debug("Field retrieved successfully",
		"key", key,
		"field", field,
		"value", value,
	)

	return value, nil
}

// StorePlanOutput saves Terraform plan output for the environment
func (r *RedisRepository) StorePlanOutput(ctx context.Context, key, planOutput string) error {
	slog.Debug("Storing plan output",
		"key", key,
		"plan_output_length", len(planOutput),
	)

	err := r.client.HSet(ctx, key, "planOutput", planOutput).Err()
	if err != nil {
		slog.Error("Failed to store plan output",
			"key", key,
			"plan_output_length", len(planOutput),
		)
		return fmt.Errorf("error storing plan output: %w", err)
	}

	slog.Debug("Plan output stored successfully", "key", key)
	return nil
}
