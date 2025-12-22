// Package api provides the HTTP API server for the Aeron radio automation system.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/oszuidwest/zwfm-aeronapi/internal/service"
)

// VacuumRequest represents the JSON request body for vacuum operations.
type VacuumRequest struct {
	Tables  []string `json:"tables"`  // Specific tables to vacuum (empty = auto-select based on bloat)
	Analyze bool     `json:"analyze"` // Run ANALYZE after VACUUM
	DryRun  bool     `json:"dry_run"` // Only show what would be done, don't execute
}

// AnalyzeRequest represents the JSON request body for analyze operations.
type AnalyzeRequest struct {
	Tables []string `json:"tables"` // Specific tables to analyze (empty = auto-select)
}

func (s *Server) handleDatabaseHealth(w http.ResponseWriter, r *http.Request) {
	health, err := s.service.GetDatabaseHealth(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, health)
}

func (s *Server) handleVacuum(w http.ResponseWriter, r *http.Request) {
	var req VacuumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
		return
	}

	result, err := s.service.VacuumTables(r.Context(), service.VacuumOptions{
		Tables:  req.Tables,
		Analyze: req.Analyze,
		DryRun:  req.DryRun,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
		return
	}

	result, err := s.service.AnalyzeTables(r.Context(), req.Tables)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}
