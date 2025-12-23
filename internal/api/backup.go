// Package api provides the HTTP API server for the Aeron radio automation system.
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/service"
)

// BackupDeleteResponse represents the response format for backup delete operations.
type BackupDeleteResponse struct {
	Message  string `json:"message"`
	Filename string `json:"filename"`
}

// BackupStartResponse represents the response when a backup is started.
type BackupStartResponse struct {
	Message string `json:"message"`
	Check   string `json:"check"`
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	var req service.BackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
		return
	}

	if err := s.service.Backup.Start(req); err != nil {
		respondError(w, errorCode(err), err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, BackupStartResponse{
		Message: "Backup gestart op achtergrond",
		Check:   "/api/db/backup/status",
	})
}

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.Backup.List()
	if err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *Server) handleBackupStatus(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, s.service.Backup.Status())
}

func (s *Server) handleDownloadBackupFile(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	filePath, err := s.service.Backup.GetFilePath(filename)
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

	if err := s.service.Backup.Delete(filename); err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, BackupDeleteResponse{
		Message:  "Backup succesvol verwijderd",
		Filename: filename,
	})
}
