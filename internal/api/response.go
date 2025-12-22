// Package api provides the HTTP API server for the Aeron radio automation system.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
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

	var notFound *types.NotFoundError
	if errors.As(err, &notFound) {
		return http.StatusNotFound
	}

	var noImage *types.NoImageError
	if errors.As(err, &noImage) {
		return http.StatusNotFound
	}

	var validation *types.ValidationError
	if errors.As(err, &validation) {
		return http.StatusBadRequest
	}

	var imageProc *types.ImageProcessingError
	if errors.As(err, &imageProc) {
		return http.StatusBadRequest
	}

	var config *types.ConfigurationError
	if errors.As(err, &config) {
		return http.StatusInternalServerError
	}

	var dbErr *types.DatabaseError
	if errors.As(err, &dbErr) {
		return http.StatusInternalServerError
	}

	var backupErr *types.BackupError
	if errors.As(err, &backupErr) {
		return http.StatusInternalServerError
	}

	errorMsg := err.Error()

	if strings.Contains(errorMsg, "bestaat niet") ||
		strings.Contains(errorMsg, "heeft geen afbeelding") {
		return http.StatusNotFound
	}

	if errorMsg == "afbeelding is verplicht" ||
		errorMsg == "gebruik óf URL óf upload, niet beide" ||
		strings.Contains(errorMsg, "ongeldig type") ||
		strings.Contains(errorMsg, "te klein") ||
		strings.Contains(errorMsg, "niet ondersteund") ||
		strings.Contains(errorMsg, "ongeldige") {
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}
