// Package config provides application configuration management.
package config

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
)

// DatabaseConfig contains PostgreSQL database connection parameters.
type DatabaseConfig struct {
	Host                   string `json:"host"`
	Port                   string `json:"port"`
	Name                   string `json:"name"`
	User                   string `json:"user"`
	Password               string `json:"password"`
	Schema                 string `json:"schema"`
	SSLMode                string `json:"sslmode"`
	MaxOpenConns           int    `json:"max_open_conns"`
	MaxIdleConns           int    `json:"max_idle_conns"`
	ConnMaxLifetimeMinutes int    `json:"conn_max_lifetime_minutes"`
}

// ImageConfig contains image processing and optimization settings.
type ImageConfig struct {
	TargetWidth               int   `json:"target_width"`
	TargetHeight              int   `json:"target_height"`
	Quality                   int   `json:"quality"`
	RejectSmaller             bool  `json:"reject_smaller"`
	MaxImageDownloadSizeBytes int64 `json:"max_image_download_size_bytes"`
}

// APIConfig contains API authentication and server settings.
type APIConfig struct {
	Enabled               bool     `json:"enabled"`
	Keys                  []string `json:"keys"`
	RequestTimeoutSeconds int      `json:"request_timeout_seconds"`
}

// MaintenanceConfig contains thresholds for database maintenance recommendations.
type MaintenanceConfig struct {
	BloatThreshold     float64 `json:"bloat_threshold"`
	DeadTupleThreshold int64   `json:"dead_tuple_threshold"`
}

// SchedulerConfig contains settings for automatic scheduled backups.
type SchedulerConfig struct {
	Enabled  bool   `json:"enabled"`
	Schedule string `json:"schedule"` // Cron expression, e.g., "0 3 * * *"
	Timezone string `json:"timezone"` // Optional IANA timezone, e.g., "Europe/Amsterdam"
}

// BackupConfig contains settings for database backup functionality.
type BackupConfig struct {
	Enabled            bool            `json:"enabled"`
	Path               string          `json:"path"`
	RetentionDays      int             `json:"retention_days"`
	MaxBackups         int             `json:"max_backups"`
	DefaultFormat      string          `json:"default_format"`
	DefaultCompression int             `json:"default_compression"`
	Scheduler          SchedulerConfig `json:"scheduler"`
}

// Config represents the complete application configuration loaded from JSON.
type Config struct {
	Database    DatabaseConfig    `json:"database"`
	Image       ImageConfig       `json:"image"`
	API         APIConfig         `json:"api"`
	Maintenance MaintenanceConfig `json:"maintenance"`
	Backup      BackupConfig      `json:"backup"`
}

const (
	DefaultMaxOpenConnections        = 25
	DefaultMaxIdleConnections        = 5
	DefaultConnMaxLifetimeMinutes    = 5
	DefaultMaxImageDownloadSizeBytes = 50 * 1024 * 1024
	DefaultRequestTimeoutSeconds     = 30
	DefaultBloatThreshold            = 10.0
	DefaultDeadTupleThreshold        = 10000
	DefaultBackupRetentionDays       = 30
	DefaultBackupMaxBackups          = 10
	DefaultBackupFormat              = "custom"
	DefaultBackupCompression         = 9
	DefaultBackupPath                = "./backups"
)

// GetMaxDownloadBytes returns the maximum download size.
func (c *ImageConfig) GetMaxDownloadBytes() int64 {
	return cmp.Or(c.MaxImageDownloadSizeBytes, DefaultMaxImageDownloadSizeBytes)
}

// GetRequestTimeout returns the request timeout duration.
func (c *APIConfig) GetRequestTimeout() time.Duration {
	return time.Duration(cmp.Or(c.RequestTimeoutSeconds, DefaultRequestTimeoutSeconds)) * time.Second
}

// GetMaxOpenConns returns the max open connections.
func (c *DatabaseConfig) GetMaxOpenConns() int {
	return cmp.Or(c.MaxOpenConns, DefaultMaxOpenConnections)
}

// GetMaxIdleConns returns the max idle connections.
func (c *DatabaseConfig) GetMaxIdleConns() int {
	return cmp.Or(c.MaxIdleConns, DefaultMaxIdleConnections)
}

// GetConnMaxLifetime returns the connection max lifetime.
func (c *DatabaseConfig) GetConnMaxLifetime() time.Duration {
	return time.Duration(cmp.Or(c.ConnMaxLifetimeMinutes, DefaultConnMaxLifetimeMinutes)) * time.Minute
}

// GetBloatThreshold returns the bloat percentage threshold.
func (c *MaintenanceConfig) GetBloatThreshold() float64 {
	return cmp.Or(c.BloatThreshold, DefaultBloatThreshold)
}

// GetDeadTupleThreshold returns the dead tuple count threshold.
func (c *MaintenanceConfig) GetDeadTupleThreshold() int64 {
	return cmp.Or(c.DeadTupleThreshold, DefaultDeadTupleThreshold)
}

// GetPath returns the backup path.
func (c *BackupConfig) GetPath() string {
	return cmp.Or(c.Path, DefaultBackupPath)
}

// GetRetentionDays returns the backup retention period.
func (c *BackupConfig) GetRetentionDays() int {
	return cmp.Or(c.RetentionDays, DefaultBackupRetentionDays)
}

// GetMaxBackups returns the maximum number of backups.
func (c *BackupConfig) GetMaxBackups() int {
	return cmp.Or(c.MaxBackups, DefaultBackupMaxBackups)
}

// GetDefaultFormat returns the default backup format.
func (c *BackupConfig) GetDefaultFormat() string {
	return cmp.Or(c.DefaultFormat, DefaultBackupFormat)
}

// GetDefaultCompression returns the default compression level.
func (c *BackupConfig) GetDefaultCompression() int {
	return min(cmp.Or(c.DefaultCompression, DefaultBackupCompression), 9)
}

// Load loads and validates application configuration from a JSON file.
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

func validate(config *Config) error {
	var errs []error

	requiredStrings := []struct {
		value string
		field string
	}{
		{config.Database.Host, "database.host"},
		{config.Database.Port, "database.port"},
		{config.Database.Name, "database.name"},
		{config.Database.User, "database.user"},
		{config.Database.Password, "database.password"},
		{config.Database.Schema, "database.schema"},
		{config.Database.SSLMode, "database.sslmode"},
	}

	for _, req := range requiredStrings {
		if req.value == "" {
			errs = append(errs, fmt.Errorf("%s is verplicht", req.field))
		}
	}

	if config.Database.Schema != "" && !types.IsValidIdentifier(config.Database.Schema) {
		errs = append(errs, fmt.Errorf("database.schema bevat ongeldige tekens"))
	}

	if config.Image.TargetWidth <= 0 {
		errs = append(errs, fmt.Errorf("image.target_width is verplicht"))
	}
	if config.Image.TargetHeight <= 0 {
		errs = append(errs, fmt.Errorf("image.target_height is verplicht"))
	}
	if config.Image.Quality <= 0 || config.Image.Quality > 100 {
		errs = append(errs, fmt.Errorf("image.quality moet tussen 1-100 zijn"))
	}

	// Validate scheduler config
	if config.Backup.Scheduler.Enabled {
		if config.Backup.Scheduler.Schedule == "" {
			errs = append(errs, fmt.Errorf("backup.scheduler.schedule is verplicht wanneer scheduler is ingeschakeld"))
		}
	}
	if config.Backup.Scheduler.Timezone != "" {
		if _, err := time.LoadLocation(config.Backup.Scheduler.Timezone); err != nil {
			errs = append(errs, fmt.Errorf("backup.scheduler.timezone is ongeldig: %s", config.Backup.Scheduler.Timezone))
		}
	}

	return errors.Join(errs...)
}

// ConnectionString returns a PostgreSQL connection string.
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}
