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

func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.Database.User, c.Database.Password, c.Database.Host, c.Database.Port, c.Database.Name, c.Database.SSLMode)
}

func loadConfig(configPath string) (*Config, error) {
	config := &Config{}

	if configPath == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		} else {
			return nil, fmt.Errorf("config.yaml niet gevonden")
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("config lezen mislukt: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("config fout: %w", err)
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config onvolledig: %w", err)
	}

	return config, nil
}

func validateConfig(config *Config) error {
	validators := []struct {
		name  string
		check func() bool
	}{
		{"database.host", func() bool { return config.Database.Host != "" }},
		{"database.port", func() bool { return config.Database.Port != "" }},
		{"database.name", func() bool { return config.Database.Name != "" }},
		{"database.user", func() bool { return config.Database.User != "" }},
		{"database.password", func() bool { return config.Database.Password != "" }},
		{"database.schema", func() bool { return config.Database.Schema != "" }},
		{"database.sslmode", func() bool { return config.Database.SSLMode != "" }},
		{"image.target_width", func() bool { return config.Image.TargetWidth > 0 }},
		{"image.target_height", func() bool { return config.Image.TargetHeight > 0 }},
		{"image.quality (1-100)", func() bool { return config.Image.Quality > 0 && config.Image.Quality <= 100 }},
	}

	var missing []string
	for _, v := range validators {
		if !v.check() {
			missing = append(missing, v.name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("config mist velden: %v", missing)
	}
	return nil
}
