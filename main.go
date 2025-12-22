// Package main implements the Aeron Image Manager API server.
//
// This server provides an unofficial REST API for managing images in the Aeron
// radio automation system. It allows adding and managing album covers for tracks
// and photos for artists directly in the Aeron database, functionality that is
// not natively supported by the system.
//
// The API server can be configured via YAML configuration file and supports
// optional API key authentication for secure access.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Version is the build version string, set at build time via ldflags.
// It defaults to "dev" for development builds.
var Version = "dev"

// Commit is the git commit hash, set at build time via ldflags.
// It defaults to "unknown" when not built from version control.
var Commit = "unknown"

// BuildTime is the build timestamp, set at build time via ldflags.
// It defaults to "unknown" when build time is not captured.
var BuildTime = "unknown"

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	var (
		configFile = flag.String("config", "", "Path to config file (default: config.yaml)")
		serverPort = flag.String("port", "8080", "API server port (default: 8080)")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *version {
		fmt.Printf("Aeron Image Manager %s (%s)\n", Version, Commit)
		fmt.Printf("Build time: %s\n", BuildTime)
		return nil
	}

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuratiefout: %v\n", err)
		return err
	}

	// Initialize simple logger to stdout
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// Connect to database
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		config.Database.User, config.Database.Password, config.Database.Host,
		config.Database.Port, config.Database.Name, config.Database.SSLMode)
	db, err := sqlx.Open("postgres", dbURL)
	if err != nil {
		slog.Error("Database verbinding mislukt", "error", err)
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("Fout bij sluiten database", "error", err)
		}
	}()

	if err := db.Ping(); err != nil {
		slog.Error("Database ping mislukt", "error", err)
		return err
	}

	// Create service and start API server
	service := NewAeronService(db, config)
	apiServer := NewAeronAPI(service, config)

	// Start API server
	slog.Info("API-server gestart op poort", "poort", *serverPort)

	if err := apiServer.Start(*serverPort); err != nil {
		slog.Error("API server gestopt met fout", "error", err)
		return err
	}

	return nil
}
