// Package api provides the HTTP API server for the Aeron radio automation system.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oszuidwest/zwfm-aeronapi/internal/service"
)

// BackupDeleteResponse represents the response format for backup delete operations.
type BackupDeleteResponse struct {
	Message  string `json:"message"`
	Filename string `json:"filename"`
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	var req service.BackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
		return
	}

	result, err := s.service.CreateBackup(r.Context(), req)
	if err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	format := query.Get("format")
	if format == "" {
		format = s.config.Backup.GetDefaultFormat()
	}

	compression := s.config.Backup.GetDefaultCompression()
	if c := query.Get("compression"); c != "" {
		if val, err := strconv.Atoi(c); err == nil {
			compression = val
		}
	}

	// Generate filename for download
	filenamePrefix := "download"
	var ext string
	if format == "custom" {
		ext = "dump"
	} else {
		ext = "sql"
	}
	filename := "aeron-backup-" + filenamePrefix + "." + ext

	// Set headers for file download
	w.Header().Del("Content-Type")
	if format == "custom" {
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		w.Header().Set("Content-Type", "application/sql")
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)

	if err := s.service.StreamBackup(r.Context(), w, format, compression); err != nil {
		// Headers already sent, can't send error JSON
		slog.Error("Backup stream mislukt", "error", err)
	}
}

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.ListBackups()
	if err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *Server) handleDownloadBackupFile(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	filePath, err := s.service.GetBackupFilePath(filename)
	if err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	// Determine content type
	w.Header().Del("Content-Type")
	if strings.HasSuffix(filename, ".dump") {
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		w.Header().Set("Content-Type", "application/sql")
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)

	http.ServeFile(w, r, filePath)
}

func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	// Require confirmation header
	const confirmHeader = "X-Confirm-Delete"
	if r.Header.Get(confirmHeader) != filename {
		respondError(w, http.StatusBadRequest, "Bevestigingsheader ontbreekt: "+confirmHeader+" moet de bestandsnaam bevatten")
		return
	}

	if err := s.service.DeleteBackup(filename); err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, BackupDeleteResponse{
		Message:  "Backup succesvol verwijderd",
		Filename: filename,
	})
}
