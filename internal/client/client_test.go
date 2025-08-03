//go:build unit

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"drift-guardian/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestConfig returns a test configuration for GitLab client
func getTestConfig(serverURL, token string) *config.Config {
	return &config.Config{
		GitLabBaseURL: serverURL,
		GitLabToken:   token,
	}
}

// TestGitLabClient_CreateIssue tests GitLab issue creation
func TestGitLabClient_CreateIssue(t *testing.T) {
	// Save original environment variables
	originalToken := os.Getenv("GITLAB_API_TOKEN")
	originalURL := os.Getenv("GITLAB_API_URL")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITLAB_API_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITLAB_API_TOKEN")
		}
		if originalURL != "" {
			os.Setenv("GITLAB_API_URL", originalURL)
		} else {
			os.Unsetenv("GITLAB_API_URL")
		}
	}()

	tests := []struct {
		name             string
		projectID        int
		title            string
		description      string
		gitlabToken      string
		mockResponseCode int
		mockResponseBody string
		expectedError    string
		expectSuccess    bool
	}{
		{
			name:             "successful issue creation",
			projectID:        123,
			title:            "Test Issue",
			description:      "Test description",
			gitlabToken:      "test-token",
			mockResponseCode: 201,
			mockResponseBody: `{"id": 456, "iid": 10, "project_id": 123, "title": "Test Issue", "web_url": "https://gitlab.com/project/issues/10"}`,
			expectSuccess:    true,
		},
		{
			name:             "missing GitLab token",
			projectID:        123,
			title:            "Test Issue",
			description:      "Test description",
			gitlabToken:      "",
			mockResponseCode: 201,
			mockResponseBody: `{}`,
			expectedError:    "GITLAB_API_TOKEN environment variable not set",
			expectSuccess:    false,
		},
		{
			name:             "GitLab API error response",
			projectID:        123,
			title:            "Test Issue",
			description:      "Test description",
			gitlabToken:      "test-token",
			mockResponseCode: 400,
			mockResponseBody: `{"error": "Bad request"}`,
			expectedError:    "received non-success status code: 400",
			expectSuccess:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and headers
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				if tt.gitlabToken != "" {
					assert.Equal(t, tt.gitlabToken, r.Header.Get("PRIVATE-TOKEN"))
				}

				// Verify request body
				var requestBody map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				require.NoError(t, err)

				assert.Equal(t, tt.title, requestBody["title"])
				assert.Equal(t, tt.description, requestBody["description"])

				// Send mock response
				w.WriteHeader(tt.mockResponseCode)
				w.Write([]byte(tt.mockResponseBody))
			}))
			defer mockServer.Close()

			// Set environment variables
			os.Setenv("GITLAB_API_TOKEN", tt.gitlabToken)
			os.Setenv("GITLAB_API_URL", mockServer.URL)

			// Create client and call function
			client := NewGitLabClient(getTestConfig(mockServer.URL, tt.gitlabToken))
			response, err := client.CreateIssue(context.Background(), tt.projectID, tt.title, tt.description)

			if tt.expectSuccess {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, 10, response.ID) // Now stores IID instead of global ID
				assert.Equal(t, tt.projectID, response.ProjectID)
			} else {
				assert.Error(t, err)
				assert.Nil(t, response)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			}
		})
	}
}

// TestGitLabClient_CreateDriftIssue tests GitLab drift-specific issue creation
func TestGitLabClient_CreateDriftIssue(t *testing.T) {
	// Save original environment variables
	originalToken := os.Getenv("GITLAB_API_TOKEN")
	originalURL := os.Getenv("GITLAB_API_URL")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITLAB_API_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITLAB_API_TOKEN")
		}
		if originalURL != "" {
			os.Setenv("GITLAB_API_URL", originalURL)
		} else {
			os.Unsetenv("GITLAB_API_URL")
		}
	}()

	tests := []struct {
		name             string
		projectID        int
		repoName         string
		environment      string
		driftIncrement   int
		threshold        int
		planOutput       string
		gitlabToken      string
		mockResponseCode int
		mockResponseBody string
		expectedError    string
		expectSuccess    bool
	}{
		{
			name:             "successful drift issue creation with plan output",
			projectID:        123,
			repoName:         "test-repo",
			environment:      "production",
			driftIncrement:   5,
			threshold:        3,
			planOutput:       "Plan: 2 to add, 1 to change, 0 to destroy.",
			gitlabToken:      "test-token",
			mockResponseCode: 201,
			mockResponseBody: `{"id": 456, "iid": 10, "project_id": 123, "title": "Drift: production", "web_url": "https://gitlab.com/project/issues/10"}`,
			expectSuccess:    true,
		},
		{
			name:             "successful drift issue creation without plan output",
			projectID:        123,
			repoName:         "test-repo",
			environment:      "staging",
			driftIncrement:   2,
			threshold:        1,
			planOutput:       "",
			gitlabToken:      "test-token",
			mockResponseCode: 201,
			mockResponseBody: `{"id": 457, "iid": 11, "project_id": 123, "title": "Drift: staging", "web_url": "https://gitlab.com/project/issues/11"}`,
			expectSuccess:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and headers
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, tt.gitlabToken, r.Header.Get("PRIVATE-TOKEN"))

				// Verify request body
				var requestBody map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&requestBody)
				require.NoError(t, err)

				expectedTitle := fmt.Sprintf("Drift: %s", tt.environment)
				assert.Equal(t, expectedTitle, requestBody["title"])

				description := requestBody["description"].(string)
				assert.Contains(t, description, tt.environment)
				assert.Contains(t, description, fmt.Sprintf("%d", tt.driftIncrement))
				assert.Contains(t, description, fmt.Sprintf("%d", tt.threshold))

				if tt.planOutput != "" {
					assert.Contains(t, description, tt.planOutput)
				}

				labels := requestBody["labels"].([]interface{})
				assert.Equal(t, 2, len(labels))
				assert.Contains(t, labels, "drift-alert")
				assert.Contains(t, labels, "automation")

				// Send mock response
				w.WriteHeader(tt.mockResponseCode)
				w.Write([]byte(tt.mockResponseBody))
			}))
			defer mockServer.Close()

			// Set environment variables
			os.Setenv("GITLAB_API_TOKEN", tt.gitlabToken)
			os.Setenv("GITLAB_API_URL", mockServer.URL)

			// Create client and call function
			client := NewGitLabClient(getTestConfig(mockServer.URL, tt.gitlabToken))
			response, err := client.CreateDriftIssue(context.Background(), tt.projectID, tt.repoName, tt.environment, tt.driftIncrement, tt.threshold, tt.planOutput)

			if tt.expectSuccess {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				if tt.name == "successful drift issue creation with plan output" {
					assert.Equal(t, 10, response.ID) // Now stores IID instead of global ID
				} else {
					assert.Equal(t, 11, response.ID) // Now stores IID instead of global ID
				}
				assert.Equal(t, tt.projectID, response.ProjectID)
			} else {
				assert.Error(t, err)
				assert.Nil(t, response)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			}
		})
	}
}

// TestGitLabClient_GetIssueStatus tests GitLab issue status checking
func TestGitLabClient_GetIssueStatus(t *testing.T) {
	originalToken := os.Getenv("GITLAB_API_TOKEN")
	originalURL := os.Getenv("GITLAB_API_URL")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITLAB_API_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITLAB_API_TOKEN")
		}
		if originalURL != "" {
			os.Setenv("GITLAB_API_URL", originalURL)
		} else {
			os.Unsetenv("GITLAB_API_URL")
		}
	}()

	tests := []struct {
		name             string
		projectID        int
		issueIID         int
		gitlabToken      string
		mockResponseCode int
		mockResponseBody string
		expectedOpen     bool
		expectedError    string
		expectError      bool
	}{
		{
			name:             "issue is open",
			projectID:        123,
			issueIID:         10,
			gitlabToken:      "test-token",
			mockResponseCode: 200,
			mockResponseBody: `{"state": "opened"}`,
			expectedOpen:     true,
			expectError:      false,
		},
		{
			name:             "issue is closed",
			projectID:        123,
			issueIID:         10,
			gitlabToken:      "test-token",
			mockResponseCode: 200,
			mockResponseBody: `{"state": "closed"}`,
			expectedOpen:     false,
			expectError:      false,
		},
		{
			name:             "issue not found",
			projectID:        123,
			issueIID:         999,
			gitlabToken:      "test-token",
			mockResponseCode: 404,
			mockResponseBody: `{"message": "404 Not found"}`,
			expectedOpen:     false,
			expectError:      false,
		},
		{
			name:             "missing GitLab token",
			projectID:        123,
			issueIID:         10,
			gitlabToken:      "",
			mockResponseCode: 200,
			mockResponseBody: `{}`,
			expectedOpen:     false,
			expectedError:    "GITLAB_API_TOKEN environment variable not set",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock server
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and headers
				assert.Equal(t, "GET", r.Method)

				if tt.gitlabToken != "" {
					assert.Equal(t, tt.gitlabToken, r.Header.Get("PRIVATE-TOKEN"))
				}

				// Verify URL
				expectedPath := fmt.Sprintf("/projects/%d/issues/%d", tt.projectID, tt.issueIID)
				assert.Equal(t, expectedPath, r.URL.Path)

				// Send mock response
				w.WriteHeader(tt.mockResponseCode)
				w.Write([]byte(tt.mockResponseBody))
			}))
			defer mockServer.Close()

			// Set environment variables
			os.Setenv("GITLAB_API_TOKEN", tt.gitlabToken)
			os.Setenv("GITLAB_API_URL", mockServer.URL)

			// Create client and call function
			client := NewGitLabClient(getTestConfig(mockServer.URL, tt.gitlabToken))
			isOpen, err := client.GetIssueStatus(context.Background(), tt.projectID, tt.issueIID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOpen, isOpen)
			}
		})
	}
}

// TestGitLabClient_IssueDescriptionGeneration tests issue description formatting
func TestGitLabClient_IssueDescriptionGeneration(t *testing.T) {
	tests := []struct {
		name           string
		environment    string
		driftIncrement int
		threshold      int
		planOutput     string
		expectedParts  []string
	}{
		{
			name:           "description with plan output",
			environment:    "production",
			driftIncrement: 5,
			threshold:      3,
			planOutput:     "Plan: 2 to add, 1 to change, 0 to destroy.",
			expectedParts: []string{
				"# Drift report for `production` environment",
				"drift increment of **5**",
				"threshold of **3**",
				"## Terraform Plan Output",
				"Plan: 2 to add, 1 to change, 0 to destroy.",
				"automatically created by Drift Guardian",
			},
		},
		{
			name:           "description without plan output",
			environment:    "staging",
			driftIncrement: 2,
			threshold:      1,
			planOutput:     "",
			expectedParts: []string{
				"# Drift report for `staging` environment",
				"drift increment of **2**",
				"threshold of **1**",
				"automatically created by Drift Guardian",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var requestBody map[string]interface{}
				json.NewDecoder(r.Body).Decode(&requestBody)

				description := requestBody["description"].(string)

				// Verify all expected parts are in the description
				for _, expectedPart := range tt.expectedParts {
					assert.Contains(t, description, expectedPart,
						"Description should contain: %s", expectedPart)
				}

				// Verify plan output is included/excluded correctly
				if tt.planOutput == "" {
					assert.NotContains(t, description, "## Terraform Plan Output",
						"Description should not contain plan output section when planOutput is empty")
				}

				w.WriteHeader(201)
				w.Write([]byte(`{"id": 1, "iid": 1, "project_id": 1, "title": "Test", "web_url": "test"}`))
			}))
			defer mockServer.Close()

			os.Setenv("GITLAB_API_TOKEN", "test-token")
			os.Setenv("GITLAB_API_URL", mockServer.URL)

			client := NewGitLabClient(getTestConfig(mockServer.URL, "test-token"))
			_, err := client.CreateDriftIssue(context.Background(), 123, "test-repo", tt.environment, tt.driftIncrement, tt.threshold, tt.planOutput)
			assert.NoError(t, err)
		})
	}
}
