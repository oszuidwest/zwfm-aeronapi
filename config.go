package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DatabaseConfig contains PostgreSQL database connection parameters.
// All fields are required for establishing a connection to the Aeron database.
type DatabaseConfig struct {
	Host     string `yaml:"host"`     // Database host address
	Port     string `yaml:"port"`     // Database port number
	Name     string `yaml:"name"`     // Database name
	User     string `yaml:"user"`     // Database username
	Password string `yaml:"password"` // Database password
	Schema   string `yaml:"schema"`   // Database schema name (typically "aeron")
	SSLMode  string `yaml:"sslmode"`  // SSL connection mode
}

// ImageConfig contains image processing and optimization settings.
// It defines how uploaded images should be resized and compressed.
type ImageConfig struct {
	TargetWidth   int  `yaml:"target_width"`   // Target width for image resizing
	TargetHeight  int  `yaml:"target_height"`  // Target height for image resizing
	Quality       int  `yaml:"quality"`        // JPEG quality (1-100)
	RejectSmaller bool `yaml:"reject_smaller"` // Whether to reject images smaller than target dimensions
}

// APIConfig contains API authentication settings.
// When enabled, all API endpoints require a valid API key in the X-API-Key header.
type APIConfig struct {
	Enabled bool     `yaml:"enabled"` // Whether API key authentication is enabled
	Keys    []string `yaml:"keys"`    // List of valid API keys
}

// Config represents the complete application configuration loaded from YAML.
// It contains all settings needed for database connectivity, image processing, and API authentication.
// The zero value is not usable; all fields must be properly configured.
type Config struct {
	Database DatabaseConfig `yaml:"database"` // Database connection settings
	Image    ImageConfig    `yaml:"image"`    // Image processing settings
	API      APIConfig      `yaml:"api"`      // API authentication settings
}

// loadConfig loads and validates application configuration from a YAML file.
// If configPath is empty, it attempts to load "config.yaml" from the current directory.
// Returns an error if the file cannot be read or contains invalid configuration.
func loadConfig(configPath string) (*Config, error) {
	config := &Config{}

	if configPath == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		} else {
			return nil, fmt.Errorf("configuratiebestand config.yaml niet gevonden")
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("lezen van configuratiebestand mislukt: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("fout in configuratiebestand: %w", err)
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuratie is onvolledig: %w", err)
	}

	return config, nil
}

// validateConfig ensures that all required configuration fields are present and valid.
// It checks database connection parameters, image settings, and returns an error
// listing any missing or invalid configuration values.
func validateConfig(config *Config) error {
	var missing []string

	// Database validation
	if config.Database.Host == "" {
		missing = append(missing, "database.host")
	}
	if config.Database.Port == "" {
		missing = append(missing, "database.port")
	}
	if config.Database.Name == "" {
		missing = append(missing, "database.name")
	}
	if config.Database.User == "" {
		missing = append(missing, "database.user")
	}
	if config.Database.Password == "" {
		missing = append(missing, "database.password")
	}
	if config.Database.Schema == "" {
		missing = append(missing, "database.schema")
	}
	if config.Database.SSLMode == "" {
		missing = append(missing, "database.sslmode")
	}

	// Image validation
	if config.Image.TargetWidth <= 0 {
		missing = append(missing, "image.target_width")
	}
	if config.Image.TargetHeight <= 0 {
		missing = append(missing, "image.target_height")
	}
	if config.Image.Quality <= 0 || config.Image.Quality > 100 {
		missing = append(missing, "image.quality (1-100)")
	}

	if len(missing) > 0 {
		return fmt.Errorf("configuratie mist de volgende velden: %v", missing)
	}
	return nil
}
