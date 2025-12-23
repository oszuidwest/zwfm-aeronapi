// Package config provides application configuration management.
package config

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
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

// LogConfig contains logging configuration.
type LogConfig struct {
	Level  string `json:"level"`  // "debug", "info", "warn", "error"
	Format string `json:"format"` // "text", "json"
}

// Config represents the complete application configuration.
type Config struct {
	Database    DatabaseConfig    `json:"database"`
	Image       ImageConfig       `json:"image"`
	API         APIConfig         `json:"api"`
	Maintenance MaintenanceConfig `json:"maintenance"`
	Backup      BackupConfig      `json:"backup"`
	Log         LogConfig         `json:"log"`
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

// GetMaxDownloadBytes returns the maximum allowed image download size in bytes.
func (c *ImageConfig) GetMaxDownloadBytes() int64 {
	return cmp.Or(c.MaxImageDownloadSizeBytes, DefaultMaxImageDownloadSizeBytes)
}

// GetRequestTimeout returns the HTTP request timeout as a Duration.
func (c *APIConfig) GetRequestTimeout() time.Duration {
	return time.Duration(cmp.Or(c.RequestTimeoutSeconds, DefaultRequestTimeoutSeconds)) * time.Second
}

// GetMaxOpenConns returns the maximum number of open database connections.
func (c *DatabaseConfig) GetMaxOpenConns() int {
	return cmp.Or(c.MaxOpenConns, DefaultMaxOpenConnections)
}

// GetMaxIdleConns returns the maximum number of idle database connections.
func (c *DatabaseConfig) GetMaxIdleConns() int {
	return cmp.Or(c.MaxIdleConns, DefaultMaxIdleConnections)
}

// GetConnMaxLifetime returns the maximum lifetime of database connections as a Duration.
func (c *DatabaseConfig) GetConnMaxLifetime() time.Duration {
	return time.Duration(cmp.Or(c.ConnMaxLifetimeMinutes, DefaultConnMaxLifetimeMinutes)) * time.Minute
}

// GetBloatThreshold returns the table bloat percentage that triggers maintenance recommendations.
func (c *MaintenanceConfig) GetBloatThreshold() float64 {
	return cmp.Or(c.BloatThreshold, DefaultBloatThreshold)
}

// GetDeadTupleThreshold returns the dead tuple count that triggers vacuum recommendations.
func (c *MaintenanceConfig) GetDeadTupleThreshold() int64 {
	return cmp.Or(c.DeadTupleThreshold, DefaultDeadTupleThreshold)
}

// GetPath returns the directory path where backup files are stored.
func (c *BackupConfig) GetPath() string {
	return cmp.Or(c.Path, DefaultBackupPath)
}

// GetRetentionDays returns the number of days to keep backup files before automatic deletion.
func (c *BackupConfig) GetRetentionDays() int {
	return cmp.Or(c.RetentionDays, DefaultBackupRetentionDays)
}

// GetMaxBackups returns the maximum number of backup files to retain.
func (c *BackupConfig) GetMaxBackups() int {
	return cmp.Or(c.MaxBackups, DefaultBackupMaxBackups)
}

// GetDefaultFormat returns the pg_dump format ("custom" or "plain").
func (c *BackupConfig) GetDefaultFormat() string {
	return cmp.Or(c.DefaultFormat, DefaultBackupFormat)
}

// GetDefaultCompression returns the compression level (0-9) for custom format backups.
func (c *BackupConfig) GetDefaultCompression() int {
	return min(cmp.Or(c.DefaultCompression, DefaultBackupCompression), 9)
}

// GetLevel returns the configured log level.
func (c *LogConfig) GetLevel() slog.Level {
	switch strings.ToLower(c.Level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// GetFormat returns the configured log format ("text" or "json").
func (c *LogConfig) GetFormat() string {
	if strings.EqualFold(c.Format, "json") {
		return "json"
	}
	return "text"
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

	// Environment variable overrides
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		config.Log.Level = envLevel
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
