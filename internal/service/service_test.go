//go:build unit

package service

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPayloadValidator tests payload validation logic comprehensively
func TestPayloadValidator(t *testing.T) {
	// Create a minimal service instance for testing validation
	service := &DriftServiceImpl{}

	tests := []struct {
		name          string
		payload       Payload
		expectedError string
	}{
		{
			name: "valid complete payload",
			payload: Payload{
				RepoName:        "valid-repo",
				Branch:          "main",
				Environment:     "production",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
				Operation:       "plan",
				ExitCode:        2,
				Scheduled:       true,
				Timestamp:       "2025-01-31T10:30:00Z",
			},
			expectedError: "",
		},
		{
			name: "valid minimal payload",
			payload: Payload{
				RepoName:        "minimal-repo",
				Branch:          "develop",
				Environment:     "staging",
				EnvironmentTier: "nonprod",
				ProjectID:       "67890",
				Operation:       "apply",
			},
			expectedError: "",
		},
		{
			name: "missing repoName",
			payload: Payload{
				Branch:          "main",
				Environment:     "production",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
				Operation:       "plan",
			},
			expectedError: "missing repoName in payload",
		},
		{
			name: "empty repoName",
			payload: Payload{
				RepoName:        "",
				Branch:          "main",
				Environment:     "production",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
				Operation:       "plan",
			},
			expectedError: "missing repoName in payload",
		},
		{
			name: "missing branchName",
			payload: Payload{
				RepoName:        "test-repo",
				Environment:     "production",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
				Operation:       "plan",
			},
			expectedError: "missing branchName in payload",
		},
		{
			name: "empty branchName",
			payload: Payload{
				RepoName:        "test-repo",
				Branch:          "",
				Environment:     "production",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
				Operation:       "plan",
			},
			expectedError: "missing branchName in payload",
		},
		{
			name: "missing environment",
			payload: Payload{
				RepoName:        "test-repo",
				Branch:          "main",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
				Operation:       "plan",
			},
			expectedError: "missing environment in payload",
		},
		{
			name: "empty environment",
			payload: Payload{
				RepoName:        "test-repo",
				Branch:          "main",
				Environment:     "",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
				Operation:       "plan",
			},
			expectedError: "missing environment in payload",
		},
		{
			name: "missing environmentTier",
			payload: Payload{
				RepoName:    "test-repo",
				Branch:      "main",
				Environment: "production",
				ProjectID:   "12345",
				Operation:   "plan",
			},
			expectedError: "missing environmentTier in payload",
		},
		{
			name: "empty environmentTier",
			payload: Payload{
				RepoName:        "test-repo",
				Branch:          "main",
				Environment:     "production",
				EnvironmentTier: "",
				ProjectID:       "12345",
				Operation:       "plan",
			},
			expectedError: "missing environmentTier in payload",
		},
		{
			name: "missing projectId",
			payload: Payload{
				RepoName:        "test-repo",
				Branch:          "main",
				Environment:     "production",
				EnvironmentTier: "prod",
				Operation:       "plan",
			},
			expectedError: "missing projectId in payload",
		},
		{
			name: "empty projectId",
			payload: Payload{
				RepoName:        "test-repo",
				Branch:          "main",
				Environment:     "production",
				EnvironmentTier: "prod",
				ProjectID:       "",
				Operation:       "plan",
			},
			expectedError: "missing projectId in payload",
		},
		{
			name: "missing operation",
			payload: Payload{
				RepoName:        "test-repo",
				Branch:          "main",
				Environment:     "production",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
			},
			expectedError: "invalid terraform operation in payload",
		},
		{
			name: "empty operation",
			payload: Payload{
				RepoName:        "test-repo",
				Branch:          "main",
				Environment:     "production",
				EnvironmentTier: "prod",
				ProjectID:       "12345",
				Operation:       "",
			},
			expectedError: "invalid terraform operation in payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidatePayload(&tt.payload)
			if tt.expectedError == "" {
				assert.NoError(t, err, "Validation should pass for valid payload")
			} else {
				assert.Error(t, err, "Validation should fail for invalid payload")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message should contain expected text")
			}
		})
	}
}

// TestGenerateKey tests Redis key generation
func TestGenerateKey(t *testing.T) {
	service := &DriftServiceImpl{}

	tests := []struct {
		name        string
		repoName    string
		environment string
		expected    string
	}{
		{
			name:        "standard repo and environment",
			repoName:    "my-terraform-repo",
			environment: "production",
			expected:    "my-terraform-repo:production",
		},
		{
			name:        "repo with dashes and environment with numbers",
			repoName:    "infrastructure-v2",
			environment: "staging-us-east-1",
			expected:    "infrastructure-v2:staging-us-east-1",
		},
		{
			name:        "complex environment name",
			repoName:    "app",
			environment: "prod-eu-west-2-cluster-1",
			expected:    "app:prod-eu-west-2-cluster-1",
		},
		{
			name:        "single character inputs",
			repoName:    "a",
			environment: "b",
			expected:    "a:b",
		},
		{
			name:        "repo with underscores",
			repoName:    "my_terraform_project",
			environment: "development",
			expected:    "my_terraform_project:development",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.GenerateKey(tt.repoName, tt.environment)
			assert.Equal(t, tt.expected, result, "Redis key should match expected format")
		})
	}
}

// TestGenerateKey_EdgeCases tests edge cases for Redis key generation
func TestGenerateKey_EdgeCases(t *testing.T) {
	service := &DriftServiceImpl{}

	tests := []struct {
		name        string
		repoName    string
		environment string
		expected    string
	}{
		{
			name:        "empty repo name",
			repoName:    "",
			environment: "production",
			expected:    ":production",
		},
		{
			name:        "empty environment",
			repoName:    "my-repo",
			environment: "",
			expected:    "my-repo:",
		},
		{
			name:        "both empty",
			repoName:    "",
			environment: "",
			expected:    ":",
		},
		{
			name:        "repo name with colon",
			repoName:    "repo:with:colons",
			environment: "prod",
			expected:    "repo:with:colons:prod",
		},
		{
			name:        "environment with colon",
			repoName:    "repo",
			environment: "env:with:colons",
			expected:    "repo:env:with:colons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.GenerateKey(tt.repoName, tt.environment)
			assert.Equal(t, tt.expected, result, "Redis key should handle edge cases correctly")
		})
	}
}

// TestProjectIDConversion tests project ID string to int conversion used in service layer
func TestProjectIDConversion(t *testing.T) {
	tests := []struct {
		name         string
		projectIDStr string
		expected     int
		expectError  bool
	}{
		{
			name:         "valid project ID",
			projectIDStr: "12345",
			expected:     12345,
			expectError:  false,
		},
		{
			name:         "zero project ID",
			projectIDStr: "0",
			expected:     0,
			expectError:  false,
		},
		{
			name:         "invalid non-numeric project ID",
			projectIDStr: "invalid",
			expected:     0,
			expectError:  true,
		},
		{
			name:         "empty project ID",
			projectIDStr: "",
			expected:     0,
			expectError:  true,
		},
		{
			name:         "negative project ID",
			projectIDStr: "-1",
			expected:     -1,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the project ID conversion logic used in service layer
			result, err := strconv.Atoi(tt.projectIDStr)

			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid project ID string")
			} else {
				assert.NoError(t, err, "Should not return error for valid project ID string")
				assert.Equal(t, tt.expected, result, "Converted project ID should match expected value")
			}
		})
	}
}
