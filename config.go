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

	// Valideer configuratie
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuratie onvolledig: %w", err)
	}

	return config, nil
}

func validateConfig(config *Config) error {
	validators := []struct {
		name  string
		check func() bool
	}{
		// Database fields
		{"database.host", func() bool { return config.Database.Host != "" }},
		{"database.port", func() bool { return config.Database.Port != "" }},
		{"database.name", func() bool { return config.Database.Name != "" }},
		{"database.user", func() bool { return config.Database.User != "" }},
		{"database.password", func() bool { return config.Database.Password != "" }},
		{"database.schema", func() bool { return config.Database.Schema != "" }},
		{"database.sslmode", func() bool { return config.Database.SSLMode != "" }},
		// Image fields
		{"image.target_width", func() bool { return config.Image.TargetWidth > 0 }},
		{"image.target_height", func() bool { return config.Image.TargetHeight > 0 }},
		{"image.quality (1-100)", func() bool { return config.Image.Quality > 0 && config.Image.Quality <= 100 }},
		{"image.max_file_size_mb", func() bool { return config.Image.MaxFileSizeMB > 0 }},
	}

	var missing []string
	for _, v := range validators {
		if !v.check() {
			missing = append(missing, v.name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("de volgende velden ontbreken of zijn ongeldig in de configuratie: %v", missing)
	}
	return nil
}
