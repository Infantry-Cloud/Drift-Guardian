//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
)

// TestRedisRepository_InitializeEnvironment tests environment initialization
func TestRedisRepository_InitializeEnvironment(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		key         string
		tier        string
		projectID   string
		threshold   string
		setupMock   func(mock redismock.ClientMock)
		expectError bool
		expectNew   bool
	}{
		{
			name:      "new environment initialization",
			key:       "test-repo:production",
			tier:      "prod",
			projectID: "123",
			threshold: "3",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectExists("test-repo:production").SetVal(0) // Key doesn't exist
				mock.ExpectHMSet("test-repo:production", map[string]interface{}{
					"driftThreshold":  "3",
					"environmentTier": "prod",
					"projectID":       "123",
					"driftIncrement":  "0",
				}).SetVal(true)
			},
			expectError: false,
			expectNew:   true,
		},
		{
			name:      "existing environment",
			key:       "test-repo:staging",
			tier:      "nonprod",
			projectID: "456",
			threshold: "5",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectExists("test-repo:staging").SetVal(1) // Key exists
			},
			expectError: false,
			expectNew:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			repo := NewRedisRepository(client)

			tt.setupMock(mock)

			isNew, err := repo.InitializeEnvironment(ctx, tt.key, tt.tier, tt.projectID, tt.threshold)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectNew, isNew)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRedisRepository_IncrementDrift tests drift increment operations
func TestRedisRepository_IncrementDrift(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		key           string
		setupMock     func(mock redismock.ClientMock)
		expectError   bool
		expectedDrift int
	}{
		{
			name: "successful drift increment",
			key:  "test-repo:production",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHIncrBy("test-repo:production", "driftIncrement", 1).SetVal(3)
			},
			expectError:   false,
			expectedDrift: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			repo := NewRedisRepository(client)

			tt.setupMock(mock)

			driftCount, err := repo.IncrementDrift(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDrift, driftCount)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRedisRepository_ResetDrift tests drift reset operations
func TestRedisRepository_ResetDrift(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		key         string
		setupMock   func(mock redismock.ClientMock)
		expectError bool
	}{
		{
			name: "successful drift reset",
			key:  "test-repo:production",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHSet("test-repo:production", "driftIncrement", "0").SetVal(1)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			repo := NewRedisRepository(client)

			tt.setupMock(mock)

			err := repo.ResetDrift(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRedisRepository_GetEnvironmentData tests environment data retrieval
func TestRedisRepository_GetEnvironmentData(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		key            string
		setupMock      func(mock redismock.ClientMock)
		expectError    bool
		expectedFields map[string]string
	}{
		{
			name: "successful data retrieval",
			key:  "test-repo:production",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHGetAll("test-repo:production").SetVal(map[string]string{
					"driftThreshold":  "3",
					"environmentTier": "prod",
					"projectID":       "123",
					"driftIncrement":  "2",
				})
			},
			expectError: false,
			expectedFields: map[string]string{
				"driftThreshold":  "3",
				"environmentTier": "prod",
				"projectID":       "123",
				"driftIncrement":  "2",
			},
		},
		{
			name: "empty data retrieval",
			key:  "nonexistent-key",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHGetAll("nonexistent-key").SetVal(map[string]string{})
			},
			expectError:    true,
			expectedFields: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			repo := NewRedisRepository(client)

			tt.setupMock(mock)

			fields, err := repo.GetEnvironmentData(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedFields, fields)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRedisRepository_SetField tests field setting
func TestRedisRepository_SetField(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		key         string
		field       string
		value       string
		setupMock   func(mock redismock.ClientMock)
		expectError bool
	}{
		{
			name:  "successful field set",
			key:   "test-repo:production",
			field: "issueId",
			value: "10",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHSet("test-repo:production", "issueId", "10").SetVal(1)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			repo := NewRedisRepository(client)

			tt.setupMock(mock)

			err := repo.SetField(ctx, tt.key, tt.field, tt.value)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRedisRepository_GetField tests field retrieval
func TestRedisRepository_GetField(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		key           string
		field         string
		setupMock     func(mock redismock.ClientMock)
		expectError   bool
		expectedValue string
	}{
		{
			name:  "successful field get",
			key:   "test-repo:production",
			field: "issueId",
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHGet("test-repo:production", "issueId").SetVal("10")
			},
			expectError:   false,
			expectedValue: "10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			repo := NewRedisRepository(client)

			tt.setupMock(mock)

			value, err := repo.GetField(ctx, tt.key, tt.field)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
