//go:build unit

package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"drift-guardian/internal/service"
)

// MockDriftService is a mock implementation of DriftService
type MockDriftService struct {
	mock.Mock
}

func (m *MockDriftService) ProcessDriftDetection(ctx context.Context, payload service.Payload) (*service.DriftResult, error) {
	args := m.Called(ctx, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.DriftResult), args.Error(1)
}

func (m *MockDriftService) HandleThresholdBreach(ctx context.Context, env service.EnvironmentInfo, driftCount int) error {
	args := m.Called(ctx, env, driftCount)
	return args.Error(0)
}

func (m *MockDriftService) ResetDriftIncrement(ctx context.Context, env service.EnvironmentInfo, operation string) error {
	args := m.Called(ctx, env, operation)
	return args.Error(0)
}

func (m *MockDriftService) ValidatePayload(payload *service.Payload) error {
	args := m.Called(payload)
	return args.Error(0)
}

func (m *MockDriftService) GenerateKey(repoName, environment string) string {
	args := m.Called(repoName, environment)
	return args.String(0)
}

// MockResponseWriter is a mock implementation of ResponseWriter
type MockResponseWriter struct {
	mock.Mock
}

func (m *MockResponseWriter) WriteSuccess(w http.ResponseWriter, payload interface{}, headers map[string]string) error {
	args := m.Called(w, payload, headers)
	return args.Error(0)
}

func (m *MockResponseWriter) WriteError(w http.ResponseWriter, message string, statusCode int) error {
	args := m.Called(w, message, statusCode)
	// Actually write the error for test assertions
	http.Error(w, message, statusCode)
	return args.Error(0)
}

func TestEnvironmentHandler_MethodValidation(t *testing.T) {
	// Setup mocks
	mockService := new(MockDriftService)
	mockWriter := new(MockResponseWriter)

	handler := NewEnvironmentHandler(mockService, mockWriter)
	ctx := context.Background()

	methods := []string{"GET", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run("method_"+method+"_should_return_405", func(t *testing.T) {
			req := httptest.NewRequest(method, "/environments", nil)
			rec := httptest.NewRecorder()

			// Setup mock expectation
			mockWriter.On("WriteError", rec, "Method not allowed", http.StatusMethodNotAllowed).Return(nil).Once()

			handler.HandleEnvironments(rec, req, ctx)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
			assert.Equal(t, "Method not allowed\n", rec.Body.String())

			// Verify mock was called
			mockWriter.AssertExpectations(t)
		})

		// Reset mock for next iteration
		mockWriter.ExpectedCalls = nil
	}
}

func TestEnvironmentHandler_InvalidPayload(t *testing.T) {
	// Setup mocks
	mockService := new(MockDriftService)
	mockWriter := new(MockResponseWriter)

	handler := NewEnvironmentHandler(mockService, mockWriter)
	ctx := context.Background()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectedError  string
		setupMocks     func()
	}{
		{
			name:           "invalid JSON",
			requestBody:    `{"repoName": "test", "invalid": }`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Error parsing JSON payload",
			setupMocks: func() {
				mockWriter.On("WriteError", mock.Anything, "Error parsing JSON payload", http.StatusBadRequest).Return(nil).Once()
			},
		},
		{
			name:           "validation error",
			requestBody:    `{"repoName": "test"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Missing branchName in payload",
			setupMocks: func() {
				mockService.On("ValidatePayload", mock.AnythingOfType("*service.Payload")).Return(errors.New("Missing branchName in payload")).Once()
				mockWriter.On("WriteError", mock.Anything, "Missing branchName in payload", http.StatusBadRequest).Return(nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			tt.setupMocks()

			req := httptest.NewRequest("POST", "/environments", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.HandleEnvironments(rec, req, ctx)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, tt.expectedError+"\n", rec.Body.String())

			// Verify mocks
			mockService.AssertExpectations(t)
			mockWriter.AssertExpectations(t)

			// Reset mocks for next test
			mockService.ExpectedCalls = nil
			mockWriter.ExpectedCalls = nil
		})
	}
}

func TestEnvironmentHandler_SuccessfulRequest(t *testing.T) {
	// Setup mocks
	mockService := new(MockDriftService)
	mockWriter := new(MockResponseWriter)

	handler := NewEnvironmentHandler(mockService, mockWriter)
	ctx := context.Background()

	validPayload := `{
		"repoName": "test-repo",
		"branchName": "main", 
		"environment": "production",
		"environmentTier": "prod",
		"projectId": "123",
		"operation": "plan"
	}`

	expectedResult := &service.DriftResult{
		EnvironmentTier: "prod",
		ProjectID:       "123",
		DriftIncrement:  "1",
		IssueID:         "",
		IssueURL:        "",
		Log:             map[string]string{"log": `{"timestamp": "2025-01-01T00:00:00Z", "operation": "plan"}`},
	}

	// Setup mock expectations
	mockService.On("ValidatePayload", mock.AnythingOfType("*service.Payload")).Return(nil).Once()
	mockService.On("ProcessDriftDetection", ctx, mock.AnythingOfType("service.Payload")).Return(expectedResult, nil).Once()
	mockWriter.On("WriteSuccess", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return(nil).Once()

	req := httptest.NewRequest("POST", "/environments", bytes.NewBufferString(validPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleEnvironments(rec, req, ctx)

	// Verify mocks were called
	mockService.AssertExpectations(t)
	mockWriter.AssertExpectations(t)
}

func TestEnvironmentHandler_ServiceError(t *testing.T) {
	// Setup mocks
	mockService := new(MockDriftService)
	mockWriter := new(MockResponseWriter)

	handler := NewEnvironmentHandler(mockService, mockWriter)
	ctx := context.Background()

	validPayload := `{"repoName": "test", "branchName": "main", "environment": "prod", "environmentTier": "prod", "projectId": "123", "operation": "plan"}`

	// Setup mock expectations
	mockService.On("ValidatePayload", mock.AnythingOfType("*service.Payload")).Return(nil).Once()
	mockService.On("ProcessDriftDetection", ctx, mock.AnythingOfType("service.Payload")).Return(nil, errors.New("service error")).Once()
	mockWriter.On("WriteError", mock.Anything, "service error", http.StatusInternalServerError).Return(nil).Once()

	req := httptest.NewRequest("POST", "/environments", bytes.NewBufferString(validPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleEnvironments(rec, req, ctx)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// Verify mocks were called
	mockService.AssertExpectations(t)
	mockWriter.AssertExpectations(t)
}
