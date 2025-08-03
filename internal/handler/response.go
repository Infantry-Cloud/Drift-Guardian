package handler

import (
	"fmt"
	"net/http"
)

// ResponseWriterImpl implements ResponseWriter interface
type ResponseWriterImpl struct{}

// NewResponseWriter creates a new response writer instance
func NewResponseWriter() *ResponseWriterImpl {
	return &ResponseWriterImpl{}
}

// WriteSuccess writes a successful response with headers and body
func (r *ResponseWriterImpl) WriteSuccess(w http.ResponseWriter, payload interface{}, headers map[string]string) error {
	// Set custom headers
	for key, value := range headers {
		w.Header().Set(key, value)
	}

	// Set content type and status
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	// Write response body
	switch v := payload.(type) {
	case string:
		_, err := fmt.Fprint(w, v)
		return err
	case []byte:
		_, err := w.Write(v)
		return err
	default:
		_, err := fmt.Fprintf(w, "%v", v)
		return err
	}
}

// WriteError writes an error response with appropriate status code
func (r *ResponseWriterImpl) WriteError(w http.ResponseWriter, message string, statusCode int) error {
	http.Error(w, message, statusCode)
	return nil
}
