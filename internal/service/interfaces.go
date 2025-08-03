package service

import (
	"context"
)

// Payload represents the JSON structure expected in the environment endpoint
type Payload struct {
	RepoName        string `json:"repoName"`
	Branch          string `json:"branchName"`
	Environment     string `json:"environment"`
	EnvironmentTier string `json:"environmentTier"`
	DriftThreshold  string `json:"driftThreshold"`
	ProjectID       string `json:"projectId"`
	Operation       string `json:"operation"`
	ExitCode        int    `json:"exitCode"`
	Scheduled       bool   `json:"scheduled"`
	Timestamp       string `json:"timestamp"`
	PlanOutput      string `json:"planOutput,omitempty"`
}

// DriftResult represents the result of drift detection processing
type DriftResult struct {
	EnvironmentTier string            `json:"environmentTier"`
	ProjectID       string            `json:"projectID"`
	DriftIncrement  string            `json:"driftIncrement"`
	IssueID         string            `json:"issueID"`
	IssueURL        string            `json:"issueURL"`
	Log             map[string]string `json:"log"`
}

// EnvironmentInfo contains environment identification data
type EnvironmentInfo struct {
	RepoName    string
	Environment string
	ProjectID   string
	Key         string
}

// DriftService defines the core business logic interface for drift detection
type DriftService interface {
	// ProcessDriftDetection handles the complete drift detection workflow
	ProcessDriftDetection(ctx context.Context, payload Payload) (*DriftResult, error)

	// ValidatePayload ensures payload contains all required fields
	ValidatePayload(payload *Payload) error

	// GenerateKey creates Redis key from repo name and environment
	GenerateKey(repoName, environment string) string

	// HandleThresholdBreach manages GitLab issue creation when drift threshold is exceeded
	HandleThresholdBreach(ctx context.Context, env EnvironmentInfo, driftCount int) error

	// ResetDriftIncrement resets drift counter and handles issue cleanup
	ResetDriftIncrement(ctx context.Context, env EnvironmentInfo, operation string) error
}

// ThresholdManager handles drift threshold validation and management
type ThresholdManager interface {
	// CheckThreshold validates if drift count exceeds configured threshold
	CheckThreshold(ctx context.Context, key string, currentDrift int) (bool, error)

	// GetThreshold retrieves the configured threshold for an environment
	GetThreshold(ctx context.Context, key string) (int, error)
}
