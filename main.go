// Package main implements the Aeron Toolbox API server.
//
// This server provides an unofficial REST API for the Aeron radio automation system.
// It offers image management, database browsing, database maintenance, and backup
// functionality through direct database access.
//
// The API server can be configured via JSON configuration file and supports
// optional API key authentication for secure access.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/api"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/config"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/service"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	configFile := flag.String("config", "", "Path to config file (default: config.json)")
	port := flag.String("port", "8080", "API server port (default: 8080)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		printVersion()
		return nil
	}

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuratiefout: %v\n", err)
		return err
	}

	initLogger()

	db, dbClose, err := setupDatabase(cfg)
	if err != nil {
		return err
	}
	defer dbClose()

	svc, err := service.New(db, cfg)
	if err != nil {
		slog.Error("Service initialisatie mislukt", "error", err)
		return err
	}
	defer svc.Close()

	scheduler := startSchedulerIfEnabled(cfg, svc)

	server := api.New(svc, Version)

	return serveUntilShutdown(server, *port, scheduler)
}

func printVersion() {
	fmt.Printf("Aeron Toolbox %s (%s)\n", Version, Commit)
	fmt.Printf("Build time: %s\n", BuildTime)
}

func initLogger() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
}

func setupDatabase(cfg *config.Config) (*sqlx.DB, func(), error) {
	db, err := sqlx.Open("postgres", cfg.Database.ConnectionString())
	if err != nil {
		slog.Error("Database verbinding mislukt", "error", err)
		return nil, nil, err
	}

	db.SetMaxOpenConns(cfg.Database.GetMaxOpenConns())
	db.SetMaxIdleConns(cfg.Database.GetMaxIdleConns())
	db.SetConnMaxLifetime(cfg.Database.GetConnMaxLifetime())

	slog.Info("Database connection pool geconfigureerd",
		"max_open", cfg.Database.GetMaxOpenConns(),
		"max_idle", cfg.Database.GetMaxIdleConns(),
		"max_lifetime", cfg.Database.GetConnMaxLifetime())

	if err := db.Ping(); err != nil {
		slog.Error("Database ping mislukt", "error", err)
		_ = db.Close()
		return nil, nil, err
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			slog.Error("Fout bij sluiten database", "error", err)
		}
	}

	return db, cleanup, nil
}

func startSchedulerIfEnabled(cfg *config.Config, svc *service.AeronService) *service.BackupScheduler {
	if !cfg.Backup.Enabled || !cfg.Backup.Scheduler.Enabled {
		return nil
	}

	scheduler, err := service.NewBackupScheduler(svc)
	if err != nil {
		slog.Error("Backup scheduler initialisatie mislukt", "error", err)
		return nil
	}

	scheduler.Start()
	return scheduler
}

func serveUntilShutdown(server *api.Server, port string, scheduler *service.BackupScheduler) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("API-server gestart op poort", "poort", port)
		if err := server.Start(port); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case <-stop:
		slog.Info("Shutdown signaal ontvangen, server wordt gestopt...")
	case err := <-serverErr:
		slog.Error("API server fout", "error", err)
		return err
	}

	return gracefulShutdown(server, scheduler)
}

func gracefulShutdown(server *api.Server, scheduler *service.BackupScheduler) error {
	if scheduler != nil {
		ctx := scheduler.Stop()
		select {
		case <-ctx.Done():
			slog.Info("Backup scheduler succesvol gestopt")
		case <-time.After(35 * time.Second):
			slog.Warn("Backup scheduler stop timeout, forceer afsluiten")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Fout bij graceful shutdown", "error", err)
		return err
	}

	slog.Info("Server succesvol gestopt")
	return nil
}
