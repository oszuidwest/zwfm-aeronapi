package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// APIResponse is the standard response format for all API endpoints
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// respondJSON sends a successful JSON response
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

// respondError sends an error JSON response
func respondError(w http.ResponseWriter, statusCode int, errorMsg string) {
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   errorMsg,
	})
}

// errorCode returns the appropriate HTTP status code for an error
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
