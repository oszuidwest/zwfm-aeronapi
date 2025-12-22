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

// tableHealthRow represents a combined row from pg_stat_user_tables and pg_class.
type tableHealthRow struct {
	TableName       string     `db:"table_name"`
	LiveTuples      int64      `db:"live_tuples"`
	DeadTuples      int64      `db:"dead_tuples"`
	LastVacuum      *time.Time `db:"last_vacuum"`
	LastAutovacuum  *time.Time `db:"last_autovacuum"`
	LastAnalyze     *time.Time `db:"last_analyze"`
	LastAutoanalyze *time.Time `db:"last_autoanalyze"`
	SeqScan         int64      `db:"seq_scan"`
	IdxScan         int64      `db:"idx_scan"`
	TotalSize       int64      `db:"total_size"`
	TableSize       int64      `db:"table_size"`
	IndexSize       int64      `db:"index_size"`
	ToastSize       int64      `db:"toast_size"`
}

// GetDatabaseHealth retrieves comprehensive database health information.
func (s *AeronService) GetDatabaseHealth() (*DatabaseHealth, error) {
	health := &DatabaseHealth{
		DatabaseName: s.config.Database.Name,
		SchemaName:   s.schema,
		CheckedAt:    time.Now(),
	}

	// Get database size
	dbSize, dbSizeRaw, err := getDatabaseSize(s.db)
	if err != nil {
		return nil, fmt.Errorf("ophalen database grootte mislukt: %w", err)
	}
	health.DatabaseSize = dbSize
	health.DatabaseSizeRaw = dbSizeRaw

	// Get table statistics (combined query)
	tables, err := getTableHealth(s.db, s.schema)
	if err != nil {
		return nil, fmt.Errorf("ophalen tabel statistieken mislukt: %w", err)
	}
	health.Tables = tables

	// Generate recommendations using configurable thresholds
	health.Recommendations = s.generateRecommendations(tables)

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

// getTableHealth retrieves health statistics for all tables in the schema using a single combined query.
func getTableHealth(db *sqlx.DB, schema string) ([]TableHealth, error) {
	// Combined query that joins pg_stat_user_tables with pg_class for size info
	query := `
		SELECT
			s.relname as table_name,
			COALESCE(s.n_live_tup, 0) as live_tuples,
			COALESCE(s.n_dead_tup, 0) as dead_tuples,
			s.last_vacuum,
			s.last_autovacuum,
			s.last_analyze,
			s.last_autoanalyze,
			COALESCE(s.seq_scan, 0) as seq_scan,
			COALESCE(s.idx_scan, 0) as idx_scan,
			COALESCE(pg_total_relation_size(c.oid), 0) as total_size,
			COALESCE(pg_table_size(c.oid), 0) as table_size,
			COALESCE(pg_indexes_size(c.oid), 0) as index_size,
			COALESCE(pg_total_relation_size(c.reltoastrelid), 0) as toast_size
		FROM pg_stat_user_tables s
		JOIN pg_class c ON c.relname = s.relname
		JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = s.schemaname
		WHERE s.schemaname = $1 AND c.relkind = 'r'
		ORDER BY s.n_live_tup DESC
	`

	var rows []tableHealthRow
	if err := db.Select(&rows, query, schema); err != nil {
		return nil, fmt.Errorf("ophalen tabel statistieken mislukt: %w", err)
	}

	// Convert to TableHealth structs
	tables := make([]TableHealth, 0, len(rows))
	for _, row := range rows {
		table := TableHealth{
			Name:            row.TableName,
			RowCount:        row.LiveTuples,
			DeadTuples:      row.DeadTuples,
			LastVacuum:      row.LastVacuum,
			LastAutovacuum:  row.LastAutovacuum,
			LastAnalyze:     row.LastAnalyze,
			LastAutoanalyze: row.LastAutoanalyze,
			SeqScans:        row.SeqScan,
			IdxScans:        row.IdxScan,
			TotalSizeRaw:    row.TotalSize,
			TotalSize:       formatBytes(row.TotalSize),
			TableSizeRaw:    row.TableSize,
			TableSize:       formatBytes(row.TableSize),
			IndexSizeRaw:    row.IndexSize,
			IndexSize:       formatBytes(row.IndexSize),
			ToastSizeRaw:    row.ToastSize,
			ToastSize:       formatBytes(row.ToastSize),
		}

		// Calculate bloat estimate based on dead tuples
		if row.LiveTuples > 0 {
			table.BloatPercent = float64(row.DeadTuples) / float64(row.LiveTuples+row.DeadTuples) * 100
		}

		tables = append(tables, table)
	}

	return tables, nil
}

// generateRecommendations creates actionable recommendations based on table health.
func (s *AeronService) generateRecommendations(tables []TableHealth) []string {
	var recommendations []string
	bloatThreshold := s.config.Maintenance.GetBloatThreshold()
	deadTupleThreshold := s.config.Maintenance.GetDeadTupleThreshold()

	for _, t := range tables {
		// High bloat warning
		if t.BloatPercent > bloatThreshold {
			recommendations = append(recommendations,
				fmt.Sprintf("Tabel '%s' heeft %.1f%% bloat - VACUUM aanbevolen", t.Name, t.BloatPercent))
		}

		// Many dead tuples
		if t.DeadTuples > deadTupleThreshold {
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

// maintenanceContext holds shared context for vacuum/analyze operations.
type maintenanceContext struct {
	tables   []TableHealth
	tableMap map[string]TableHealth
	schema   string
}

// newMaintenanceContext creates a new maintenance context by fetching table health.
func (s *AeronService) newMaintenanceContext() (*maintenanceContext, error) {
	tables, err := getTableHealth(s.db, s.schema)
	if err != nil {
		return nil, fmt.Errorf("ophalen tabel statistieken mislukt: %w", err)
	}

	tableMap := make(map[string]TableHealth, len(tables))
	for _, t := range tables {
		tableMap[t.Name] = t
	}

	return &maintenanceContext{
		tables:   tables,
		tableMap: tableMap,
		schema:   s.schema,
	}, nil
}

// resolveTables resolves which tables to process based on user input or auto-selection.
// Returns tables to process and any skipped table results.
func (ctx *maintenanceContext) resolveTables(requestedTables []string, autoSelectFn func(TableHealth) bool) ([]TableHealth, []VacuumResult) {
	var tablesToProcess []TableHealth
	var skipped []VacuumResult

	if len(requestedTables) > 0 {
		// User specified specific tables
		for _, tableName := range requestedTables {
			if t, exists := ctx.tableMap[tableName]; exists {
				tablesToProcess = append(tablesToProcess, t)
			} else {
				skipped = append(skipped, VacuumResult{
					Table:         tableName,
					Success:       false,
					Message:       fmt.Sprintf("Tabel '%s' niet gevonden in schema '%s'", tableName, ctx.schema),
					Skipped:       true,
					SkippedReason: "niet gevonden",
				})
			}
		}
	} else {
		// Auto-select tables based on criteria
		for _, t := range ctx.tables {
			if autoSelectFn(t) {
				tablesToProcess = append(tablesToProcess, t)
			}
		}
	}

	return tablesToProcess, skipped
}

// VacuumTables performs VACUUM on tables in the schema.
// If no tables are specified, it automatically selects tables with high bloat or many dead tuples.
func (s *AeronService) VacuumTables(opts VacuumOptions) (*VacuumResponse, error) {
	response := &VacuumResponse{
		DryRun:     opts.DryRun,
		ExecutedAt: time.Now(),
		Results:    []VacuumResult{},
	}

	ctx, err := s.newMaintenanceContext()
	if err != nil {
		return nil, err
	}

	bloatThreshold := s.config.Maintenance.GetBloatThreshold()
	deadTupleThreshold := s.config.Maintenance.GetDeadTupleThreshold()

	// Auto-select criteria for vacuum
	autoSelect := func(t TableHealth) bool {
		return t.BloatPercent > bloatThreshold || t.DeadTuples > deadTupleThreshold
	}

	tablesToVacuum, skipped := ctx.resolveTables(opts.Tables, autoSelect)
	response.Results = append(response.Results, skipped...)
	response.TablesSkipped = len(skipped)
	response.TablesTotal = len(tablesToVacuum) + len(skipped)

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
	if !isValidIdentifier(tableName) {
		return fmt.Errorf("ongeldige tabelnaam: %s", tableName)
	}

	var query string
	if analyze {
		query = fmt.Sprintf("VACUUM ANALYZE %s.%s", s.schema, tableName)
	} else {
		query = fmt.Sprintf("VACUUM %s.%s", s.schema, tableName)
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

	ctx, err := s.newMaintenanceContext()
	if err != nil {
		return nil, err
	}

	// Auto-select criteria for analyze: tables never analyzed with data
	autoSelect := func(t TableHealth) bool {
		return t.LastAnalyze == nil && t.LastAutoanalyze == nil && t.RowCount > 0
	}

	tablesToAnalyze, skipped := ctx.resolveTables(tableNames, autoSelect)
	response.Results = append(response.Results, skipped...)
	response.TablesSkipped = len(skipped)
	response.TablesTotal = len(tablesToAnalyze) + len(skipped)

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
	if !isValidIdentifier(tableName) {
		return fmt.Errorf("ongeldige tabelnaam: %s", tableName)
	}

	query := fmt.Sprintf("ANALYZE %s.%s", s.schema, tableName)
	_, err := s.db.Exec(query)
	return err
}
