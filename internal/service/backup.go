// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/config"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/database"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/util"
)

// BackupService handles database backup operations.
type BackupService struct {
	repo       *database.Repository
	config     *config.Config
	backupRoot *os.Root
	done       chan struct{}
	wg         sync.WaitGroup
}

// newBackupService returns a BackupService for database backup operations.
func newBackupService(repo *database.Repository, cfg *config.Config) (*BackupService, error) {
	svc := &BackupService{
		repo:   repo,
		config: cfg,
		done:   make(chan struct{}),
	}

	if cfg.Backup.Enabled {
		backupPath := cfg.Backup.GetPath()
		if err := os.MkdirAll(backupPath, 0o750); err != nil {
			return nil, types.NewConfigError("backup.path", fmt.Sprintf("backup directory niet toegankelijk: %v", err))
		}

		root, err := os.OpenRoot(backupPath)
		if err != nil {
			return nil, types.NewConfigError("backup.path", fmt.Sprintf("backup directory niet te openen: %v", err))
		}
		svc.backupRoot = root
	}

	return svc, nil
}

// Close gracefully shuts down the backup service and waits for background tasks.
func (s *BackupService) Close() {
	close(s.done)
	s.wg.Wait()
}

// --- Types ---

// BackupRequest represents the request body for backup operations.
type BackupRequest struct {
	Format      string `json:"format"`
	Compression int    `json:"compression"`
	SchemaOnly  bool   `json:"schema_only"`
	DryRun      bool   `json:"dry_run"`
}

// BackupResult represents the result of a backup operation.
type BackupResult struct {
	Filename      string    `json:"filename"`
	FilePath      string    `json:"file_path,omitzero"`
	Format        string    `json:"format"`
	Size          int64     `json:"size_bytes,omitzero"`
	SizeFormatted string    `json:"size,omitzero"`
	Duration      string    `json:"duration,omitzero"`
	CreatedAt     time.Time `json:"created_at,omitzero"`
	DryRun        bool      `json:"dry_run,omitzero"`
	Command       string    `json:"command,omitzero"`
}

// BackupInfo represents metadata about an existing backup file.
type BackupInfo struct {
	Filename      string    `json:"filename"`
	Format        string    `json:"format"`
	Size          int64     `json:"size_bytes"`
	SizeFormatted string    `json:"size"`
	CreatedAt     time.Time `json:"created_at"`
}

// BackupListResponse represents the response for listing backups.
type BackupListResponse struct {
	Backups    []BackupInfo `json:"backups"`
	TotalSize  int64        `json:"total_size_bytes"`
	TotalCount int          `json:"total_count"`
}

// --- Helpers ---

var safeBackupFilenamePattern = regexp.MustCompile(`^[a-zA-Z0-9_\-.]+$`)

func (s *BackupService) checkEnabled() error {
	if !s.config.Backup.Enabled || s.backupRoot == nil {
		return types.NewConfigError("backup.enabled", "backup functionaliteit is niet ingeschakeld")
	}
	return nil
}

func findPgDump() (string, error) {
	path, err := exec.LookPath("pg_dump")
	if err != nil {
		return "", types.NewConfigError("pg_dump", "postgresql-client niet geïnstalleerd")
	}
	return path, nil
}

func validateBackupFilename(filename string) error {
	if !safeBackupFilenamePattern.MatchString(filename) {
		return types.NewValidationError("filename", "ongeldige bestandsnaam")
	}
	if !strings.HasPrefix(filename, "aeron-backup-") {
		return types.NewValidationError("filename", "geen geldig backup bestand")
	}
	return nil
}

func (s *BackupService) buildPgDumpArgs(format string, compression int) []string {
	args := []string{
		"--format=" + format,
		"--host=" + s.config.Database.Host,
		"--port=" + s.config.Database.Port,
		"--username=" + s.config.Database.User,
		"--dbname=" + s.config.Database.Name,
		"--schema=" + s.config.Database.Schema,
		"--no-password",
	}

	if format == "custom" {
		args = append(args, "--compress="+strconv.Itoa(compression))
	}

	return args
}

func (s *BackupService) validateRequest(req BackupRequest) (format string, compression int, err error) {
	format = strings.ToLower(req.Format)
	if format == "" {
		format = s.config.Backup.GetDefaultFormat()
	}
	if format != "custom" && format != "plain" {
		return "", 0, types.NewValidationError("format", fmt.Sprintf("ongeldig backup formaat: %s (gebruik 'custom' of 'plain')", format))
	}

	compression = req.Compression
	if compression == 0 {
		compression = s.config.Backup.GetDefaultCompression()
	}
	if compression < 0 || compression > 9 {
		return "", 0, types.NewValidationError("compression", fmt.Sprintf("ongeldige compressie waarde: %d (gebruik 0-9)", compression))
	}

	return format, compression, nil
}

func generateBackupFilename(format string) string {
	timestamp := time.Now().Format("2006-01-02-150405")
	ext := "sql"
	if format == "custom" {
		ext = "dump"
	}
	return fmt.Sprintf("aeron-backup-%s.%s", timestamp, ext)
}

func (s *BackupService) executePgDump(ctx context.Context, pgDumpPath, filename, fullPath string, args []string) (os.FileInfo, time.Duration, error) {
	cmd := exec.CommandContext(ctx, pgDumpPath, args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.config.Database.Password)

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		if removeErr := s.backupRoot.Remove(filename); removeErr != nil && !os.IsNotExist(removeErr) {
			slog.Warn("Opruimen van mislukte backup gefaald", "filename", filename, "error", removeErr)
		}

		// Provide clear error message based on error type
		var errMsg string
		switch {
		case ctx.Err() == context.DeadlineExceeded:
			errMsg = fmt.Sprintf("backup timeout na %s (configureer backup.timeout_minutes)", duration.Round(time.Second))
		case ctx.Err() == context.Canceled:
			errMsg = "backup geannuleerd"
		case len(output) > 0:
			errMsg = strings.TrimSpace(string(output))
		default:
			errMsg = err.Error()
		}

		slog.Error("Backup mislukt", "error", err, "duration", duration, "output", string(output))
		return nil, 0, types.NewOperationError("backup maken", errors.New(errMsg))
	}

	fileInfo, err := s.backupRoot.Stat(filename)
	if err != nil {
		return nil, 0, types.NewOperationError("backup maken", fmt.Errorf("backup bestand niet gevonden na creatie: %w", err))
	}

	if err := os.Chmod(fullPath, 0o600); err != nil {
		slog.Warn("Kon bestandspermissies niet instellen", "file", filename, "error", err)
	}

	return fileInfo, duration, nil
}

// --- Public methods ---

// Create creates a database backup using pg_dump.
// Uses a background context because the backup writes to a file - even if the client
// disconnects, we want the backup file to be created successfully.
func (s *BackupService) Create(_ context.Context, req BackupRequest) (*BackupResult, error) {
	if err := s.checkEnabled(); err != nil {
		return nil, err
	}

	pgDumpPath, err := findPgDump()
	if err != nil {
		return nil, err
	}

	format, compression, err := s.validateRequest(req)
	if err != nil {
		return nil, err
	}

	filename := generateBackupFilename(format)
	fullPath := filepath.Join(s.config.Backup.GetPath(), filename)
	args := s.buildPgDumpArgs(format, compression)

	if req.SchemaOnly {
		args = append(args, "--schema-only")
	}
	args = append(args, "--file="+fullPath)
	slog.Debug("pg_dump voorbereid", "format", format, "compression", compression, "schemaOnly", req.SchemaOnly, "filename", filename)

	if req.DryRun {
		return &BackupResult{
			Filename:  filename,
			FilePath:  fullPath,
			Format:    format,
			DryRun:    true,
			Command:   fmt.Sprintf("pg_dump %s", strings.Join(args, " ")),
			CreatedAt: time.Now(),
		}, nil
	}

	// Use background context - backup writes to file, should complete even if client disconnects
	backupCtx, cancel := context.WithTimeout(context.Background(), s.config.Backup.GetTimeout())
	defer cancel()

	fileInfo, duration, err := s.executePgDump(backupCtx, pgDumpPath, filename, fullPath, args)
	if err != nil {
		return nil, err
	}

	slog.Info("Backup succesvol gemaakt",
		"filename", filename,
		"size", util.FormatBytes(fileInfo.Size()),
		"duration", duration.Round(time.Millisecond).String())

	// Run cleanup in background with proper lifecycle management
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		select {
		case <-s.done:
			return
		default:
			s.cleanupOldBackups()
		}
	}()

	return &BackupResult{
		Filename:      filename,
		FilePath:      fullPath,
		Format:        format,
		Size:          fileInfo.Size(),
		SizeFormatted: util.FormatBytes(fileInfo.Size()),
		Duration:      duration.Round(time.Millisecond).String(),
		CreatedAt:     fileInfo.ModTime(),
		DryRun:        false,
	}, nil
}

// List returns a list of available backup files.
func (s *BackupService) List() (*BackupListResponse, error) {
	if err := s.checkEnabled(); err != nil {
		return nil, err
	}

	backupPath := s.config.Backup.GetPath()
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &BackupListResponse{
				Backups:    []BackupInfo{},
				TotalSize:  0,
				TotalCount: 0,
			}, nil
		}
		return nil, types.NewConfigError("backup.path", fmt.Sprintf("backup directory niet leesbaar: %v", err))
	}

	var backups []BackupInfo
	var totalSize int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "aeron-backup-") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		var format string
		switch {
		case strings.HasSuffix(name, ".dump"):
			format = "custom"
		case strings.HasSuffix(name, ".sql"):
			format = "plain"
		default:
			continue
		}

		backups = append(backups, BackupInfo{
			Filename:      name,
			Format:        format,
			Size:          info.Size(),
			SizeFormatted: util.FormatBytes(info.Size()),
			CreatedAt:     info.ModTime(),
		})
		totalSize += info.Size()
	}

	slices.SortFunc(backups, func(a, b BackupInfo) int {
		return b.CreatedAt.Compare(a.CreatedAt) // Descending order
	})

	return &BackupListResponse{
		Backups:    backups,
		TotalSize:  totalSize,
		TotalCount: len(backups),
	}, nil
}

// Delete deletes a specific backup file.
func (s *BackupService) Delete(filename string) error {
	if err := s.checkEnabled(); err != nil {
		return err
	}

	if err := validateBackupFilename(filename); err != nil {
		return err
	}

	if _, err := s.backupRoot.Stat(filename); os.IsNotExist(err) {
		return types.NewNotFoundError("backup", filename)
	}

	if err := s.backupRoot.Remove(filename); err != nil {
		return types.NewOperationError("backup verwijderen", err)
	}

	slog.Info("Backup verwijderd", "filename", filename)
	return nil
}

// GetFilePath returns the full path to a backup file if it exists.
func (s *BackupService) GetFilePath(filename string) (string, error) {
	if err := s.checkEnabled(); err != nil {
		return "", err
	}

	if err := validateBackupFilename(filename); err != nil {
		return "", err
	}

	if _, err := s.backupRoot.Stat(filename); os.IsNotExist(err) {
		return "", types.NewNotFoundError("backup", filename)
	}

	return filepath.Join(s.config.Backup.GetPath(), filename), nil
}

// --- Background cleanup ---

func (s *BackupService) cleanupOldBackups() {
	backups, err := s.List()
	if err != nil {
		slog.Error("Kon backups niet ophalen voor cleanup", "error", err)
		return
	}

	maxAge := time.Duration(s.config.Backup.GetRetentionDays()) * 24 * time.Hour
	maxBackups := s.config.Backup.GetMaxBackups()
	cutoff := time.Now().Add(-maxAge)

	var deleted int

	for _, backup := range backups.Backups {
		if backup.CreatedAt.Before(cutoff) {
			if err := s.Delete(backup.Filename); err != nil {
				slog.Warn("Backup verwijderen mislukt (retention)", "filename", backup.Filename, "error", err)
			} else {
				deleted++
				slog.Info("Oude backup verwijderd (retention)", "filename", backup.Filename)
			}
		}
	}

	backups, err = s.List()
	if err != nil {
		slog.Error("Backup lijst ophalen mislukt tijdens cleanup", "error", err)
		return
	}
	if len(backups.Backups) > maxBackups {
		for i := maxBackups; i < len(backups.Backups); i++ {
			if err := s.Delete(backups.Backups[i].Filename); err != nil {
				slog.Warn("Backup verwijderen mislukt (max_backups)", "filename", backups.Backups[i].Filename, "error", err)
			} else {
				deleted++
				slog.Info("Oude backup verwijderd (max_backups)", "filename", backups.Backups[i].Filename)
			}
		}
	}

	if deleted > 0 {
		slog.Info("Backup cleanup voltooid", "deleted", deleted)
	}
}
