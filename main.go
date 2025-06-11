package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// ANSI color codes
const (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Green = "\033[32m"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Constants
const (
	ScopeArtist = "artist"
	ScopeTrack  = "track"
	Kilobyte    = 1024
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
		log.Fatal(err)
	}

	// Connect to database
	db, err := sql.Open("postgres", config.DatabaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	// Create service and start API server
	service := NewImageService(db, config)
	apiServer := NewAeronAPI(service, config)

	fmt.Printf("%sAeron Image Manager%s\n", Bold, Reset)
	fmt.Printf("Version: %s (%s)\n", Version, Commit)
	fmt.Printf("Database: %s\n", config.Database.Name)
	fmt.Printf("API Authentication: %s\n", func() string {
		if config.API.Enabled {
			return fmt.Sprintf("Enabled (%d keys)", len(config.API.Keys))
		}
		return "Disabled"
	}())
	fmt.Printf("\n%sStarting API server on port %s...%s\n", Green, *serverPort, Reset)

	log.Fatal(apiServer.Start(*serverPort))
}
