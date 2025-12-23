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
	done       chan struct{}
	wg         sync.WaitGroup
	running    atomic.Bool

	statusMu   sync.RWMutex
	lastStatus *BackupStatus
}

// BackupStatus represents the status of the last backup operation.
type BackupStatus struct {
	Running   bool       `json:"running"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Success   bool       `json:"success"`
	Error     string     `json:"error,omitempty"`
	Filename  string     `json:"filename,omitempty"`
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

// Start starts a database backup in the background.
// Returns immediately after validation. Check GET /backups for status.
// Returns error if a backup is already running.
func (s *BackupService) Start(req BackupRequest) error {
	if err := s.checkEnabled(); err != nil {
		return err
	}
	if _, err := findPgDump(); err != nil {
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

	pgDumpPath, err := findPgDump()
	if err != nil {
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

	fileInfo, duration, err := s.executePgDump(ctx, pgDumpPath, filename, fullPath, args)
	if err != nil {
		s.setStatusDone(false, filename, err.Error())
		return err
	}

	s.setStatusDone(true, filename, "")
	slog.Info("Backup voltooid",
		"filename", filename,
		"size", util.FormatBytes(fileInfo.Size()),
		"duration", duration.Round(time.Millisecond).String())

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
