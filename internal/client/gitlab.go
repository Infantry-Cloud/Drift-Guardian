package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"drift-guardian/internal/config"
)

// GitLabClient implements IssueTracker interface for GitLab operations
type GitLabClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewGitLabClient creates a new GitLab client instance
func NewGitLabClient(cfg *config.Config) *GitLabClient {
	slog.Debug("Initializing GitLab client",
		"base_url", cfg.GitLabBaseURL,
		"skip_tls", cfg.GitLabSkipTLS,
		"token_configured", cfg.GitLabToken != "",
	)

	// Configure HTTP client with TLS settings
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Check if TLS verification should be skipped
	if cfg.GitLabSkipTLS {
		slog.Warn("TLS verification disabled for GitLab client")
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	slog.Info("GitLab client initialized successfully", "base_url", cfg.GitLabBaseURL)

	return &GitLabClient{
		httpClient: httpClient,
		baseURL:    cfg.GitLabBaseURL,
		token:      cfg.GitLabToken,
	}
}

// issueRequest represents the request body for creating/updating a GitLab issue
type issueRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description"`
	Labels      []string `json:"labels,omitempty"`
}

// issueResponse represents the response from GitLab API
type issueResponse struct {
	ID        int    `json:"iid"`
	ProjectID int    `json:"project_id"`
	Title     string `json:"title"`
	WebURL    string `json:"web_url"`
	State     string `json:"state"`
}

// CreateIssue creates a new GitLab issue and returns issue details
func (g *GitLabClient) CreateIssue(ctx context.Context, projectID int, title, description string) (*Issue, error) {
	slog.Debug("Creating GitLab issue",
		"project_id", projectID,
		"title", title,
		"description_length", len(description),
	)

	if g.token == "" {
		slog.Error("GitLab API token not configured")
		return nil, fmt.Errorf("GITLAB_API_TOKEN environment variable not set")
	}

	// Prepare request body
	issueReq := issueRequest{
		Title:       title,
		Description: description,
		Labels:      []string{"drift-alert", "automation"},
	}

	slog.Debug("Marshaling issue request", "project_id", projectID, "labels", issueReq.Labels)
	requestBody, err := json.Marshal(issueReq)
	if err != nil {
		slog.Error("Failed to marshal issue request", "error", err, "project_id", projectID)
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/projects/%d/issues", g.baseURL, projectID)
	slog.Debug("Creating HTTP request", "url", url, "method", "POST")
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		slog.Error("Failed to create HTTP request", "error", err, "url", url)
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", g.token)

	// Send request
	slog.Debug("Sending HTTP request to GitLab API", "url", url)
	resp, err := g.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to send HTTP request", "error", err, "url", url)
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("Received response from GitLab API",
		"status_code", resp.StatusCode,
		"url", url,
	)

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("GitLab API returned error status",
			"status_code", resp.StatusCode,
			"url", url,
			"project_id", projectID,
		)
		return nil, fmt.Errorf("received non-success status code: %d", resp.StatusCode)
	}

	// Parse response
	var issueResp issueResponse
	if err := json.NewDecoder(resp.Body).Decode(&issueResp); err != nil {
		slog.Error("Failed to decode GitLab API response", "error", err, "url", url, "project_id", projectID)
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	result := &Issue{
		ID:        issueResp.ID,
		ProjectID: issueResp.ProjectID,
		Title:     issueResp.Title,
		WebURL:    issueResp.WebURL,
		State:     issueResp.State,
	}

	return result, nil
}

// CloseIssue closes a GitLab issue instead of deleting it
func (g *GitLabClient) CloseIssue(ctx context.Context, projectID, issueID int, operation string) error {
	slog.Info("Closing GitLab issue",
		"project_id", projectID,
		"issue_id", issueID,
	)

	if g.token == "" {
		slog.Error("GitLab API token not configured")
		return fmt.Errorf("GITLAB_API_TOKEN environment variable not set")
	}

	// First, add a comment to the issue
	commentURL := fmt.Sprintf("%s/projects/%d/issues/%d/notes", g.baseURL, projectID, issueID)
	commentRequest := map[string]string{
		"body": fmt.Sprintf("**Drift Resolved** - Infrastructure drift has been resolved through successful Terraform `%s` operation. Issue automatically closed by Drift Guardian.", operation),
	}

	commentBody, err := json.Marshal(commentRequest)
	if err != nil {
		slog.Error("Failed to marshal comment request", "error", err, "issue_id", issueID)
		return fmt.Errorf("error marshaling comment request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", commentURL, bytes.NewBuffer(commentBody))
	if err != nil {
		slog.Error("Failed to create comment request", "error", err, "url", commentURL)
		return fmt.Errorf("error creating comment request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", g.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to add comment", "error", err, "url", commentURL)
		// Continue with closing even if comment fails
	} else {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			slog.Debug("Comment added successfully", "issue_id", issueID)
		}
	}

	// Now close the issue
	url := fmt.Sprintf("%s/projects/%d/issues/%d", g.baseURL, projectID, issueID)
	slog.Debug("Creating PUT request to close issue", "url", url)

	updateRequest := map[string]string{
		"state_event": "close",
	}

	requestBody, err := json.Marshal(updateRequest)
	if err != nil {
		slog.Error("Failed to marshal close request", "error", err, "issue_id", issueID)
		return fmt.Errorf("error marshaling close request: %w", err)
	}

	// Create HTTP request for closing the issue
	req, err = http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(requestBody))
	if err != nil {
		slog.Error("Failed to create PUT request", "error", err, "url", url)
		return fmt.Errorf("error creating close request: %w", err)
	}

	// Set headers
	req.Header.Set("PRIVATE-TOKEN", g.token)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	slog.Debug("Sending PUT request to close issue", "url", url)
	resp, err = g.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to send PUT request", "error", err, "url", url)
		return fmt.Errorf("error sending close request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("Received close response", "status_code", resp.StatusCode, "url", url)

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("GitLab API close failed",
			"status_code", resp.StatusCode,
			"project_id", projectID,
			"issue_id", issueID,
			"url", url,
		)
		return fmt.Errorf("received non-success status code for close: %d", resp.StatusCode)
	}

	slog.Info("GitLab issue closed successfully",
		"project_id", projectID,
		"issue_id", issueID,
	)

	return nil
}

// GetIssueStatus checks if an issue exists and is open
func (g *GitLabClient) GetIssueStatus(ctx context.Context, projectID, issueID int) (bool, error) {
	slog.Debug("Checking GitLab issue status",
		"project_id", projectID,
		"issue_id", issueID,
	)

	if g.token == "" {
		slog.Error("GitLab API token not configured")
		return false, fmt.Errorf("GITLAB_API_TOKEN environment variable not set")
	}

	// Create HTTP request to get issue status
	url := fmt.Sprintf("%s/projects/%d/issues/%d", g.baseURL, projectID, issueID)
	slog.Debug("Creating GET request for issue status", "url", url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		slog.Error("Failed to create GET request", "error", err, "url", url)
		return false, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("PRIVATE-TOKEN", g.token)

	// Send request
	slog.Debug("Sending GET request to GitLab API", "url", url)
	resp, err := g.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to send GET request", "error", err, "url", url)
		return false, fmt.Errorf("error sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("Received GET response", "status_code", resp.StatusCode, "url", url)

	// Check response status
	if resp.StatusCode == 404 {
		slog.Debug("Issue not found",
			"project_id", projectID,
			"issue_id", issueID,
		)
		// Issue not found
		return false, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("GitLab API status check failed",
			"status_code", resp.StatusCode,
			"project_id", projectID,
			"issue_id", issueID,
			"url", url,
		)
		return false, fmt.Errorf("received non-success status code: %d", resp.StatusCode)
	}

	// Parse response
	var issueResp issueResponse
	if err := json.NewDecoder(resp.Body).Decode(&issueResp); err != nil {
		slog.Error("Failed to decode GitLab status response",
			"project_id", projectID,
			"issue_id", issueID,
			"url", url,
		)
		return false, fmt.Errorf("error decoding response: %w", err)
	}

	// Check if issue is open
	isOpen := issueResp.State == "opened"
	slog.Debug("Issue status retrieved",
		"project_id", projectID,
		"issue_id", issueID,
		"state", issueResp.State,
		"is_open", isOpen,
	)

	return isOpen, nil
}

// CreateDriftIssue creates a drift-specific issue with formatted content
func (g *GitLabClient) CreateDriftIssue(ctx context.Context, projectID int, repoName, environment string, driftIncrement, threshold int, planOutput string) (*Issue, error) {
	title := fmt.Sprintf("Drift: %s", environment)

	// Base description
	description := fmt.Sprintf(
		"# Drift report for `%s` environment\n\n"+
			"Environment **%s** has a drift increment of **%d**, "+
			"which meets or exceeds the configured threshold of **%d**.\n\n"+
			"Please investigate and address this drift as soon as possible.\n\n",
		environment, environment, driftIncrement, threshold)

	// Add plan output if available
	if planOutput != "" {
		description += fmt.Sprintf("## Terraform Plan Output\n\n```\n%s\n```\n\n", planOutput)
	}

	// Add timestamp
	description += fmt.Sprintf("*This issue was automatically created by Drift Guardian on %s*",
		time.Now().Format(time.RFC1123))

	slog.Debug("Calling CreateIssue with drift-specific content",
		"title", title,
		"description_length", len(description),
	)

	return g.CreateIssue(ctx, projectID, title, description)
}

// UpdateIssueDescription updates the description of an existing GitLab issue
func (g *GitLabClient) UpdateIssueDescription(ctx context.Context, projectID, issueID int, repoName, environment string, driftIncrement, threshold int, planOutput string) error {
	slog.Info("Updating GitLab issue description",
		"project_id", projectID,
		"issue_id", issueID,
		"repo", repoName,
		"environment", environment,
		"drift_count", driftIncrement,
		"threshold", threshold,
		"has_plan_output", planOutput != "",
	)

	if g.token == "" {
		slog.Error("GitLab API token not configured")
		return fmt.Errorf("GITLAB_API_TOKEN environment variable not set")
	}

	// Create updated description
	description := fmt.Sprintf(
		"# Drift report for `%s` environment\n\n"+
			"Environment **%s** has a drift increment of **%d**, "+
			"which meets or exceeds the configured threshold of **%d**.\n\n"+
			"Please investigate and address this drift as soon as possible.\n\n",
		environment, environment, driftIncrement, threshold)

	// Add plan output if available
	if planOutput != "" {
		description += fmt.Sprintf("## Terraform Plan Output\n\n```\n%s\n```\n\n", planOutput)
	}

	// Add timestamp
	description += fmt.Sprintf("*This issue was automatically updated by Drift Guardian on %s*",
		time.Now().Format(time.RFC1123))

	// Prepare request body
	updateRequest := issueRequest{
		Description: description,
	}

	slog.Debug("Marshaling update request", "issue_id", issueID, "description_length", len(description))
	requestBody, err := json.Marshal(updateRequest)
	if err != nil {
		slog.Error("Failed to marshal update request", "issue_id", issueID)
		return fmt.Errorf("error marshaling request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/projects/%d/issues/%d", g.baseURL, projectID, issueID)
	slog.Debug("Creating PUT request for issue update", "url", url)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(requestBody))
	if err != nil {
		slog.Error("Failed to create PUT request", "url", url)
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", g.token)

	// Send request
	slog.Debug("Sending PUT request to GitLab API", "url", url)
	resp, err := g.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to send PUT request", "url", url)
		return fmt.Errorf("error sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("Received PUT response", "status_code", resp.StatusCode, "url", url)

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("GitLab API update failed",
			"status_code", resp.StatusCode,
			"project_id", projectID,
			"issue_id", issueID,
			"url", url,
		)
		return fmt.Errorf("received non-success status code: %d", resp.StatusCode)
	}

	slog.Info("GitLab issue description updated successfully",
		"project_id", projectID,
		"issue_id", issueID,
		"environment", environment,
	)

	return nil
}
