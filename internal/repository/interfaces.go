package repository

import "context"

// StorageRepository defines the interface for environment data persistence
type StorageRepository interface {
	// InitializeEnvironment creates a new environment hash with default values
	InitializeEnvironment(ctx context.Context, key, tier, projectID, threshold string) (bool, error)

	// UpdateOperationLog records operation timestamp and type
	UpdateOperationLog(ctx context.Context, key, timestamp, operation string) error

	// IncrementDrift increases drift counter and returns new value
	IncrementDrift(ctx context.Context, key string) (int, error)

	// ResetDrift sets drift counter to zero
	ResetDrift(ctx context.Context, key string) error

	// GetEnvironmentData retrieves all environment data as map
	GetEnvironmentData(ctx context.Context, key string) (map[string]string, error)

	// SetField updates a specific field in the environment hash
	SetField(ctx context.Context, key, field, value string) error

	// GetField retrieves a specific field from the environment hash
	GetField(ctx context.Context, key, field string) (string, error)

	// StorePlanOutput saves Terraform plan output for the environment
	StorePlanOutput(ctx context.Context, key, planOutput string) error
}
