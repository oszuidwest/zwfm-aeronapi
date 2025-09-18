package main

import (
	"encoding/json"
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
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

// respondError sends an error JSON response with the specified status code and error message.
// It automatically sets the success field to false and includes the error message.
func respondError(w http.ResponseWriter, statusCode int, errorMsg string) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   errorMsg,
	})
}

// errorCode determines the appropriate HTTP status code based on the error message content.
// It maps common Dutch error messages to their corresponding HTTP status codes.
func errorCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

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
