// Package config provides application configuration management.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
)

// DatabaseConfig contains PostgreSQL database connection parameters.
// All fields are required for establishing a connection to the Aeron database.
type DatabaseConfig struct {
	Host                   string `json:"host"`                      // Database host address
	Port                   string `json:"port"`                      // Database port number
	Name                   string `json:"name"`                      // Database name
	User                   string `json:"user"`                      // Database username
	Password               string `json:"password"`                  // Database password
	Schema                 string `json:"schema"`                    // Database schema name (typically "aeron")
	SSLMode                string `json:"sslmode"`                   // SSL connection mode
	MaxOpenConns           int    `json:"max_open_conns"`            // Maximum number of open connections (default: 25)
	MaxIdleConns           int    `json:"max_idle_conns"`            // Maximum number of idle connections (default: 5)
	ConnMaxLifetimeMinutes int    `json:"conn_max_lifetime_minutes"` // Connection max lifetime in minutes (default: 5)
}

// ImageConfig contains image processing and optimization settings.
// It defines how uploaded images should be resized and compressed.
type ImageConfig struct {
	TargetWidth               int   `json:"target_width"`                   // Target width for image resizing
	TargetHeight              int   `json:"target_height"`                  // Target height for image resizing
	Quality                   int   `json:"quality"`                        // JPEG quality (1-100)
	RejectSmaller             bool  `json:"reject_smaller"`                 // Whether to reject images smaller than target dimensions
	MaxImageDownloadSizeBytes int64 `json:"max_image_download_size_bytes"`  // Maximum download size in bytes (default: 50MB)
}

// APIConfig contains API authentication and server settings.
// When enabled, all API endpoints require a valid API key in the X-API-Key header.
type APIConfig struct {
	Enabled               bool     `json:"enabled"`                  // Whether API key authentication is enabled
	Keys                  []string `json:"keys"`                     // List of valid API keys
	RequestTimeoutSeconds int      `json:"request_timeout_seconds"`  // Request timeout in seconds (default: 30)
}

// MaintenanceConfig contains thresholds for database maintenance recommendations.
type MaintenanceConfig struct {
	BloatThreshold     float64 `json:"bloat_threshold"`      // Bloat percentage threshold for vacuum recommendation (default: 10.0)
	DeadTupleThreshold int64   `json:"dead_tuple_threshold"` // Dead tuple count threshold for vacuum recommendation (default: 10000)
}

// BackupConfig contains settings for database backup functionality.
type BackupConfig struct {
	Enabled            bool   `json:"enabled"`             // Whether backup endpoints are enabled
	Path               string `json:"path"`                // Directory for storing backups
	RetentionDays      int    `json:"retention_days"`      // Auto-delete backups older than this (default: 30)
	MaxBackups         int    `json:"max_backups"`         // Maximum number of backups to keep (default: 10)
	DefaultFormat      string `json:"default_format"`      // Default backup format: "custom" or "plain" (default: "custom")
	DefaultCompression int    `json:"default_compression"` // Default compression level 0-9 (default: 9)
}

// Config represents the complete application configuration loaded from JSON.
// It contains all settings needed for database connectivity, image processing, and API authentication.
// The zero value is not usable; all fields must be properly configured.
type Config struct {
	Database    DatabaseConfig    `json:"database"`    // Database connection settings
	Image       ImageConfig       `json:"image"`       // Image processing settings
	API         APIConfig         `json:"api"`         // API authentication settings
	Maintenance MaintenanceConfig `json:"maintenance"` // Maintenance thresholds
	Backup      BackupConfig      `json:"backup"`      // Backup settings
}

// Default configuration values
const (
	DefaultMaxOpenConnections             = 25
	DefaultMaxIdleConnections             = 5
	DefaultConnMaxLifetimeMinutes         = 5                // minutes
	DefaultMaxImageDownloadSizeBytes      = 50 * 1024 * 1024 // 50MB
	DefaultRequestTimeoutSeconds          = 30               // seconds
	DefaultBloatThreshold                 = 10.0
	DefaultDeadTupleThreshold             = 10000
	DefaultBackupRetentionDays            = 30
	DefaultBackupMaxBackups               = 10
	DefaultBackupFormat                   = "custom"
	DefaultBackupCompression              = 9
	DefaultBackupPath                     = "./backups"
)

// GetMaxDownloadBytes returns the maximum download size, using default if not configured.
func (c *ImageConfig) GetMaxDownloadBytes() int64 {
	if c.MaxImageDownloadSizeBytes <= 0 {
		return DefaultMaxImageDownloadSizeBytes
	}
	return c.MaxImageDownloadSizeBytes
}

// GetRequestTimeout returns the request timeout duration.
func (c *APIConfig) GetRequestTimeout() time.Duration {
	if c.RequestTimeoutSeconds <= 0 {
		return time.Duration(DefaultRequestTimeoutSeconds) * time.Second
	}
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

// GetMaxOpenConns returns the max open connections, using default if not configured.
func (c *DatabaseConfig) GetMaxOpenConns() int {
	if c.MaxOpenConns <= 0 {
		return DefaultMaxOpenConnections
	}
	return c.MaxOpenConns
}

// GetMaxIdleConns returns the max idle connections, using default if not configured.
func (c *DatabaseConfig) GetMaxIdleConns() int {
	if c.MaxIdleConns <= 0 {
		return DefaultMaxIdleConnections
	}
	return c.MaxIdleConns
}

// GetConnMaxLifetime returns the connection max lifetime.
func (c *DatabaseConfig) GetConnMaxLifetime() time.Duration {
	if c.ConnMaxLifetimeMinutes <= 0 {
		return time.Duration(DefaultConnMaxLifetimeMinutes) * time.Minute
	}
	return time.Duration(c.ConnMaxLifetimeMinutes) * time.Minute
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

// Load loads and validates application configuration from a JSON file.
// If configPath is empty, it attempts to load "config.json" from the current directory.
// Returns an error if the file cannot be read or contains invalid configuration.
func Load(configPath string) (*Config, error) {
	config := &Config{}

	if configPath == "" {
		if _, err := os.Stat("config.json"); err == nil {
			configPath = "config.json"
		} else {
			return nil, fmt.Errorf("configuratiebestand config.json niet gevonden")
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("lezen van configuratiebestand mislukt: %w", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
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
