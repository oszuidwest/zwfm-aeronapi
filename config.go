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
	RejectSmaller bool `yaml:"reject_smaller"`
}

type APIConfig struct {
	Enabled bool     `yaml:"enabled"`
	Keys    []string `yaml:"keys"`
}

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Image    ImageConfig    `yaml:"image"`
	API      APIConfig      `yaml:"api"`
}

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
