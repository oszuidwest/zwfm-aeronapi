// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"log/slog"
	"time"

	cron "github.com/netresearch/go-cron"
)

// BackupScheduler executes database backups on a configured cron schedule.
type BackupScheduler struct {
	cron    *cron.Cron
	service *AeronService
}

// NewBackupScheduler creates a scheduler with the configured cron schedule and timezone.
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

// Start activates the backup scheduler to run on its configured schedule.
func (s *BackupScheduler) Start() {
	s.cron.Start()
	slog.Info("Backup scheduler gestart",
		"schedule", s.service.Config().Backup.Scheduler.Schedule,
		"next_run", s.cron.Entries()[0].Next.Format(time.RFC3339))
}

// Stop halts the scheduler and waits for any running backup to finish.
func (s *BackupScheduler) Stop() context.Context {
	slog.Info("Backup scheduler wordt gestopt...")
	return s.cron.Stop()
}

// runBackup performs a backup with default settings from configuration.
func (s *BackupScheduler) runBackup() {
	cfg := s.service.Config().Backup
	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetTimeout())
	defer cancel()

	// Run() handles all logging internally
	_ = s.service.Backup.Run(ctx, BackupRequest{
		Compression: cfg.GetDefaultCompression(),
	})
}
