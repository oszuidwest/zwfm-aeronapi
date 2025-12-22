// Package config provides application configuration management.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
	"gopkg.in/yaml.v3"
)

// DatabaseConfig contains PostgreSQL database connection parameters.
// All fields are required for establishing a connection to the Aeron database.
type DatabaseConfig struct {
	Host            string `yaml:"host"`              // Database host address
	Port            string `yaml:"port"`              // Database port number
	Name            string `yaml:"name"`              // Database name
	User            string `yaml:"user"`              // Database username
	Password        string `yaml:"password"`          // Database password
	Schema          string `yaml:"schema"`            // Database schema name (typically "aeron")
	SSLMode         string `yaml:"sslmode"`           // SSL connection mode
	MaxOpenConns    int    `yaml:"max_open_conns"`    // Maximum number of open connections (default: 25)
	MaxIdleConns    int    `yaml:"max_idle_conns"`    // Maximum number of idle connections (default: 5)
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // Connection max lifetime in minutes (default: 5)
}

// ImageConfig contains image processing and optimization settings.
// It defines how uploaded images should be resized and compressed.
type ImageConfig struct {
	TargetWidth      int   `yaml:"target_width"`       // Target width for image resizing
	TargetHeight     int   `yaml:"target_height"`      // Target height for image resizing
	Quality          int   `yaml:"quality"`            // JPEG quality (1-100)
	RejectSmaller    bool  `yaml:"reject_smaller"`     // Whether to reject images smaller than target dimensions
	MaxDownloadBytes int64 `yaml:"max_download_bytes"` // Maximum download size in bytes (default: 50MB)
}

// APIConfig contains API authentication and server settings.
// When enabled, all API endpoints require a valid API key in the X-API-Key header.
type APIConfig struct {
	Enabled        bool     `yaml:"enabled"`         // Whether API key authentication is enabled
	Keys           []string `yaml:"keys"`            // List of valid API keys
	RequestTimeout int      `yaml:"request_timeout"` // Request timeout in seconds (default: 30)
}

// MaintenanceConfig contains thresholds for database maintenance recommendations.
type MaintenanceConfig struct {
	BloatThreshold     float64 `yaml:"bloat_threshold"`      // Bloat percentage threshold for vacuum recommendation (default: 10.0)
	DeadTupleThreshold int64   `yaml:"dead_tuple_threshold"` // Dead tuple count threshold for vacuum recommendation (default: 10000)
}

// BackupConfig contains settings for database backup functionality.
type BackupConfig struct {
	Enabled            bool   `yaml:"enabled"`             // Whether backup endpoints are enabled
	Path               string `yaml:"path"`                // Directory for storing backups
	RetentionDays      int    `yaml:"retention_days"`      // Auto-delete backups older than this (default: 30)
	MaxBackups         int    `yaml:"max_backups"`         // Maximum number of backups to keep (default: 10)
	DefaultFormat      string `yaml:"default_format"`      // Default backup format: "custom" or "plain" (default: "custom")
	DefaultCompression int    `yaml:"default_compression"` // Default compression level 0-9 (default: 9)
}

// Config represents the complete application configuration loaded from YAML.
// It contains all settings needed for database connectivity, image processing, and API authentication.
// The zero value is not usable; all fields must be properly configured.
type Config struct {
	Database    DatabaseConfig    `yaml:"database"`    // Database connection settings
	Image       ImageConfig       `yaml:"image"`       // Image processing settings
	API         APIConfig         `yaml:"api"`         // API authentication settings
	Maintenance MaintenanceConfig `yaml:"maintenance"` // Maintenance thresholds
	Backup      BackupConfig      `yaml:"backup"`      // Backup settings
}

// Default configuration values
const (
	DefaultMaxOpenConns        = 25
	DefaultMaxIdleConns        = 5
	DefaultConnMaxLifetime     = 5                // minutes
	DefaultMaxDownloadBytes    = 50 * 1024 * 1024 // 50MB
	DefaultRequestTimeout      = 30               // seconds
	DefaultBloatThreshold      = 10.0
	DefaultDeadTupleThreshold  = 10000
	DefaultBackupRetentionDays = 30
	DefaultBackupMaxBackups    = 10
	DefaultBackupFormat        = "custom"
	DefaultBackupCompression   = 9
	DefaultBackupPath          = "./backups"
)

// GetMaxDownloadBytes returns the maximum download size, using default if not configured.
func (c *ImageConfig) GetMaxDownloadBytes() int64 {
	if c.MaxDownloadBytes <= 0 {
		return DefaultMaxDownloadBytes
	}
	return c.MaxDownloadBytes
}

// GetRequestTimeout returns the request timeout duration.
func (c *APIConfig) GetRequestTimeout() time.Duration {
	if c.RequestTimeout <= 0 {
		return time.Duration(DefaultRequestTimeout) * time.Second
	}
	return time.Duration(c.RequestTimeout) * time.Second
}

// GetMaxOpenConns returns the max open connections, using default if not configured.
func (c *DatabaseConfig) GetMaxOpenConns() int {
	if c.MaxOpenConns <= 0 {
		return DefaultMaxOpenConns
	}
	return c.MaxOpenConns
}

// GetMaxIdleConns returns the max idle connections, using default if not configured.
func (c *DatabaseConfig) GetMaxIdleConns() int {
	if c.MaxIdleConns <= 0 {
		return DefaultMaxIdleConns
	}
	return c.MaxIdleConns
}

// GetConnMaxLifetime returns the connection max lifetime.
func (c *DatabaseConfig) GetConnMaxLifetime() time.Duration {
	if c.ConnMaxLifetime <= 0 {
		return time.Duration(DefaultConnMaxLifetime) * time.Minute
	}
	return time.Duration(c.ConnMaxLifetime) * time.Minute
}

// GetBloatThreshold returns the bloat percentage threshold for vacuum recommendations.
func (c *MaintenanceConfig) GetBloatThreshold() float64 {
	if c.BloatThreshold <= 0 {
		return DefaultBloatThreshold
	}
	return c.BloatThreshold
}

// GetDeadTupleThreshold returns the dead tuple count threshold for vacuum recommendations.
func (c *MaintenanceConfig) GetDeadTupleThreshold() int64 {
	if c.DeadTupleThreshold <= 0 {
		return DefaultDeadTupleThreshold
	}
	return c.DeadTupleThreshold
}

// GetPath returns the backup path, using default if not configured.
func (c *BackupConfig) GetPath() string {
	if c.Path == "" {
		return DefaultBackupPath
	}
	return c.Path
}

// GetRetentionDays returns the backup retention period in days.
func (c *BackupConfig) GetRetentionDays() int {
	if c.RetentionDays <= 0 {
		return DefaultBackupRetentionDays
	}
	return c.RetentionDays
}

// GetMaxBackups returns the maximum number of backups to keep.
func (c *BackupConfig) GetMaxBackups() int {
	if c.MaxBackups <= 0 {
		return DefaultBackupMaxBackups
	}
	return c.MaxBackups
}

// GetDefaultFormat returns the default backup format ("custom" or "plain").
func (c *BackupConfig) GetDefaultFormat() string {
	if c.DefaultFormat == "" {
		return DefaultBackupFormat
	}
	return c.DefaultFormat
}

// GetDefaultCompression returns the default compression level (0-9).
func (c *BackupConfig) GetDefaultCompression() int {
	if c.DefaultCompression <= 0 {
		return DefaultBackupCompression
	}
	if c.DefaultCompression > 9 {
		return 9
	}
	return c.DefaultCompression
}

// Load loads and validates application configuration from a YAML file.
// If configPath is empty, it attempts to load "config.yaml" from the current directory.
// Returns an error if the file cannot be read or contains invalid configuration.
func Load(configPath string) (*Config, error) {
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

	if err := validate(config); err != nil {
		return nil, fmt.Errorf("configuratie is onvolledig: %w", err)
	}

	return config, nil
}

// validate ensures that all required configuration fields are present and valid.
// It checks database connection parameters, image settings, and returns an error
// listing any missing or invalid configuration values.
func validate(config *Config) error {
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

	// Validate schema name for SQL safety
	if config.Database.Schema != "" && !types.IsValidIdentifier(config.Database.Schema) {
		return fmt.Errorf("database.schema bevat ongeldige tekens")
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

// ConnectionString returns a PostgreSQL connection string.
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}
