package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// APIResponse is the standard response format for all API endpoints.
// It provides a consistent structure for both successful and error responses.
type APIResponse struct {
	Success bool        `json:"success"`         // Whether the operation was successful
	Data    interface{} `json:"data,omitempty"`  // Response data for successful operations
	Error   string      `json:"error,omitempty"` // Error message for failed operations
}

// respondJSON sends a successful JSON response with the specified status code and data.
// It automatically sets the success field to true and includes the provided data.
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	}); err != nil {
		slog.Debug("Schrijven JSON response naar client mislukt", "error", err)
	}
}

// respondError sends an error JSON response with the specified status code and error message.
// It automatically sets the success field to false and includes the error message.
func respondError(w http.ResponseWriter, statusCode int, errorMsg string) {
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   errorMsg,
	}); err != nil {
		slog.Debug("Schrijven error response naar client mislukt", "error", err)
	}
}

// errorCode determines the appropriate HTTP status code based on the error type.
// It first checks custom error types using errors.As(), then falls back to string
// matching for backwards compatibility during the migration period.
func errorCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Check custom error types first (preferred method)
	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		return http.StatusNotFound
	}

	var noImage *NoImageError
	if errors.As(err, &noImage) {
		return http.StatusNotFound
	}

	var validation *ValidationError
	if errors.As(err, &validation) {
		return http.StatusBadRequest
	}

	var imageProc *ImageProcessingError
	if errors.As(err, &imageProc) {
		return http.StatusBadRequest
	}

	var config *ConfigurationError
	if errors.As(err, &config) {
		return http.StatusInternalServerError
	}

	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return http.StatusInternalServerError
	}

	var backupErr *BackupError
	if errors.As(err, &backupErr) {
		return http.StatusInternalServerError
	}

	// Fallback to string matching for backwards compatibility during migration
	errorMsg := err.Error()

	// 404 Not Found errors
	if strings.Contains(errorMsg, ErrSuffixNotExists) ||
		strings.Contains(errorMsg, "heeft geen afbeelding") {
		return http.StatusNotFound
	}

	// 400 Bad Request errors
	if errorMsg == "afbeelding is verplicht" ||
		errorMsg == "gebruik óf URL óf upload, niet beide" ||
		strings.Contains(errorMsg, "ongeldig type") ||
		strings.Contains(errorMsg, "te klein") ||
		strings.Contains(errorMsg, "niet ondersteund") ||
		strings.Contains(errorMsg, "ongeldige") {
		return http.StatusBadRequest
	}

	// 500 Internal Server Error for everything else
	return http.StatusInternalServerError
}
