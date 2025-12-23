// Package api provides the HTTP API server for the Aeron radio automation system.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
)

// Response is the standard response format for all API endpoints.
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func respondJSON(w http.ResponseWriter, statusCode int, data any) {
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    data,
	}); err != nil {
		slog.Debug("Schrijven JSON response naar client mislukt", "error", err)
	}
}

func respondError(w http.ResponseWriter, statusCode int, errorMsg string) {
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   errorMsg,
	}); err != nil {
		slog.Debug("Schrijven error response naar client mislukt", "error", err)
	}
}

func errorCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Check for HTTPError interface (all typed errors implement this)
	var httpErr types.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode()
	}

	// Fallback for legacy string matching
	return errorCodeFromMessage(err.Error())
}

func errorCodeFromMessage(msg string) int {
	if strings.Contains(msg, "bestaat niet") ||
		strings.Contains(msg, "heeft geen afbeelding") {
		return http.StatusNotFound
	}

	if msg == "afbeelding is verplicht" ||
		msg == "gebruik óf URL óf upload, niet beide" ||
		strings.Contains(msg, "ongeldig type") ||
		strings.Contains(msg, "te klein") ||
		strings.Contains(msg, "niet ondersteund") ||
		strings.Contains(msg, "ongeldige") {
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}
