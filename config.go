package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Schema   string `yaml:"schema"`
	SSLMode  string `yaml:"sslmode"`
}

type ImageConfig struct {
	TargetWidth   int  `yaml:"target_width"`
	TargetHeight  int  `yaml:"target_height"`
	Quality       int  `yaml:"quality"`
	MaxFileSizeMB int  `yaml:"max_file_size_mb"`
	RejectSmaller bool `yaml:"reject_smaller"`
}

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Image    ImageConfig    `yaml:"image"`
}

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.Database.User, c.Database.Password, c.Database.Host, c.Database.Port, c.Database.Name, c.Database.SSLMode)
}

func loadConfig(configPath string) (*Config, error) {
	// Geen defaults - alles moet in config file staan
	config := &Config{}

	// Config bestand is verplicht
	if configPath == "" {
		// Zoek naar config.yaml in huidige directory
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		} else {
			return nil, fmt.Errorf("geen config bestand gevonden: config.yaml niet aanwezig en geen --config opgegeven")
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("kon config bestand niet lezen: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("kon config bestand niet parsen: %w", err)
	}

	// Valideer database configuratie
	if err := validateDatabaseConfig(&config.Database); err != nil {
		return nil, fmt.Errorf("database configuratie onvolledig: %w", err)
	}

	// Valideer image configuratie
	if err := validateImageConfig(&config.Image); err != nil {
		return nil, fmt.Errorf("image configuratie onvolledig: %w", err)
	}

	return config, nil
}

func validateDatabaseConfig(db *DatabaseConfig) error {
	missing := []string{}

	if db.Host == "" {
		missing = append(missing, "host")
	}
	if db.Port == "" {
		missing = append(missing, "port")
	}
	if db.Name == "" {
		missing = append(missing, "name")
	}
	if db.User == "" {
		missing = append(missing, "user")
	}
	if db.Password == "" {
		missing = append(missing, "password")
	}
	if db.Schema == "" {
		missing = append(missing, "schema")
	}
	if db.SSLMode == "" {
		missing = append(missing, "sslmode")
	}

	if len(missing) > 0 {
		return fmt.Errorf("de volgende database velden ontbreken in de configuratie: %v", missing)
	}

	return nil
}

func validateImageConfig(img *ImageConfig) error {
	missing := []string{}

	if img.TargetWidth <= 0 {
		missing = append(missing, "target_width")
	}
	if img.TargetHeight <= 0 {
		missing = append(missing, "target_height")
	}
	if img.Quality <= 0 || img.Quality > 100 {
		missing = append(missing, "quality (must be 1-100)")
	}
	if img.MaxFileSizeMB <= 0 {
		missing = append(missing, "max_file_size_mb")
	}

	if len(missing) > 0 {
		return fmt.Errorf("de volgende image velden ontbreken of zijn ongeldig in de configuratie: %v", missing)
	}

	return nil
}
