// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"log/slog"
	"time"

	cron "github.com/netresearch/go-cron"
)

// BackupScheduler manages scheduled automatic backups.
type BackupScheduler struct {
	cron    *cron.Cron
	service *AeronService
}

// NewBackupScheduler creates a new backup scheduler.
func NewBackupScheduler(svc *AeronService) (*BackupScheduler, error) {
	cfg := svc.Config().Backup.Scheduler

	loc := time.Local
	if cfg.Timezone != "" {
		var err error
		if loc, err = time.LoadLocation(cfg.Timezone); err != nil {
			return nil, err
		}
	}

	c := cron.New(
		cron.WithLocation(loc),
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)

	s := &BackupScheduler{cron: c, service: svc}

	if _, err := c.AddFunc(cfg.Schedule, s.runBackup); err != nil {
		return nil, err
	}

	return s, nil
}

// Start begins the scheduler.
func (s *BackupScheduler) Start() {
	s.cron.Start()
	slog.Info("Backup scheduler gestart",
		"schedule", s.service.Config().Backup.Scheduler.Schedule,
		"next_run", s.cron.Entries()[0].Next.Format(time.RFC3339))
}

// Stop gracefully stops the scheduler and waits for running jobs to complete.
func (s *BackupScheduler) Stop() context.Context {
	slog.Info("Backup scheduler wordt gestopt...")
	return s.cron.Stop()
}

// runBackup executes the scheduled backup job.
func (s *BackupScheduler) runBackup() {
	start := time.Now()
	slog.Info("Geplande backup gestart")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cfg := s.service.Config().Backup
	result, err := s.service.Backup.Create(ctx, BackupRequest{
		Format:      cfg.GetDefaultFormat(),
		Compression: cfg.GetDefaultCompression(),
	})

	if err != nil {
		slog.Error("Geplande backup mislukt", "error", err, "duration", time.Since(start))
		return
	}

	slog.Info("Geplande backup voltooid",
		"filename", result.Filename,
		"size", result.SizeFormatted,
		"duration", time.Since(start))
}
