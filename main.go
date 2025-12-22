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
	var (
		configFile = flag.String("config", "", "Path to config file (default: config.json)")
		serverPort = flag.String("port", "8080", "API server port (default: 8080)")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *version {
		fmt.Printf("Aeron Toolbox %s (%s)\n", Version, Commit)
		fmt.Printf("Build time: %s\n", BuildTime)
		return nil
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuratiefout: %v\n", err)
		return err
	}

	// Initialize simple logger to stdout
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// Connect to database
	db, err := connectDatabase(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("Fout bij sluiten database", "error", err)
		}
	}()

	// Create service and API server
	svc, err := service.New(db, cfg)
	if err != nil {
		slog.Error("Service initialisatie mislukt", "error", err)
		return err
	}

	// Create and start backup scheduler if enabled
	var scheduler *service.BackupScheduler
	if cfg.Backup.Enabled && cfg.Backup.Scheduler.Enabled {
		scheduler, err = service.NewBackupScheduler(svc)
		if err != nil {
			slog.Error("Backup scheduler initialisatie mislukt", "error", err)
			return err
		}
		scheduler.Start()
	}

	api.Version = Version
	apiServer := api.New(svc, cfg)

	// Setup signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start API server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("API-server gestart op poort", "poort", *serverPort)
		if err := apiServer.Start(*serverPort); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case <-stop:
		slog.Info("Shutdown signaal ontvangen, server wordt gestopt...")
	case err := <-serverErr:
		slog.Error("API server fout", "error", err)
		return err
	}

	// Stop scheduler first (before database closes)
	if scheduler != nil {
		ctx := scheduler.Stop()
		select {
		case <-ctx.Done():
			slog.Info("Backup scheduler succesvol gestopt")
		case <-time.After(35 * time.Second):
			slog.Warn("Backup scheduler stop timeout, forceer afsluiten")
		}
	}

	// Graceful shutdown API server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := apiServer.Shutdown(ctx); err != nil {
		slog.Error("Fout bij graceful shutdown", "error", err)
		return err
	}

	slog.Info("Server succesvol gestopt")
	return nil
}

// connectDatabase establishes a connection to the PostgreSQL database with configured pool settings.
func connectDatabase(cfg *config.Config) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", cfg.Database.ConnectionString())
	if err != nil {
		slog.Error("Database verbinding mislukt", "error", err)
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.Database.GetMaxOpenConns())
	db.SetMaxIdleConns(cfg.Database.GetMaxIdleConns())
	db.SetConnMaxLifetime(cfg.Database.GetConnMaxLifetime())

	slog.Info("Database connection pool geconfigureerd",
		"max_open", cfg.Database.GetMaxOpenConns(),
		"max_idle", cfg.Database.GetMaxIdleConns(),
		"max_lifetime", cfg.Database.GetConnMaxLifetime())

	if err := db.Ping(); err != nil {
		slog.Error("Database ping mislukt", "error", err)
		return nil, err
	}

	return db, nil
}
