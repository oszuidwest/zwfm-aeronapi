package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	var (
		configFile = flag.String("config", "", "Path to config file (default: config.yaml)")
		serverPort = flag.String("port", "8080", "API server port (default: 8080)")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *version {
		fmt.Printf("Aeron Image Manager %s (%s)\n", Version, Commit)
		fmt.Printf("Build time: %s\n", BuildTime)
		return
	}

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuratiefout: %v\n", err)
		os.Exit(1)
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
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("Fout bij sluiten database", "error", err)
		}
	}()

	if err := db.Ping(); err != nil {
		slog.Error("Database ping mislukt", "error", err)
		os.Exit(1)
	}

	// Database connected successfully

	// Create service and start API server
	service := NewAeronService(db, config)
	apiServer := NewAeronAPI(service, config)

	// Start API server
	slog.Info("API-server gestart op poort", "poort", *serverPort)

	if err := apiServer.Start(*serverPort); err != nil {
		slog.Error("API server gestopt met fout", "error", err)
		os.Exit(1)
	}
}
