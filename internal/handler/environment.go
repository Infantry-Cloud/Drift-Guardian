package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"drift-guardian/internal/service"
)

// EnvironmentHandlerImpl implements EnvironmentHandler interface
type EnvironmentHandlerImpl struct {
	driftService service.DriftService
	writer       ResponseWriter
}

// NewEnvironmentHandler creates a new environment handler instance
func NewEnvironmentHandler(
	driftService service.DriftService,
	writer ResponseWriter,
) *EnvironmentHandlerImpl {
	return &EnvironmentHandlerImpl{
		driftService: driftService,
		writer:       writer,
	}
}

// HandleEnvironments processes HTTP requests to the /environments endpoint
func (h *EnvironmentHandlerImpl) HandleEnvironments(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		_ = h.writer.WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		_ = h.writer.WriteError(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Parse the JSON payload
	var payload service.Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		_ = h.writer.WriteError(w, "Error parsing JSON payload", http.StatusBadRequest)
		return
	}

	// Validate the payload
	if err := h.driftService.ValidatePayload(&payload); err != nil {
		_ = h.writer.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Process drift detection
	result, err := h.driftService.ProcessDriftDetection(ctx, payload)
	if err != nil {
		_ = h.writer.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Prepare response headers
	headers := make(map[string]string)
	if result.EnvironmentTier != "" {
		headers["X-Environment-Tier"] = result.EnvironmentTier
	}
	if result.DriftIncrement != "" {
		headers["X-Drift-Increment"] = result.DriftIncrement
	}
	if result.ProjectID != "" {
		headers["X-Project-ID"] = result.ProjectID
	}
	if result.IssueID != "" {
		headers["X-Issue-ID"] = result.IssueID
	}
	if result.IssueURL != "" {
		headers["X-Issue-URL"] = result.IssueURL
	}

	// Prepare response body (maintaining exact format for backward compatibility)
	responseBody := fmt.Sprintf(
		"Environment values retrieved for repository: %s, environment: %s\\nValues: {\"environmentTier\": \"%s\", \"projectID\": \"%s\", \"driftIncrement\": \"%s\", \"issueID\": \"%s\", \"issueURL\": \"%s\", \"log\": %s}",
		payload.RepoName,
		payload.Environment,
		result.EnvironmentTier,
		result.ProjectID,
		result.DriftIncrement,
		result.IssueID,
		result.IssueURL,
		result.Log["log"],
	)

	// Write successful response
	err = h.writer.WriteSuccess(w, responseBody, headers)
	if err != nil {
		// Fallback error response
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
