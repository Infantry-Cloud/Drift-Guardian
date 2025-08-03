package service

import (
	"context"
	"fmt"
	"strconv"

	"drift-guardian/internal/config"
	"drift-guardian/internal/repository"
)

// ThresholdManagerImpl implements ThresholdManager interface
type ThresholdManagerImpl struct {
	storage repository.StorageRepository
	config  *config.Config
}

// NewThresholdManager creates a new threshold manager instance
func NewThresholdManager(storage repository.StorageRepository, cfg *config.Config) *ThresholdManagerImpl {
	return &ThresholdManagerImpl{
		storage: storage,
		config:  cfg,
	}
}

// CheckThreshold validates if drift count exceeds configured threshold
func (t *ThresholdManagerImpl) CheckThreshold(ctx context.Context, key string, currentDrift int) (bool, error) {
	threshold, err := t.GetThreshold(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get threshold: %w", err)
	}

	return currentDrift >= threshold, nil
}

// GetThreshold retrieves the configured threshold for an environment
func (t *ThresholdManagerImpl) GetThreshold(ctx context.Context, key string) (int, error) {
	thresholdStr, err := t.storage.GetField(ctx, key, "driftThreshold")
	if err != nil {
		return 0, fmt.Errorf("failed to get drift threshold from storage: %w", err)
	}

	if thresholdStr == "" {
		return t.config.DriftThreshold, nil // Use configured default threshold
	}

	threshold, err := strconv.Atoi(thresholdStr)
	if err != nil {
		return 0, fmt.Errorf("invalid threshold value: %w", err)
	}

	return threshold, nil
}
