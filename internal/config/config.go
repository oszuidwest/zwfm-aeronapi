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

// MaintenanceConfig contains thresholds and settings for database maintenance operations.
type MaintenanceConfig struct {
	BloatThreshold           float64         `json:"bloat_threshold"`
	DeadTupleThreshold       int64           `json:"dead_tuple_threshold"`
	VacuumStalenessDays      int             `json:"vacuum_staleness_days"`
	MinRowsForRecommendation int64           `json:"min_rows_for_recommendation"`
	ToastSizeWarningBytes    int64           `json:"toast_size_warning_bytes"`
	StaleStatsThresholdPct   int             `json:"stale_stats_threshold_pct"`
	SeqScanRatioThreshold    float64         `json:"seq_scan_ratio_threshold"`
	TimeoutMinutes           int             `json:"timeout_minutes"`
	Scheduler                SchedulerConfig `json:"scheduler"`
}

// SchedulerConfig contains settings for automatic scheduled backups.
type SchedulerConfig struct {
	Enabled  bool   `json:"enabled"`
	Schedule string `json:"schedule"` // Cron expression, e.g., "0 3 * * *"
	Timezone string `json:"timezone"` // Optional IANA timezone, e.g., "Europe/Amsterdam"
}

// S3Config contains settings for S3-compatible storage synchronization.
type S3Config struct {
	Enabled         bool   `json:"enabled"`
	Bucket          string `json:"bucket"`
	Region          string `json:"region"`
	Endpoint        string `json:"endpoint"` // Custom endpoint for S3-compatible services (MinIO, Backblaze B2, etc.)
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	PathPrefix      string `json:"path_prefix"`      // Prefix for S3 keys, e.g., "backups/"
	ForcePathStyle  bool   `json:"force_path_style"` // Use path-style URLs (required for MinIO)
}

// BackupConfig contains settings for database backup functionality.
type BackupConfig struct {
	Enabled            bool            `json:"enabled"`
	Path               string          `json:"path"`
	RetentionDays      int             `json:"retention_days"`
	MaxBackups         int             `json:"max_backups"`
	DefaultCompression int             `json:"default_compression"`
	TimeoutMinutes     int             `json:"timeout_minutes"`
	PgDumpPath         string          `json:"pg_dump_path"`    // Custom path to pg_dump, empty = auto-detect
	PgRestorePath      string          `json:"pg_restore_path"` // Custom path to pg_restore, empty = auto-detect
	Scheduler          SchedulerConfig `json:"scheduler"`
	S3                 S3Config        `json:"s3"`
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
	DefaultVacuumStalenessDays       = 7
	DefaultMinRowsForRecommendation  = 1000
	DefaultToastSizeWarningBytes     = 500 * 1024 * 1024
	DefaultStaleStatsThresholdPct    = 10
	DefaultSeqScanRatioThreshold     = 10.0
	DefaultMaintenanceTimeoutMinutes = 30
	DefaultBackupRetentionDays       = 30
	DefaultBackupMaxBackups          = 10
	DefaultBackupCompression         = 9
	DefaultBackupPath                = "./backups"
	DefaultBackupTimeoutMinutes      = 30
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

// GetVacuumStalenessDays returns the number of days after which a table is considered stale.
func (c *MaintenanceConfig) GetVacuumStalenessDays() int {
	return cmp.Or(c.VacuumStalenessDays, DefaultVacuumStalenessDays)
}

// GetVacuumStaleness returns the staleness threshold as a Duration.
func (c *MaintenanceConfig) GetVacuumStaleness() time.Duration {
	return time.Duration(c.GetVacuumStalenessDays()) * 24 * time.Hour
}

// GetMinRowsForRecommendation returns the minimum row count for maintenance recommendations.
func (c *MaintenanceConfig) GetMinRowsForRecommendation() int64 {
	return cmp.Or(c.MinRowsForRecommendation, DefaultMinRowsForRecommendation)
}

// GetToastSizeWarningBytes returns the TOAST size threshold for warnings.
func (c *MaintenanceConfig) GetToastSizeWarningBytes() int64 {
	return cmp.Or(c.ToastSizeWarningBytes, DefaultToastSizeWarningBytes)
}

// GetStaleStatsThreshold returns the percentage of modified rows that triggers ANALYZE.
func (c *MaintenanceConfig) GetStaleStatsThreshold() int {
	return cmp.Or(c.StaleStatsThresholdPct, DefaultStaleStatsThresholdPct)
}

// GetSeqScanRatioThreshold returns the seq_scan/idx_scan ratio for missing index warnings.
func (c *MaintenanceConfig) GetSeqScanRatioThreshold() float64 {
	return cmp.Or(c.SeqScanRatioThreshold, DefaultSeqScanRatioThreshold)
}

// GetTimeout returns the maximum duration for maintenance operations.
func (c *MaintenanceConfig) GetTimeout() time.Duration {
	return time.Duration(cmp.Or(c.TimeoutMinutes, DefaultMaintenanceTimeoutMinutes)) * time.Minute
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

// GetDefaultCompression returns the compression level (0-9) for backups.
func (c *BackupConfig) GetDefaultCompression() int {
	return min(cmp.Or(c.DefaultCompression, DefaultBackupCompression), 9)
}

// GetTimeout returns the maximum duration for backup operations.
func (c *BackupConfig) GetTimeout() time.Duration {
	return time.Duration(cmp.Or(c.TimeoutMinutes, DefaultBackupTimeoutMinutes)) * time.Minute
}

// GetPathPrefix returns the S3 path prefix with a trailing slash for key construction.
func (c *S3Config) GetPathPrefix() string {
	prefix := c.PathPrefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

// GetLevel returns the configured log level, defaulting to Info for unrecognized values.
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

// validator accumulates validation errors with a field prefix for clear error messages.
type validator struct {
	prefix string
	errs   []error
}

func newValidator(prefix string) *validator {
	return &validator{prefix: prefix}
}

func (v *validator) required(value, field string) {
	if value == "" {
		v.errs = append(v.errs, fmt.Errorf("%s.%s is verplicht", v.prefix, field))
	}
}

func (v *validator) positive(value int, field string) {
	if value <= 0 {
		v.errs = append(v.errs, fmt.Errorf("%s.%s moet groter dan 0 zijn", v.prefix, field))
	}
}

func (v *validator) inRange(value, minVal, maxVal int, field string) {
	if value < minVal || value > maxVal {
		v.errs = append(v.errs, fmt.Errorf("%s.%s moet tussen %d en %d zijn", v.prefix, field, minVal, maxVal))
	}
}

func (v *validator) timezone(tz, field string) {
	if tz == "" {
		return
	}
	if _, err := time.LoadLocation(tz); err != nil {
		v.errs = append(v.errs, fmt.Errorf("%s.%s is ongeldig: %s", v.prefix, field, tz))
	}
}

func (v *validator) identifier(value, field string) {
	if value != "" && !types.IsValidIdentifier(value) {
		v.errs = append(v.errs, fmt.Errorf("%s.%s bevat ongeldige tekens", v.prefix, field))
	}
}

func (v *validator) errors() []error {
	return v.errs
}

// validate checks the configuration for required fields and valid values.
func validate(config *Config) error {
	var errs []error
	errs = append(errs, validateDatabase(&config.Database)...)
	errs = append(errs, validateImage(&config.Image)...)
	errs = append(errs, validateScheduler(&config.Maintenance.Scheduler, "maintenance.scheduler")...)
	errs = append(errs, validateBackup(&config.Backup)...)
	return errors.Join(errs...)
}

func validateDatabase(cfg *DatabaseConfig) []error {
	v := newValidator("database")
	v.required(cfg.Host, "host")
	v.required(cfg.Port, "port")
	v.required(cfg.Name, "name")
	v.required(cfg.User, "user")
	v.required(cfg.Password, "password")
	v.required(cfg.Schema, "schema")
	v.required(cfg.SSLMode, "sslmode")
	v.identifier(cfg.Schema, "schema")
	return v.errors()
}

func validateImage(cfg *ImageConfig) []error {
	v := newValidator("image")
	v.positive(cfg.TargetWidth, "target_width")
	v.positive(cfg.TargetHeight, "target_height")
	v.inRange(cfg.Quality, 1, 100, "quality")
	return v.errors()
}

func validateScheduler(cfg *SchedulerConfig, prefix string) []error {
	v := newValidator(prefix)
	if cfg.Enabled && cfg.Schedule == "" {
		v.required("", "schedule") // triggers error
	}
	v.timezone(cfg.Timezone, "timezone")
	return v.errors()
}

func validateBackup(cfg *BackupConfig) []error {
	v := newValidator("backup")
	if cfg.TimeoutMinutes < 0 {
		v.errs = append(v.errs, fmt.Errorf("backup.timeout_minutes mag niet negatief zijn"))
	}
	v.errs = append(v.errs, validateScheduler(&cfg.Scheduler, "backup.scheduler")...)
	v.errs = append(v.errs, validateS3(&cfg.S3)...)
	return v.errors()
}

func validateS3(cfg *S3Config) []error {
	if !cfg.Enabled {
		return nil
	}
	v := newValidator("backup.s3")
	v.required(cfg.Bucket, "bucket")
	v.required(cfg.AccessKeyID, "access_key_id")
	v.required(cfg.SecretAccessKey, "secret_access_key")
	if cfg.Region == "" && cfg.Endpoint == "" {
		v.errs = append(v.errs, fmt.Errorf("backup.s3.region is verplicht wanneer geen endpoint is opgegeven"))
	}
	return v.errors()
}

// ConnectionString returns a PostgreSQL connection string.
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}
