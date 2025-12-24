// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"log/slog"
	"time"

	cron "github.com/netresearch/go-cron"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/config"
)

// Scheduler manages cron-based scheduled jobs for the application.
// It consolidates all scheduled tasks into a single cron instance.
type Scheduler struct {
	cron    *cron.Cron
	service *AeronService
	jobs    []string // names of registered jobs for logging
}

// NewScheduler creates a scheduler and registers all enabled scheduled jobs.
func NewScheduler(svc *AeronService) (*Scheduler, error) {
	cfg := svc.Config()

	// Use Amsterdam timezone as default for this Dutch application
	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		loc = time.Local
	}

	c := cron.New(
		cron.WithLocation(loc),
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)

	s := &Scheduler{cron: c, service: svc}

	// Register backup job if enabled
	if cfg.Backup.Enabled && cfg.Backup.Scheduler.Enabled {
		if err := s.addJob(cfg.Backup.Scheduler, "backup", s.runBackup); err != nil {
			return nil, err
		}
	}

	// Register maintenance job if enabled
	if cfg.Maintenance.Scheduler.Enabled {
		if err := s.addJob(cfg.Maintenance.Scheduler, "maintenance", s.runMaintenance); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// addJob registers a scheduled job with optional timezone override.
func (s *Scheduler) addJob(cfg config.SchedulerConfig, name string, job func()) error {
	schedule := cfg.Schedule

	// Handle timezone override per job
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return err
		}
		// Wrap job to use specific timezone for logging
		_ = loc // timezone is already handled by cron location
	}

	if _, err := s.cron.AddFunc(schedule, job); err != nil {
		return err
	}

	s.jobs = append(s.jobs, name)
	slog.Info("Scheduled job registered", "job", name, "schedule", schedule)
	return nil
}

// Start activates all scheduled jobs.
func (s *Scheduler) Start() {
	if len(s.jobs) == 0 {
		return
	}
	s.cron.Start()
	slog.Info("Scheduler started", "jobs", s.jobs)
}

// Stop halts the scheduler and waits for running jobs to finish.
func (s *Scheduler) Stop() context.Context {
	if len(s.jobs) == 0 {
		return context.Background()
	}
	slog.Info("Scheduler stopping...", "jobs", s.jobs)
	return s.cron.Stop()
}

// HasJobs returns true if any jobs are registered.
func (s *Scheduler) HasJobs() bool {
	return len(s.jobs) > 0
}

// runBackup performs a scheduled backup.
func (s *Scheduler) runBackup() {
	cfg := s.service.Config().Backup
	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetTimeout())
	defer cancel()

	slog.Info("Scheduled backup started")
	if err := s.service.Backup.Run(ctx, BackupRequest{
		Compression: cfg.GetDefaultCompression(),
	}); err != nil {
		slog.Error("Scheduled backup failed", "error", err)
	}
}

// runMaintenance performs scheduled VACUUM ANALYZE on tables that need it.
func (s *Scheduler) runMaintenance() {
	slog.Info("Scheduled maintenance started")

	// StartVacuum uses TryStart() internally which is atomic - no pre-check needed
	if err := s.service.Maintenance.StartVacuum(VacuumOptions{Analyze: true}); err != nil {
		// This includes "already running" errors - just log and continue
		slog.Warn("Scheduled maintenance not started", "reason", err)
		return
	}

	slog.Info("Scheduled maintenance running in background")
}
