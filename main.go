package main

import (
	"flag"
	"fmt"
	"log"

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
		log.Fatal(err)
	}

	// Connect to database
	db, err := sqlx.Open("postgres", config.DatabaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	// Create service and start API server
	service := NewAeronService(db, config)
	apiServer := NewAeronAPI(service, config)

	fmt.Println("Aeron Image Manager")
	fmt.Printf("Version: %s (%s)\n", Version, Commit)
	fmt.Printf("Database: %s\n", config.Database.Name)
	fmt.Printf("API Authentication: %s\n", func() string {
		if config.API.Enabled {
			return fmt.Sprintf("Enabled (%d keys)", len(config.API.Keys))
		}
		return "Disabled"
	}())
	fmt.Printf("\nStarting API server on port %s...\n", *serverPort)

	log.Fatal(apiServer.Start(*serverPort))
}
