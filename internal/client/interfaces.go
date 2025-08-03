package client

import "context"

// Issue represents a GitLab issue
type Issue struct {
	ID        int    `json:"iid"`
	ProjectID int    `json:"project_id"`
	Title     string `json:"title"`
	WebURL    string `json:"web_url"`
	State     string `json:"state"`
}

// IssueTracker defines the interface for GitLab issue management
type IssueTracker interface {
	// CreateIssue creates a new GitLab issue and returns issue details
	CreateIssue(ctx context.Context, projectID int, title, description string) (*Issue, error)

	// CloseIssue removes a GitLab issue
	CloseIssue(ctx context.Context, projectID, issueID int, operation string) error

	// GetIssueStatus checks if an issue exists and is open
	GetIssueStatus(ctx context.Context, projectID, issueID int) (bool, error)
}
