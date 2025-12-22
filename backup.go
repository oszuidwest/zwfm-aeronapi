package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BackupFormat represents the pg_dump output format.
type BackupFormat string

const (
	BackupFormatCustom BackupFormat = "custom" // pg_dump -Fc (binary, compressed)
	BackupFormatPlain  BackupFormat = "plain"  // pg_dump -Fp (SQL text)
)

// BackupRequest represents the JSON request body for backup operations.
type BackupRequest struct {
	Format      string `json:"format"`      // "custom" or "plain"
	Compression int    `json:"compression"` // 0-9 compression level
	SchemaOnly  bool   `json:"schema_only"` // Only backup schema, no data
	DryRun      bool   `json:"dry_run"`     // Only show what would be done
}

// BackupResult represents the result of a backup operation.
type BackupResult struct {
	Filename      string    `json:"filename"`
	FilePath      string    `json:"file_path,omitempty"`
	Format        string    `json:"format"`
	Size          int64     `json:"size_bytes"`
	SizeFormatted string    `json:"size"`
	Duration      string    `json:"duration"`
	CreatedAt     time.Time `json:"created_at"`
	DryRun        bool      `json:"dry_run"`
	Command       string    `json:"command,omitempty"`
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

// validFilenameRegex validates backup filenames to prevent path traversal.
var validFilenameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)

// CreateBackup creates a database backup using pg_dump.
func (s *AeronService) CreateBackup(ctx context.Context, req BackupRequest) (*BackupResult, error) {
	if !s.config.Backup.Enabled {
		return nil, &ConfigurationError{Field: "backup.enabled", Message: "backup functionaliteit is niet ingeschakeld"}
	}

	// Check if pg_dump is available
	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		return nil, &ConfigurationError{Field: "pg_dump", Message: "postgresql-client niet geïnstalleerd"}
	}

	// Validate and set defaults
	format := strings.ToLower(req.Format)
	if format == "" {
		format = s.config.Backup.GetDefaultFormat()
	}
	if format != "custom" && format != "plain" {
		return nil, &ValidationError{Field: "format", Message: fmt.Sprintf("ongeldig backup formaat: %s (gebruik 'custom' of 'plain')", format)}
	}

	compression := req.Compression
	if compression == 0 {
		compression = s.config.Backup.GetDefaultCompression()
	}
	if compression < 0 || compression > 9 {
		return nil, &ValidationError{Field: "compression", Message: fmt.Sprintf("ongeldige compressie waarde: %d (gebruik 0-9)", compression)}
	}

	// Ensure backup directory exists
	backupPath := s.config.Backup.GetPath()
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		return nil, &ConfigurationError{Field: "backup.path", Message: fmt.Sprintf("backup directory niet toegankelijk: %v", err)}
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02-150405")
	var ext string
	if format == "custom" {
		ext = "dump"
	} else {
		ext = "sql"
	}
	filename := fmt.Sprintf("aeron-backup-%s.%s", timestamp, ext)
	fullPath := filepath.Join(backupPath, filename)

	// Build pg_dump arguments
	args := []string{
		"--format=" + format,
		"--host=" + s.config.Database.Host,
		"--port=" + s.config.Database.Port,
		"--username=" + s.config.Database.User,
		"--dbname=" + s.config.Database.Name,
		"--schema=" + s.config.Database.Schema,
		"--no-password",
	}

	// Add compression for custom format
	if format == "custom" {
		args = append(args, "--compress="+strconv.Itoa(compression))
	}

	// Schema only option
	if req.SchemaOnly {
		args = append(args, "--schema-only")
	}

	// Add output file
	args = append(args, "--file="+fullPath)

	// Build command string for dry-run display
	cmdDisplay := fmt.Sprintf("pg_dump %s", strings.Join(args, " "))

	if req.DryRun {
		return &BackupResult{
			Filename:  filename,
			FilePath:  fullPath,
			Format:    format,
			DryRun:    true,
			Command:   cmdDisplay,
			CreatedAt: time.Now(),
		}, nil
	}

	// Execute pg_dump
	cmd := exec.CommandContext(ctx, pgDumpPath, args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.config.Database.Password)

	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		// Clean up partial file if it exists
		if removeErr := os.Remove(fullPath); removeErr != nil && !os.IsNotExist(removeErr) {
			slog.Warn("Opruimen van mislukte backup gefaald", "path", fullPath, "error", removeErr)
		}
		slog.Error("Backup mislukt", "error", err, "output", string(output))
		return nil, &BackupError{Operation: "maken", Err: fmt.Errorf("%s", strings.TrimSpace(string(output)))}
	}

	// Get file info
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return nil, &BackupError{Operation: "maken", Err: fmt.Errorf("backup bestand niet gevonden na creatie: %w", err)}
	}

	// Set restrictive permissions
	if err := os.Chmod(fullPath, 0600); err != nil {
		slog.Warn("Kon bestandspermissies niet instellen", "file", fullPath, "error", err)
	}

	slog.Info("Backup succesvol gemaakt",
		"filename", filename,
		"size", formatBytes(fileInfo.Size()),
		"duration", duration.Round(time.Millisecond).String())

	// Cleanup old backups
	go s.cleanupOldBackups()

	return &BackupResult{
		Filename:      filename,
		FilePath:      fullPath,
		Format:        format,
		Size:          fileInfo.Size(),
		SizeFormatted: formatBytes(fileInfo.Size()),
		Duration:      duration.Round(time.Millisecond).String(),
		CreatedAt:     fileInfo.ModTime(),
		DryRun:        false,
	}, nil
}

// StreamBackup streams a backup directly to a writer (for download).
func (s *AeronService) StreamBackup(ctx context.Context, w io.Writer, format string, compression int) error {
	if !s.config.Backup.Enabled {
		return &ConfigurationError{Field: "backup.enabled", Message: "backup functionaliteit is niet ingeschakeld"}
	}

	// Check if pg_dump is available
	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		return &ConfigurationError{Field: "pg_dump", Message: "postgresql-client niet geïnstalleerd"}
	}

	// Validate format
	if format == "" {
		format = s.config.Backup.GetDefaultFormat()
	}
	if format != "custom" && format != "plain" {
		return &ValidationError{Field: "format", Message: fmt.Sprintf("ongeldig backup formaat: %s", format)}
	}

	if compression == 0 {
		compression = s.config.Backup.GetDefaultCompression()
	}

	// Build pg_dump arguments (output to stdout)
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

	cmd := exec.CommandContext(ctx, pgDumpPath, args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.config.Database.Password)
	cmd.Stdout = w

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &BackupError{Operation: "streamen", Err: fmt.Errorf("%s", strings.TrimSpace(stderr.String()))}
	}

	return nil
}

// ListBackups returns a list of available backup files.
func (s *AeronService) ListBackups() (*BackupListResponse, error) {
	if !s.config.Backup.Enabled {
		return nil, &ConfigurationError{Field: "backup.enabled", Message: "backup functionaliteit is niet ingeschakeld"}
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
		return nil, &ConfigurationError{Field: "backup.path", Message: fmt.Sprintf("backup directory niet leesbaar: %v", err)}
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

		// Determine format from extension
		var format string
		if strings.HasSuffix(name, ".dump") {
			format = "custom"
		} else if strings.HasSuffix(name, ".sql") {
			format = "plain"
		} else {
			continue
		}

		backups = append(backups, BackupInfo{
			Filename:      name,
			Format:        format,
			Size:          info.Size(),
			SizeFormatted: formatBytes(info.Size()),
			CreatedAt:     info.ModTime(),
		})
		totalSize += info.Size()
	}

	// Sort by creation time, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return &BackupListResponse{
		Backups:    backups,
		TotalSize:  totalSize,
		TotalCount: len(backups),
	}, nil
}

// DeleteBackup deletes a specific backup file.
func (s *AeronService) DeleteBackup(filename string) error {
	if !s.config.Backup.Enabled {
		return &ConfigurationError{Field: "backup.enabled", Message: "backup functionaliteit is niet ingeschakeld"}
	}

	// Validate filename to prevent path traversal
	if !validFilenameRegex.MatchString(filename) {
		return &ValidationError{Field: "filename", Message: "ongeldige bestandsnaam"}
	}

	if !strings.HasPrefix(filename, "aeron-backup-") {
		return &ValidationError{Field: "filename", Message: "geen geldig backup bestand"}
	}

	backupPath := s.config.Backup.GetPath()
	fullPath := filepath.Join(backupPath, filename)

	// Ensure the path is within the backup directory
	absBackupPath, err := filepath.Abs(backupPath)
	if err != nil {
		return &ConfigurationError{Field: "backup.path", Message: fmt.Sprintf("backup pad niet resolveerbaar: %v", err)}
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return &ValidationError{Field: "filename", Message: fmt.Sprintf("bestandspad niet resolveerbaar: %v", err)}
	}
	if !strings.HasPrefix(absFullPath, absBackupPath) {
		return &ValidationError{Field: "filename", Message: "ongeldige bestandsnaam"}
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return NewNotFoundError("backup", filename)
	}

	if err := os.Remove(fullPath); err != nil {
		return &BackupError{Operation: "verwijderen", Err: err}
	}

	slog.Info("Backup verwijderd", "filename", filename)
	return nil
}

// cleanupOldBackups removes backups that exceed retention policy.
func (s *AeronService) cleanupOldBackups() {
	backups, err := s.ListBackups()
	if err != nil {
		slog.Error("Kon backups niet ophalen voor cleanup", "error", err)
		return
	}

	maxAge := time.Duration(s.config.Backup.GetRetentionDays()) * 24 * time.Hour
	maxBackups := s.config.Backup.GetMaxBackups()
	cutoff := time.Now().Add(-maxAge)

	var deleted int

	// Delete backups older than retention period
	for _, backup := range backups.Backups {
		if backup.CreatedAt.Before(cutoff) {
			if err := s.DeleteBackup(backup.Filename); err == nil {
				deleted++
				slog.Info("Oude backup verwijderd (retention)", "filename", backup.Filename)
			}
		}
	}

	// If still too many backups, delete oldest ones
	backups, _ = s.ListBackups()
	if backups != nil && len(backups.Backups) > maxBackups {
		// Backups are sorted newest first, so we delete from the end
		for i := maxBackups; i < len(backups.Backups); i++ {
			if err := s.DeleteBackup(backups.Backups[i].Filename); err == nil {
				deleted++
				slog.Info("Oude backup verwijderd (max_backups)", "filename", backups.Backups[i].Filename)
			}
		}
	}

	if deleted > 0 {
		slog.Info("Backup cleanup voltooid", "deleted", deleted)
	}
}

// GetBackupFilePath returns the full path to a backup file if it exists.
func (s *AeronService) GetBackupFilePath(filename string) (string, error) {
	if !s.config.Backup.Enabled {
		return "", &ConfigurationError{Field: "backup.enabled", Message: "backup functionaliteit is niet ingeschakeld"}
	}

	// Validate filename
	if !validFilenameRegex.MatchString(filename) {
		return "", &ValidationError{Field: "filename", Message: "ongeldige bestandsnaam"}
	}

	if !strings.HasPrefix(filename, "aeron-backup-") {
		return "", &ValidationError{Field: "filename", Message: "geen geldig backup bestand"}
	}

	backupPath := s.config.Backup.GetPath()
	fullPath := filepath.Join(backupPath, filename)

	// Ensure the path is within the backup directory
	absBackupPath, err := filepath.Abs(backupPath)
	if err != nil {
		return "", &ConfigurationError{Field: "backup.path", Message: fmt.Sprintf("backup pad niet resolveerbaar: %v", err)}
	}
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", &ValidationError{Field: "filename", Message: fmt.Sprintf("bestandspad niet resolveerbaar: %v", err)}
	}
	if !strings.HasPrefix(absFullPath, absBackupPath) {
		return "", &ValidationError{Field: "filename", Message: "ongeldige bestandsnaam"}
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", NewNotFoundError("backup", filename)
	}

	return fullPath, nil
}
