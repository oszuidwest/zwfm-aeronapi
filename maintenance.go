package main

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// DatabaseHealth represents the overall health status of the database.
type DatabaseHealth struct {
	DatabaseName    string        `json:"database_name"`
	DatabaseSize    string        `json:"database_size"`
	DatabaseSizeRaw int64         `json:"database_size_bytes"`
	SchemaName      string        `json:"schema_name"`
	Tables          []TableHealth `json:"tables"`
	Recommendations []string      `json:"recommendations"`
	CheckedAt       time.Time     `json:"checked_at"`
}

// TableHealth represents health statistics for a single table.
type TableHealth struct {
	Name            string     `json:"name"`
	RowCount        int64      `json:"row_count"`
	DeadTuples      int64      `json:"dead_tuples"`
	TotalSize       string     `json:"total_size"`
	TotalSizeRaw    int64      `json:"total_size_bytes"`
	TableSize       string     `json:"table_size"`
	TableSizeRaw    int64      `json:"table_size_bytes"`
	IndexSize       string     `json:"index_size"`
	IndexSizeRaw    int64      `json:"index_size_bytes"`
	ToastSize       string     `json:"toast_size"`
	ToastSizeRaw    int64      `json:"toast_size_bytes"`
	LastVacuum      *time.Time `json:"last_vacuum"`
	LastAutovacuum  *time.Time `json:"last_autovacuum"`
	LastAnalyze     *time.Time `json:"last_analyze"`
	LastAutoanalyze *time.Time `json:"last_autoanalyze"`
	BloatPercent    float64    `json:"bloat_percent"`
	SeqScans        int64      `json:"seq_scans"`
	IdxScans        int64      `json:"idx_scans"`
}

// tableStatsRow represents a row from pg_stat_user_tables.
type tableStatsRow struct {
	TableName       string     `db:"table_name"`
	LiveTuples      int64      `db:"live_tuples"`
	DeadTuples      int64      `db:"dead_tuples"`
	LastVacuum      *time.Time `db:"last_vacuum"`
	LastAutovacuum  *time.Time `db:"last_autovacuum"`
	LastAnalyze     *time.Time `db:"last_analyze"`
	LastAutoanalyze *time.Time `db:"last_autoanalyze"`
	SeqScan         int64      `db:"seq_scan"`
	IdxScan         int64      `db:"idx_scan"`
}

// tableSizeRow represents size information for a table.
type tableSizeRow struct {
	TableName string `db:"table_name"`
	TotalSize int64  `db:"total_size"`
	TableSize int64  `db:"table_size"`
	IndexSize int64  `db:"index_size"`
	ToastSize int64  `db:"toast_size"`
}

// GetDatabaseHealth retrieves comprehensive database health information.
func (s *AeronService) GetDatabaseHealth() (*DatabaseHealth, error) {
	health := &DatabaseHealth{
		DatabaseName: s.config.Database.Name,
		SchemaName:   s.config.Database.Schema,
		CheckedAt:    time.Now(),
	}

	// Get database size
	dbSize, dbSizeRaw, err := getDatabaseSize(s.db)
	if err != nil {
		return nil, fmt.Errorf("ophalen database grootte mislukt: %w", err)
	}
	health.DatabaseSize = dbSize
	health.DatabaseSizeRaw = dbSizeRaw

	// Get table statistics
	tables, err := getTableHealth(s.db, s.config.Database.Schema)
	if err != nil {
		return nil, fmt.Errorf("ophalen tabel statistieken mislukt: %w", err)
	}
	health.Tables = tables

	// Generate recommendations
	health.Recommendations = generateRecommendations(tables)

	return health, nil
}

// getDatabaseSize returns the total database size.
func getDatabaseSize(db *sqlx.DB) (string, int64, error) {
	var sizeRaw int64
	err := db.Get(&sizeRaw, "SELECT pg_database_size(current_database())")
	if err != nil {
		return "", 0, err
	}

	var sizePretty string
	err = db.Get(&sizePretty, "SELECT pg_size_pretty(pg_database_size(current_database()))")
	if err != nil {
		return "", 0, err
	}

	return sizePretty, sizeRaw, nil
}

// getTableHealth retrieves health statistics for all tables in the schema.
func getTableHealth(db *sqlx.DB, schema string) ([]TableHealth, error) {
	// Get table statistics from pg_stat_user_tables
	statsQuery := `
		SELECT
			relname as table_name,
			COALESCE(n_live_tup, 0) as live_tuples,
			COALESCE(n_dead_tup, 0) as dead_tuples,
			last_vacuum,
			last_autovacuum,
			last_analyze,
			last_autoanalyze,
			COALESCE(seq_scan, 0) as seq_scan,
			COALESCE(idx_scan, 0) as idx_scan
		FROM pg_stat_user_tables
		WHERE schemaname = $1
		ORDER BY n_live_tup DESC
	`

	var stats []tableStatsRow
	if err := db.Select(&stats, statsQuery, schema); err != nil {
		return nil, fmt.Errorf("ophalen tabel statistieken mislukt: %w", err)
	}

	// Get table sizes
	sizeQuery := `
		SELECT
			c.relname as table_name,
			pg_total_relation_size(c.oid) as total_size,
			pg_table_size(c.oid) as table_size,
			pg_indexes_size(c.oid) as index_size,
			COALESCE(pg_total_relation_size(c.reltoastrelid), 0) as toast_size
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relkind = 'r'
		ORDER BY pg_total_relation_size(c.oid) DESC
	`

	var sizes []tableSizeRow
	if err := db.Select(&sizes, sizeQuery, schema); err != nil {
		return nil, fmt.Errorf("ophalen tabel groottes mislukt: %w", err)
	}

	// Create a map for easy lookup
	sizeMap := make(map[string]tableSizeRow)
	for _, s := range sizes {
		sizeMap[s.TableName] = s
	}

	// Combine stats and sizes
	tables := make([]TableHealth, 0, len(stats))
	for _, stat := range stats {
		size, hasSize := sizeMap[stat.TableName]

		table := TableHealth{
			Name:            stat.TableName,
			RowCount:        stat.LiveTuples,
			DeadTuples:      stat.DeadTuples,
			LastVacuum:      stat.LastVacuum,
			LastAutovacuum:  stat.LastAutovacuum,
			LastAnalyze:     stat.LastAnalyze,
			LastAutoanalyze: stat.LastAutoanalyze,
			SeqScans:        stat.SeqScan,
			IdxScans:        stat.IdxScan,
		}

		if hasSize {
			table.TotalSizeRaw = size.TotalSize
			table.TotalSize = formatBytes(size.TotalSize)
			table.TableSizeRaw = size.TableSize
			table.TableSize = formatBytes(size.TableSize)
			table.IndexSizeRaw = size.IndexSize
			table.IndexSize = formatBytes(size.IndexSize)
			table.ToastSizeRaw = size.ToastSize
			table.ToastSize = formatBytes(size.ToastSize)
		}

		// Calculate bloat estimate based on dead tuples
		if stat.LiveTuples > 0 {
			table.BloatPercent = float64(stat.DeadTuples) / float64(stat.LiveTuples+stat.DeadTuples) * 100
		}

		tables = append(tables, table)
	}

	return tables, nil
}

// generateRecommendations creates actionable recommendations based on table health.
func generateRecommendations(tables []TableHealth) []string {
	var recommendations []string

	for _, t := range tables {
		// High bloat warning
		if t.BloatPercent > 10 {
			recommendations = append(recommendations,
				fmt.Sprintf("Tabel '%s' heeft %.1f%% bloat - VACUUM aanbevolen", t.Name, t.BloatPercent))
		}

		// Many dead tuples
		if t.DeadTuples > 10000 {
			recommendations = append(recommendations,
				fmt.Sprintf("Tabel '%s' heeft %d dead tuples - VACUUM aanbevolen", t.Name, t.DeadTuples))
		}

		// Never vacuumed
		if t.LastVacuum == nil && t.LastAutovacuum == nil && t.RowCount > 1000 {
			recommendations = append(recommendations,
				fmt.Sprintf("Tabel '%s' is nog nooit gevacuumd", t.Name))
		}

		// Old vacuum (more than 7 days)
		lastVac := t.LastAutovacuum
		if t.LastVacuum != nil && (lastVac == nil || t.LastVacuum.After(*lastVac)) {
			lastVac = t.LastVacuum
		}
		if lastVac != nil && time.Since(*lastVac) > 7*24*time.Hour && t.RowCount > 1000 {
			recommendations = append(recommendations,
				fmt.Sprintf("Tabel '%s' is meer dan 7 dagen niet gevacuumd", t.Name))
		}

		// Never analyzed
		if t.LastAnalyze == nil && t.LastAutoanalyze == nil && t.RowCount > 1000 {
			recommendations = append(recommendations,
				fmt.Sprintf("Tabel '%s' is nog nooit geanalyseerd - ANALYZE aanbevolen", t.Name))
		}

		// High sequential scans vs index scans (potential missing index)
		if t.SeqScans > 1000 && t.IdxScans > 0 && float64(t.SeqScans)/float64(t.IdxScans) > 10 {
			recommendations = append(recommendations,
				fmt.Sprintf("Tabel '%s' heeft veel sequential scans (%d) vs index scans (%d) - mogelijk ontbrekende index",
					t.Name, t.SeqScans, t.IdxScans))
		}

		// Large TOAST size (images stored in track/artist tables)
		if t.ToastSizeRaw > 500*1024*1024 { // > 500MB
			recommendations = append(recommendations,
				fmt.Sprintf("Tabel '%s' heeft %s aan TOAST data (afbeeldingen)", t.Name, t.ToastSize))
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Geen problemen gedetecteerd")
	}

	return recommendations
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// VacuumOptions configures vacuum operation parameters.
type VacuumOptions struct {
	Tables  []string // Specific tables to vacuum (empty = auto-select based on bloat)
	Analyze bool     // Run ANALYZE after VACUUM
	DryRun  bool     // Only show what would be done, don't execute
}

// VacuumResult represents the result of a vacuum operation on a single table.
type VacuumResult struct {
	Table         string  `json:"table"`
	Success       bool    `json:"success"`
	Message       string  `json:"message"`
	DeadTuples    int64   `json:"dead_tuples_before"`
	BloatPercent  float64 `json:"bloat_percent_before"`
	Duration      string  `json:"duration,omitempty"`
	Analyzed      bool    `json:"analyzed"`
	Skipped       bool    `json:"skipped,omitempty"`
	SkippedReason string  `json:"skipped_reason,omitempty"`
}

// VacuumResponse represents the overall result of vacuum operations.
type VacuumResponse struct {
	DryRun        bool           `json:"dry_run"`
	TablesTotal   int            `json:"tables_total"`
	TablesSuccess int            `json:"tables_success"`
	TablesFailed  int            `json:"tables_failed"`
	TablesSkipped int            `json:"tables_skipped"`
	Results       []VacuumResult `json:"results"`
	ExecutedAt    time.Time      `json:"executed_at"`
}

// VacuumTables performs VACUUM on tables in the schema.
// If no tables are specified, it automatically selects tables with high bloat (>10%) or many dead tuples (>10000).
func (s *AeronService) VacuumTables(opts VacuumOptions) (*VacuumResponse, error) {
	response := &VacuumResponse{
		DryRun:     opts.DryRun,
		ExecutedAt: time.Now(),
		Results:    []VacuumResult{},
	}

	// Get current table health to determine which tables need vacuum
	tables, err := getTableHealth(s.db, s.config.Database.Schema)
	if err != nil {
		return nil, fmt.Errorf("ophalen tabel statistieken mislukt: %w", err)
	}

	// Determine which tables to vacuum
	var tablesToVacuum []TableHealth
	if len(opts.Tables) > 0 {
		// User specified specific tables
		tableMap := make(map[string]TableHealth)
		for _, t := range tables {
			tableMap[t.Name] = t
		}
		for _, tableName := range opts.Tables {
			if t, exists := tableMap[tableName]; exists {
				tablesToVacuum = append(tablesToVacuum, t)
			} else {
				// Table not found in schema
				response.Results = append(response.Results, VacuumResult{
					Table:         tableName,
					Success:       false,
					Message:       fmt.Sprintf("Tabel '%s' niet gevonden in schema '%s'", tableName, s.config.Database.Schema),
					Skipped:       true,
					SkippedReason: "niet gevonden",
				})
				response.TablesSkipped++
			}
		}
	} else {
		// Auto-select tables with high bloat or many dead tuples
		for _, t := range tables {
			if t.BloatPercent > 10 || t.DeadTuples > 10000 {
				tablesToVacuum = append(tablesToVacuum, t)
			}
		}
	}

	response.TablesTotal = len(tablesToVacuum) + response.TablesSkipped

	// Process each table
	for _, table := range tablesToVacuum {
		result := VacuumResult{
			Table:        table.Name,
			DeadTuples:   table.DeadTuples,
			BloatPercent: table.BloatPercent,
			Analyzed:     opts.Analyze,
		}

		if opts.DryRun {
			result.Success = true
			if opts.Analyze {
				result.Message = fmt.Sprintf("Zou VACUUM ANALYZE uitvoeren op '%s'", table.Name)
			} else {
				result.Message = fmt.Sprintf("Zou VACUUM uitvoeren op '%s'", table.Name)
			}
			response.TablesSuccess++
		} else {
			// Execute the vacuum
			start := time.Now()
			err := s.executeVacuum(table.Name, opts.Analyze)
			duration := time.Since(start)
			result.Duration = duration.Round(time.Millisecond).String()

			if err != nil {
				result.Success = false
				result.Message = fmt.Sprintf("VACUUM mislukt: %v", err)
				response.TablesFailed++
			} else {
				result.Success = true
				if opts.Analyze {
					result.Message = fmt.Sprintf("VACUUM ANALYZE succesvol uitgevoerd op '%s'", table.Name)
				} else {
					result.Message = fmt.Sprintf("VACUUM succesvol uitgevoerd op '%s'", table.Name)
				}
				response.TablesSuccess++
			}
		}

		response.Results = append(response.Results, result)
	}

	return response, nil
}

// executeVacuum runs VACUUM on a single table.
func (s *AeronService) executeVacuum(tableName string, analyze bool) error {
	// Validate table name to prevent SQL injection
	if !isValidTableName(tableName) {
		return fmt.Errorf("ongeldige tabelnaam: %s", tableName)
	}

	// Build the VACUUM command
	// Note: VACUUM cannot be run inside a transaction, so we use Exec directly
	var query string
	if analyze {
		query = fmt.Sprintf("VACUUM ANALYZE %s.%s", s.config.Database.Schema, tableName)
	} else {
		query = fmt.Sprintf("VACUUM %s.%s", s.config.Database.Schema, tableName)
	}

	_, err := s.db.Exec(query)
	return err
}

// AnalyzeTables performs ANALYZE on tables in the schema.
func (s *AeronService) AnalyzeTables(tableNames []string) (*VacuumResponse, error) {
	response := &VacuumResponse{
		DryRun:     false,
		ExecutedAt: time.Now(),
		Results:    []VacuumResult{},
	}

	// Get current table health
	tables, err := getTableHealth(s.db, s.config.Database.Schema)
	if err != nil {
		return nil, fmt.Errorf("ophalen tabel statistieken mislukt: %w", err)
	}

	// Determine which tables to analyze
	var tablesToAnalyze []TableHealth
	if len(tableNames) > 0 {
		// User specified specific tables
		tableMap := make(map[string]TableHealth)
		for _, t := range tables {
			tableMap[t.Name] = t
		}
		for _, tableName := range tableNames {
			if t, exists := tableMap[tableName]; exists {
				tablesToAnalyze = append(tablesToAnalyze, t)
			} else {
				response.Results = append(response.Results, VacuumResult{
					Table:         tableName,
					Success:       false,
					Message:       fmt.Sprintf("Tabel '%s' niet gevonden in schema '%s'", tableName, s.config.Database.Schema),
					Skipped:       true,
					SkippedReason: "niet gevonden",
				})
				response.TablesSkipped++
			}
		}
	} else {
		// Analyze tables that were never analyzed
		for _, t := range tables {
			if t.LastAnalyze == nil && t.LastAutoanalyze == nil && t.RowCount > 0 {
				tablesToAnalyze = append(tablesToAnalyze, t)
			}
		}
	}

	response.TablesTotal = len(tablesToAnalyze) + response.TablesSkipped

	// Process each table
	for _, table := range tablesToAnalyze {
		result := VacuumResult{
			Table:        table.Name,
			DeadTuples:   table.DeadTuples,
			BloatPercent: table.BloatPercent,
			Analyzed:     true,
		}

		start := time.Now()
		err := s.executeAnalyze(table.Name)
		duration := time.Since(start)
		result.Duration = duration.Round(time.Millisecond).String()

		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("ANALYZE mislukt: %v", err)
			response.TablesFailed++
		} else {
			result.Success = true
			result.Message = fmt.Sprintf("ANALYZE succesvol uitgevoerd op '%s'", table.Name)
			response.TablesSuccess++
		}

		response.Results = append(response.Results, result)
	}

	return response, nil
}

// executeAnalyze runs ANALYZE on a single table.
func (s *AeronService) executeAnalyze(tableName string) error {
	if !isValidTableName(tableName) {
		return fmt.Errorf("ongeldige tabelnaam: %s", tableName)
	}

	query := fmt.Sprintf("ANALYZE %s.%s", s.config.Database.Schema, tableName)
	_, err := s.db.Exec(query)
	return err
}

// isValidTableName validates that a table name contains only safe characters.
func isValidTableName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
