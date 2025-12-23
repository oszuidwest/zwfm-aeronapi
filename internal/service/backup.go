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
	"sync/atomic"
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
	s3         *s3Service // nil if S3 is disabled
	done       chan struct{}
	wg         sync.WaitGroup
	running    atomic.Bool

	// Paths to required external tools, resolved at startup
	pgDumpPath    string
	pgRestorePath string

	statusMu   sync.RWMutex
	lastStatus *BackupStatus
}

// BackupStatus represents the status of the last backup operation.
type BackupStatus struct {
	Running   bool          `json:"running"`
	StartedAt *time.Time    `json:"started_at,omitempty"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Filename  string        `json:"filename,omitempty"`
	S3Sync    *S3SyncStatus `json:"s3_sync,omitempty"`
}

// S3SyncStatus represents the status of S3 synchronization.
type S3SyncStatus struct {
	Synced bool   `json:"synced"`
	Error  string `json:"error,omitempty"`
}

// newBackupService returns a BackupService for database backup operations.
func newBackupService(repo *database.Repository, cfg *config.Config) (*BackupService, error) {
	svc := &BackupService{
		repo:   repo,
		config: cfg,
		done:   make(chan struct{}),
	}

	if cfg.Backup.Enabled {
		// Resolve required external tools at startup
		pgDumpPath, err := resolveToolPath(cfg.Backup.PgDumpPath, "pg_dump")
		if err != nil {
			return nil, err
		}
		svc.pgDumpPath = pgDumpPath

		pgRestorePath, err := resolveToolPath(cfg.Backup.PgRestorePath, "pg_restore")
		if err != nil {
			return nil, err
		}
		svc.pgRestorePath = pgRestorePath

		backupPath := cfg.Backup.GetPath()
		if err := os.MkdirAll(backupPath, 0o750); err != nil {
			return nil, types.NewConfigError("backup.path", fmt.Sprintf("backup directory niet toegankelijk: %v", err))
		}

		root, err := os.OpenRoot(backupPath)
		if err != nil {
			return nil, types.NewConfigError("backup.path", fmt.Sprintf("backup directory niet te openen: %v", err))
		}
		svc.backupRoot = root

		// Initialize S3 backend if configured
		s3svc, err := newS3Service(&cfg.Backup.S3)
		if err != nil {
			return nil, err
		}
		svc.s3 = s3svc
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

// resolveToolPath resolves the path to an external tool.
// If customPath is provided, it validates and returns that path.
// Otherwise, it searches for the tool in PATH.
func resolveToolPath(customPath, toolName string) (string, error) {
	if customPath != "" {
		if _, err := os.Stat(customPath); err != nil {
			return "", types.NewConfigError(toolName, fmt.Sprintf("%s niet gevonden op pad: %s", toolName, customPath))
		}
		return customPath, nil
	}

	path, err := exec.LookPath(toolName)
	if err != nil {
		return "", types.NewConfigError(toolName, fmt.Sprintf("%s niet gevonden in PATH", toolName))
	}
	return path, nil
}

func (s *BackupService) checkEnabled() error {
	if !s.config.Backup.Enabled || s.backupRoot == nil {
		return types.NewConfigError("backup.enabled", "backup functionaliteit is niet ingeschakeld")
	}
	return nil
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

// Start starts a database backup in the background.
// Returns immediately after validation. Check GET /backups for status.
// Returns error if a backup is already running.
func (s *BackupService) Start(req BackupRequest) error {
	if err := s.checkEnabled(); err != nil {
		return err
	}
	if _, _, err := s.validateRequest(req); err != nil {
		return err
	}

	if !s.running.CompareAndSwap(false, true) {
		return types.NewOperationError("backup starten", errors.New("backup is al bezig"))
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.running.Store(false)

		ctx, cancel := context.WithTimeout(context.Background(), s.config.Backup.GetTimeout())
		defer cancel()

		_ = s.execute(ctx, req) // Error tracked in status
	}()

	return nil
}

// Run executes a backup synchronously. Used by scheduler.
// Returns error if a backup is already running.
func (s *BackupService) Run(ctx context.Context, req BackupRequest) error {
	if !s.running.CompareAndSwap(false, true) {
		return types.NewOperationError("backup starten", errors.New("backup is al bezig"))
	}
	defer s.running.Store(false)

	return s.execute(ctx, req)
}

// execute is the internal backup implementation.
func (s *BackupService) execute(ctx context.Context, req BackupRequest) error {
	// Record start immediately so status is always consistent
	s.setStatusStarted()

	if err := s.checkEnabled(); err != nil {
		s.setStatusDone(false, "", err.Error())
		return err
	}

	format, compression, err := s.validateRequest(req)
	if err != nil {
		s.setStatusDone(false, "", err.Error())
		return err
	}

	filename := generateBackupFilename(format)
	fullPath := filepath.Join(s.config.Backup.GetPath(), filename)
	args := s.buildPgDumpArgs(format, compression)

	args = append(args, "--file="+fullPath)

	s.setStatusFilename(filename)
	slog.Info("Backup gestart", "filename", filename, "format", format)

	fileInfo, duration, err := s.executePgDump(ctx, s.pgDumpPath, filename, fullPath, args)
	if err != nil {
		s.setStatusDone(false, filename, err.Error())
		return err
	}

	// Validate backup file integrity
	slog.Info("Backup valideren", "filename", filename, "format", format)

	validateCtx, validateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer validateCancel()

	if err := validateBackup(validateCtx, fullPath, format, s.pgRestorePath); err != nil {
		slog.Error("Backup validatie mislukt", "filename", filename, "error", err)
		s.setStatusDone(false, filename, err.Error())
		return err
	}

	slog.Info("Backup gevalideerd", "filename", filename)

	s.setStatusDone(true, filename, "")
	slog.Info("Backup voltooid",
		"filename", filename,
		"size", util.FormatBytes(fileInfo.Size()),
		"duration", duration.Round(time.Millisecond).String())

	// Sync to S3 in background (non-blocking)
	if s.s3 != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			uploadCtx, cancel := context.WithTimeout(context.Background(), s.config.Backup.GetTimeout())
			defer cancel()

			if err := s.s3.upload(uploadCtx, filename, fullPath); err != nil {
				slog.Error("S3 synchronisatie mislukt", "filename", filename, "error", err)
				s.setS3SyncStatus(false, err.Error())
			} else {
				s.setS3SyncStatus(true, "")
			}
		}()
	}

	s.cleanupOldBackups()
	return nil
}

// Status returns the status of the last backup operation.
// Running state always comes from the atomic bool, not stored state.
func (s *BackupService) Status() *BackupStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()

	if s.lastStatus == nil {
		return &BackupStatus{Running: s.running.Load()}
	}

	status := *s.lastStatus
	status.Running = s.running.Load()
	return &status
}

// setStatusStarted records that a backup has started.
// Called at the very beginning of execute() to ensure StartedAt is always set.
func (s *BackupService) setStatusStarted() {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()

	now := time.Now()
	s.lastStatus = &BackupStatus{
		StartedAt: &now,
	}
}

// setStatusFilename updates the filename once it's known.
func (s *BackupService) setStatusFilename(filename string) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()

	if s.lastStatus != nil {
		s.lastStatus.Filename = filename
	}
}

// setStatusDone records that a backup has completed (success or failure).
func (s *BackupService) setStatusDone(success bool, filename, errMsg string) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()

	now := time.Now()
	if s.lastStatus == nil {
		s.lastStatus = &BackupStatus{StartedAt: &now}
	}
	s.lastStatus.EndedAt = &now
	s.lastStatus.Success = success
	s.lastStatus.Error = errMsg
	if filename != "" {
		s.lastStatus.Filename = filename
	}
}

// setS3SyncStatus records the result of S3 synchronization.
func (s *BackupService) setS3SyncStatus(synced bool, errMsg string) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()

	if s.lastStatus != nil {
		s.lastStatus.S3Sync = &S3SyncStatus{
			Synced: synced,
			Error:  errMsg,
		}
	}
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

		format := detectBackupFormat(name)
		if format == "" {
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

	// Also delete from S3 in background (non-blocking)
	if s.s3 != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := s.s3.delete(ctx, filename); err != nil {
				slog.Warn("S3 backup verwijderen mislukt", "filename", filename, "error", err)
			}
		}()
	}

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

// ValidationResult represents the result of on-demand backup validation.
type ValidationResult struct {
	Filename string `json:"filename"`
	Format   string `json:"format"`
	Valid    bool   `json:"valid"`
	Error    string `json:"error,omitempty"`
}

// Validate validates an existing backup file and returns the result.
func (s *BackupService) Validate(filename string) (*ValidationResult, error) {
	fullPath, err := s.GetFilePath(filename)
	if err != nil {
		return nil, err
	}

	format := detectBackupFormat(filename)
	result := &ValidationResult{
		Filename: filename,
		Format:   format,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := validateBackup(ctx, fullPath, format, s.pgRestorePath); err != nil {
		result.Valid = false
		result.Error = err.Error()
	} else {
		result.Valid = true
	}

	return result, nil
}

func detectBackupFormat(filename string) string {
	switch {
	case strings.HasSuffix(filename, ".dump"):
		return "custom"
	case strings.HasSuffix(filename, ".sql"):
		return "plain"
	default:
		return ""
	}
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
