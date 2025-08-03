package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"drift-guardian/internal/client"
	"drift-guardian/internal/config"
	"drift-guardian/internal/repository"
)

// DriftServiceImpl implements the DriftService interface
type DriftServiceImpl struct {
	storage      repository.StorageRepository
	issueTracker client.IssueTracker
	threshold    ThresholdManager
	config       *config.Config
}

// NewDriftService creates a new drift service instance
func NewDriftService(
	storage repository.StorageRepository,
	issueTracker client.IssueTracker,
	threshold ThresholdManager,
	cfg *config.Config,
) *DriftServiceImpl {
	return &DriftServiceImpl{
		storage:      storage,
		issueTracker: issueTracker,
		threshold:    threshold,
		config:       cfg,
	}
}

// ValidatePayload ensures payload contains all required fields
func (d *DriftServiceImpl) ValidatePayload(payload *Payload) error {
	if payload.RepoName == "" {
		return fmt.Errorf("missing repoName in payload")
	}

	if payload.Branch == "" {
		return fmt.Errorf("missing branchName in payload")
	}

	if payload.Environment == "" {
		return fmt.Errorf("missing environment in payload")
	}

	if payload.EnvironmentTier == "" {
		return fmt.Errorf("missing environmentTier in payload")
	}

	if payload.ProjectID == "" {
		return fmt.Errorf("missing projectId in payload")
	}

	if payload.Operation == "" {
		return fmt.Errorf("invalid terraform operation in payload")
	}

	return nil
}

// GenerateKey creates Redis key from repo name and environment
func (d *DriftServiceImpl) GenerateKey(repoName, environment string) string {
	return repoName + ":" + environment
}

// ProcessDriftDetection handles the complete drift detection workflow
func (d *DriftServiceImpl) ProcessDriftDetection(ctx context.Context, payload Payload) (*DriftResult, error) {
	// Log the start of drift processing (NORMAL OPERATION)
	slog.Info("Starting drift detection processing",
		"repo", payload.RepoName,
		"environment", payload.Environment,
		"operation", payload.Operation,
		"exit_code", payload.ExitCode,
		"scheduled", payload.Scheduled,
	)

	// Generate Redis key
	key := d.GenerateKey(payload.RepoName, payload.Environment)

	// Use configured default threshold if payload threshold is empty
	threshold := payload.DriftThreshold
	if threshold == "" {
		threshold = strconv.Itoa(d.config.DriftThreshold)
	}

	// Initialize environment if needed
	_, err := d.storage.InitializeEnvironment(ctx, key, payload.EnvironmentTier, payload.ProjectID, threshold)
	if err != nil {
		slog.Error("Failed to initialize environment", "error", err, "repo", payload.RepoName, "environment", payload.Environment)
		return nil, fmt.Errorf("failed to initialize environment: %w", err)
	}

	// Update operation log
	timestamp := payload.Timestamp
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	err = d.storage.UpdateOperationLog(ctx, key, timestamp, payload.Operation)
	if err != nil {
		slog.Error("Failed to update operation log", "error", err, "repo", payload.RepoName, "environment", payload.Environment)
		return nil, fmt.Errorf("failed to update operation log: %w", err)
	}
	slog.Info("Operation log updated successfully", "key", key, "operation", payload.Operation)

	// Handle drift increment for scheduled operations
	var incrementVal int
	if payload.Scheduled && payload.Operation == "plan" && payload.ExitCode == 2 && payload.Branch == d.config.ComparisonBranch {
		slog.Info("Drift detected: incrementing drift counter",
			"repo", payload.RepoName,
			"environment", payload.Environment,
			"branch", payload.Branch,
			"comparison_branch", d.config.ComparisonBranch,
		)

		incrementVal, err = d.storage.IncrementDrift(ctx, key)
		if err != nil {
			slog.Error("Failed to increment drift counter", "error", err, "repo", payload.RepoName, "environment", payload.Environment)
			return nil, fmt.Errorf("failed to increment drift: %w", err)
		}

		slog.Info("Drift counter incremented",
			"key", key,
			"new_drift_count", incrementVal,
			"repo", payload.RepoName,
			"environment", payload.Environment,
		)

		// Store plan output if provided
		if payload.PlanOutput != "" {
			err = d.storage.StorePlanOutput(ctx, key, payload.PlanOutput)
			if err != nil {
				slog.Error("Failed to store plan output", "error", err, "repo", payload.RepoName, "environment", payload.Environment)
				return nil, fmt.Errorf("failed to store plan output: %w", err)
			}
		}

		// Check threshold and create GitLab issue if needed
		env := EnvironmentInfo{
			RepoName:    payload.RepoName,
			Environment: payload.Environment,
			ProjectID:   payload.ProjectID,
			Key:         key,
		}

		err = d.HandleThresholdBreach(ctx, env, incrementVal)
		if err != nil {
			slog.Error("Failed to handle threshold breach", "error", err, "repo", payload.RepoName, "environment", payload.Environment)
			return nil, fmt.Errorf("failed to handle threshold breach: %w", err)
		}
	}

	// Reset drift increment for successful operations
	if payload.Operation == "apply" || (payload.Operation == "plan" && payload.ExitCode == 0 && payload.Branch == d.config.ComparisonBranch) {
		slog.Info("Resetting drift counter - successful operation detected",
			"operation", payload.Operation,
			"exit_code", payload.ExitCode,
			"branch", payload.Branch,
			"repo", payload.RepoName,
			"environment", payload.Environment,
		)

		env := EnvironmentInfo{
			RepoName:    payload.RepoName,
			Environment: payload.Environment,
			ProjectID:   payload.ProjectID,
			Key:         key,
		}

		err = d.ResetDriftIncrement(ctx, env, payload.Operation)
		if err != nil {
			slog.Error("Failed to reset drift increment", "error", err, "repo", payload.RepoName, "environment", payload.Environment)
			return nil, fmt.Errorf("failed to reset drift increment: %w", err)
		}
	}

	// Get final environment data
	environmentData, err := d.storage.GetEnvironmentData(ctx, key)
	if err != nil {
		slog.Error("Failed to get environment data", "error", err, "repo", payload.RepoName, "environment", payload.Environment)
		return nil, fmt.Errorf("failed to get environment data: %w", err)
	}

	result := &DriftResult{
		EnvironmentTier: environmentData["environmentTier"],
		ProjectID:       environmentData["projectID"],
		DriftIncrement:  environmentData["driftIncrement"],
		IssueID:         environmentData["issueID"],
		IssueURL:        environmentData["issueURL"],
		Log:             map[string]string{"log": environmentData["log"]},
	}

	slog.Info("Drift detection processing completed successfully",
		"repo", payload.RepoName,
		"environment", payload.Environment,
		"operation", payload.Operation,
		"final_drift_count", result.DriftIncrement,
		"issue_id", result.IssueID,
	)

	return result, nil
}

// HandleThresholdBreach manages GitLab issue creation when drift threshold is exceeded
func (d *DriftServiceImpl) HandleThresholdBreach(ctx context.Context, env EnvironmentInfo, driftCount int) error {

	// Check if threshold is exceeded
	exceeded, err := d.threshold.CheckThreshold(ctx, env.Key, driftCount)
	if err != nil {
		slog.Error("Failed to check threshold", "error", err, "repo", env.RepoName, "environment", env.Environment)
		return fmt.Errorf("failed to check threshold: %w", err)
	}

	if !exceeded {
		slog.Info("Threshold not exceeded, no action required",
			"key", env.Key,
			"drift_count", driftCount,
			"repo", env.RepoName,
			"environment", env.Environment,
		)
		return nil
	}

	slog.Warn("Threshold exceeded, proceeding with issue management",
		"key", env.Key,
		"drift_count", driftCount,
		"repo", env.RepoName,
		"environment", env.Environment,
	)

	// Convert project ID to integer
	projectID, err := strconv.Atoi(env.ProjectID)
	if err != nil {
		slog.Error("Invalid project ID format", "error", err, "repo", env.RepoName, "environment", env.Environment)
		return fmt.Errorf("invalid project ID: %w", err)
	}

	// Check for existing issue
	existingIssueIDStr, err := d.storage.GetField(ctx, env.Key, "issueID")
	if err != nil {
		slog.Error("Failed to get existing issue ID", "error", err, "repo", env.RepoName, "environment", env.Environment)
		return fmt.Errorf("failed to get existing issue ID: %w", err)
	}

	var existingIssueID int
	if existingIssueIDStr != "" {
		existingIssueID, err = strconv.Atoi(existingIssueIDStr)
		if err != nil {
			slog.Warn("Invalid existing issue ID format, resetting to 0",
				"existing_issue_id", existingIssueIDStr,
				"key", env.Key,
			)
			existingIssueID = 0 // Reset if conversion fails
		}
	}

	// Get plan output if available
	planOutput, _ := d.storage.GetField(ctx, env.Key, "planOutput")

	// Get threshold value
	thresholdValue, err := d.threshold.GetThreshold(ctx, env.Key)
	if err != nil {
		slog.Error("Failed to get threshold value", "error", err, "repo", env.RepoName, "environment", env.Environment)
		return fmt.Errorf("failed to get threshold value: %w", err)
	}

	// Check if existing issue is still open
	if existingIssueID > 0 {
		slog.Info("Checking status of existing issue",
			"issue_id", existingIssueID,
			"project_id", projectID,
			"repo", env.RepoName,
			"environment", env.Environment,
		)

		isOpen, err := d.issueTracker.GetIssueStatus(ctx, projectID, existingIssueID)
		if err != nil {
			slog.Error("Failed to check existing issue status", "error", err, "repo", env.RepoName, "environment", env.Environment)
			return fmt.Errorf("failed to check existing issue status: %w", err)
		}

		if isOpen {
			slog.Info("Updating existing open issue",
				"issue_id", existingIssueID,
				"drift_count", driftCount,
				"threshold", thresholdValue,
			)

			// Update existing issue instead of creating new one
			if gitlabClient, ok := d.issueTracker.(*client.GitLabClient); ok {
				err = gitlabClient.UpdateIssueDescription(ctx, projectID, existingIssueID, env.RepoName, env.Environment, driftCount, thresholdValue, planOutput)
				if err != nil {
					slog.Error("Failed to update existing issue", "error", err, "repo", env.RepoName, "environment", env.Environment)
					return fmt.Errorf("failed to update existing issue: %w", err)
				}
				slog.Info("Existing issue updated successfully", "issue_id", existingIssueID)
			}
			return nil
		} else {
			slog.Info("Existing issue is closed, will create new issue", "issue_id", existingIssueID)
		}
	}

	// Create new issue
	slog.Info("Creating new drift issue",
		"project_id", projectID,
		"repo", env.RepoName,
		"environment", env.Environment,
		"drift_count", driftCount,
		"threshold", thresholdValue,
	)

	if gitlabClient, ok := d.issueTracker.(*client.GitLabClient); ok {
		issue, err := gitlabClient.CreateDriftIssue(ctx, projectID, env.RepoName, env.Environment, driftCount, thresholdValue, planOutput)
		if err != nil {
			slog.Error("Failed to create drift issue", "error", err, "repo", env.RepoName, "environment", env.Environment)
			return fmt.Errorf("failed to create drift issue: %w", err)
		}

		slog.Info("Drift issue created successfully",
			"issue_id", issue.ID,
			"issue_url", issue.WebURL,
			"environment", env.Environment,
		)

		// Store issue details in Redis
		err = d.storage.SetField(ctx, env.Key, "issueID", strconv.Itoa(issue.ID))
		if err != nil {
			slog.Error("Failed to store issue ID", "error", err, "repo", env.RepoName, "environment", env.Environment)
			return fmt.Errorf("failed to store issue ID: %w", err)
		}

		err = d.storage.SetField(ctx, env.Key, "issueURL", issue.WebURL)
		if err != nil {
			slog.Error("Failed to store issue URL", "error", err, "repo", env.RepoName, "environment", env.Environment)
			return fmt.Errorf("failed to store issue URL: %w", err)
		}

	}

	return nil
}

// ResetDriftIncrement resets drift counter and handles issue cleanup
func (d *DriftServiceImpl) ResetDriftIncrement(ctx context.Context, env EnvironmentInfo, operation string) error {
	// Reset drift counter
	err := d.storage.ResetDrift(ctx, env.Key)
	if err != nil {
		slog.Error("Failed to reset drift counter", "error", err, "repo", env.RepoName, "environment", env.Environment)
		return fmt.Errorf("failed to reset drift: %w", err)
	}
	slog.Info("Drift counter reset successfully", "key", env.Key)

	// Check for existing open issue that needs to be closed
	slog.Debug("Checking for existing issue to close", "key", env.Key)
	issueIDStr, err := d.storage.GetField(ctx, env.Key, "issueID")
	if err != nil || issueIDStr == "" {
		if err != nil {
			slog.Warn("Error getting issue ID, skipping issue cleanup", "error", err, "repo", env.RepoName, "environment", env.Environment)
		} else {
			slog.Debug("No existing issue found to close", "key", env.Key)
		}
		return nil // No issue to close
	}

	slog.Debug("Found existing issue to check", "issue_id", issueIDStr, "key", env.Key)

	issueID, err := strconv.Atoi(issueIDStr)
	if err != nil || issueID <= 0 {
		slog.Warn("Invalid issue ID format, skipping issue cleanup",
			"issue_id_str", issueIDStr,
			"key", env.Key,
		)
		return nil // Invalid issue ID
	}

	projectID, err := strconv.Atoi(env.ProjectID)
	if err != nil {
		slog.Error("Invalid project ID format during issue cleanup", "error", err, "repo", env.RepoName, "environment", env.Environment)
		return fmt.Errorf("invalid project ID: %w", err)
	}

	// Check if issue is still open

	isOpen, err := d.issueTracker.GetIssueStatus(ctx, projectID, issueID)
	if err != nil {
		slog.Error("Failed to check issue status", "error", err, "repo", env.RepoName, "environment", env.Environment)
		return fmt.Errorf("failed to check issue status: %w", err)
	}

	if isOpen {
		slog.Info("Deleting open issue due to drift reset",
			"issue_id", issueID,
			"project_id", projectID,
			"repo", env.RepoName,
			"environment", env.Environment,
		)

		// Close the issue
		err = d.issueTracker.CloseIssue(ctx, projectID, issueID, operation)
		if err != nil {
			slog.Error("Failed to delete issue", "error", err, "repo", env.RepoName, "environment", env.Environment)
			return fmt.Errorf("failed to delete issue: %w", err)
		}

		slog.Info("Issue deleted successfully", "issue_id", issueID)

		// Clear issue details from Redis
		err = d.storage.SetField(ctx, env.Key, "issueID", "")
		if err != nil {
			slog.Error("Failed to clear issue ID from Redis", "error", err, "repo", env.RepoName, "environment", env.Environment)
			return fmt.Errorf("failed to clear issue ID: %w", err)
		}

		err = d.storage.SetField(ctx, env.Key, "issueURL", "")
		if err != nil {
			slog.Error("Failed to clear issue URL from Redis", "error", err, "repo", env.RepoName, "environment", env.Environment)
			return fmt.Errorf("failed to clear issue URL: %w", err)
		}

	}

	return nil
}
