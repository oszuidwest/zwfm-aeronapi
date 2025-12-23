// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"encoding/base64"
	"io"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/config"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/database"
)

// AeronService is the main service that provides access to all sub-services.
type AeronService struct {
	Media       *MediaService
	Backup      *BackupService
	Maintenance *MaintenanceService

	repo   *database.Repository
	config *config.Config
}

// New creates a new AeronService instance with all sub-services.
func New(db *sqlx.DB, cfg *config.Config) (*AeronService, error) {
	repo := database.NewRepository(db, cfg.Database.Schema)

	backupSvc, err := newBackupService(repo, cfg)
	if err != nil {
		return nil, err
	}

	return &AeronService{
		Media:       newMediaService(repo, cfg),
		Backup:      backupSvc,
		Maintenance: newMaintenanceService(repo, cfg),
		repo:        repo,
		config:      cfg,
	}, nil
}

// Config returns the service configuration.
func (s *AeronService) Config() *config.Config {
	return s.config
}

// Repository returns the database repository.
func (s *AeronService) Repository() *database.Repository {
	return s.repo
}

// Close gracefully shuts down all services.
func (s *AeronService) Close() {
	s.Backup.Close()
}

// DecodeBase64 decodes a base64-encoded string into raw bytes.
func DecodeBase64(data string) ([]byte, error) {
	if _, after, found := strings.Cut(data, ","); found {
		data = after
	}
	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}
