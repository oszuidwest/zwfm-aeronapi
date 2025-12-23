// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/config"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/database"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/util"
)

// MaintenanceService handles database health monitoring and maintenance operations.
type MaintenanceService struct {
	repo   *database.Repository
	config *config.Config
}

// newMaintenanceService creates a new MaintenanceService instance.
func newMaintenanceService(repo *database.Repository, cfg *config.Config) *MaintenanceService {
	return &MaintenanceService{
		repo:   repo,
		config: cfg,
	}
}

// --- Types ---

// DatabaseHealth represents the overall health status of the database.
type DatabaseHealth struct {
	DatabaseName    string        `json:"database_name"`
	DatabaseVersion string        `json:"database_version"`
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

// VacuumOptions configures vacuum operation parameters.
type VacuumOptions struct {
	Tables  []string // Specific tables to vacuum (empty = auto-select based on bloat)
	Analyze bool     // Run ANALYZE after VACUUM
	DryRun  bool     // Only show what would be done, don't execute
}

// MaintenanceResult represents the result of a maintenance operation (vacuum or analyze) on a single table.
type MaintenanceResult struct {
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

// MaintenanceResponse represents the overall result of maintenance operations (vacuum/analyze).
type MaintenanceResponse struct {
	DryRun        bool                `json:"dry_run"`
	TablesTotal   int                 `json:"tables_total"`
	TablesSuccess int                 `json:"tables_success"`
	TablesFailed  int                 `json:"tables_failed"`
	TablesSkipped int                 `json:"tables_skipped"`
	Results       []MaintenanceResult `json:"results"`
	ExecutedAt    time.Time           `json:"executed_at"`
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

// maintenanceContext holds shared context for vacuum/analyze operations.
type maintenanceContext struct {
	tables       []TableHealth
	tablesByName map[string]TableHealth
	schema       string
}

// --- Health operations ---

// GetHealth retrieves comprehensive database health information.
func (s *MaintenanceService) GetHealth(ctx context.Context) (*DatabaseHealth, error) {
	schema := s.repo.Schema()
	health := &DatabaseHealth{
		DatabaseName: s.config.Database.Name,
		SchemaName:   schema,
		CheckedAt:    time.Now(),
	}

	var version string
	if err := s.repo.DB().GetContext(ctx, &version, "SELECT version()"); err == nil {
		health.DatabaseVersion = version
	}

	dbSize, dbSizeRaw, err := s.getDatabaseSize(ctx)
	if err != nil {
		return nil, types.NewOperationError("ophalen database grootte", err)
	}
	health.DatabaseSize = dbSize
	health.DatabaseSizeRaw = dbSizeRaw

	tables, err := s.getTableHealth(ctx)
	if err != nil {
		return nil, types.NewOperationError("ophalen tabel statistieken", err)
	}
	health.Tables = tables

	health.Recommendations = s.generateRecommendations(tables)

	return health, nil
}

// getDatabaseSize returns the total database size.
func (s *MaintenanceService) getDatabaseSize(ctx context.Context) (size string, sizeRaw int64, err error) {
	err = s.repo.DB().GetContext(ctx, &sizeRaw, "SELECT pg_database_size(current_database())")
	if err != nil {
		return "", 0, err
	}
	return util.FormatBytes(sizeRaw), sizeRaw, nil
}

// getTableHealth retrieves health statistics for all tables in the schema.
func (s *MaintenanceService) getTableHealth(ctx context.Context) ([]TableHealth, error) {
	schema := s.repo.Schema()
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
	if err := s.repo.DB().SelectContext(ctx, &rows, query, schema); err != nil {
		return nil, types.NewOperationError("ophalen tabel statistieken", err)
	}

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
			TotalSize:       util.FormatBytes(row.TotalSize),
			TableSizeRaw:    row.TableSize,
			TableSize:       util.FormatBytes(row.TableSize),
			IndexSizeRaw:    row.IndexSize,
			IndexSize:       util.FormatBytes(row.IndexSize),
			ToastSizeRaw:    row.ToastSize,
			ToastSize:       util.FormatBytes(row.ToastSize),
		}

		if row.LiveTuples > 0 {
			table.BloatPercent = float64(row.DeadTuples) / float64(row.LiveTuples+row.DeadTuples) * 100
		}

		tables = append(tables, table)
	}

	return tables, nil
}

// generateRecommendations creates actionable recommendations based on table health.
func (s *MaintenanceService) generateRecommendations(tables []TableHealth) []string {
	var recs []string
	bloatThreshold := s.config.Maintenance.GetBloatThreshold()
	deadTupleThreshold := s.config.Maintenance.GetDeadTupleThreshold()

	for i := range tables {
		t := &tables[i]
		recs = s.checkTableHealth(t, recs, bloatThreshold, deadTupleThreshold)
	}

	if len(recs) == 0 {
		return []string{"Geen problemen gedetecteerd"}
	}
	return recs
}

func (s *MaintenanceService) checkTableHealth(t *TableHealth, recs []string, bloatThreshold float64, deadTupleThreshold int64) []string {
	if t.BloatPercent > bloatThreshold {
		recs = append(recs, fmt.Sprintf("Tabel '%s' heeft %.1f%% bloat - VACUUM aanbevolen", t.Name, t.BloatPercent))
	}

	if t.DeadTuples > deadTupleThreshold {
		recs = append(recs, fmt.Sprintf("Tabel '%s' heeft %d dead tuples - VACUUM aanbevolen", t.Name, t.DeadTuples))
	}

	if t.LastVacuum == nil && t.LastAutovacuum == nil && t.RowCount > 1000 {
		recs = append(recs, fmt.Sprintf("Tabel '%s' is nog nooit gevacuumd", t.Name))
	}

	if lastVac := lastVacuumTime(t); lastVac != nil && time.Since(*lastVac) > 7*24*time.Hour && t.RowCount > 1000 {
		recs = append(recs, fmt.Sprintf("Tabel '%s' is meer dan 7 dagen niet gevacuumd", t.Name))
	}

	if t.LastAnalyze == nil && t.LastAutoanalyze == nil && t.RowCount > 1000 {
		recs = append(recs, fmt.Sprintf("Tabel '%s' is nog nooit geanalyseerd - ANALYZE aanbevolen", t.Name))
	}

	if t.SeqScans > 1000 && t.IdxScans > 0 && float64(t.SeqScans)/float64(t.IdxScans) > 10 {
		recs = append(recs, fmt.Sprintf("Tabel '%s' heeft veel sequential scans (%d) vs index scans (%d) - mogelijk ontbrekende index", t.Name, t.SeqScans, t.IdxScans))
	}

	if t.ToastSizeRaw > 500*1024*1024 {
		recs = append(recs, fmt.Sprintf("Tabel '%s' heeft %s aan TOAST data (afbeeldingen)", t.Name, t.ToastSize))
	}

	return recs
}

func lastVacuumTime(t *TableHealth) *time.Time {
	if t.LastVacuum != nil && (t.LastAutovacuum == nil || t.LastVacuum.After(*t.LastAutovacuum)) {
		return t.LastVacuum
	}
	return t.LastAutovacuum
}

// --- Vacuum operations ---

// newMaintenanceContext creates a new maintenance context with current table health data.
func (s *MaintenanceService) newMaintenanceContext(ctx context.Context) (*maintenanceContext, error) {
	tables, err := s.getTableHealth(ctx)
	if err != nil {
		return nil, err
	}

	tablesByName := make(map[string]TableHealth, len(tables))
	for i := range tables {
		tablesByName[tables[i].Name] = tables[i]
	}

	return &maintenanceContext{
		tables:       tables,
		tablesByName: tablesByName,
		schema:       s.repo.Schema(),
	}, nil
}

// selectTablesToProcess resolves which tables to process based on user input or auto-selection.
// Returns tables to process and any skipped table results.
func (mctx *maintenanceContext) selectTablesToProcess(requestedTables []string, autoSelectFn func(TableHealth) bool) ([]TableHealth, []MaintenanceResult) {
	var tablesToProcess []TableHealth
	var skipped []MaintenanceResult

	if len(requestedTables) > 0 {
		for _, tableName := range requestedTables {
			if t, exists := mctx.tablesByName[tableName]; exists {
				tablesToProcess = append(tablesToProcess, t)
			} else {
				skipped = append(skipped, MaintenanceResult{
					Table:         tableName,
					Success:       false,
					Message:       fmt.Sprintf("Tabel '%s' niet gevonden in schema '%s'", tableName, mctx.schema),
					Skipped:       true,
					SkippedReason: "niet gevonden",
				})
			}
		}
	} else {
		for i := range mctx.tables {
			if autoSelectFn(mctx.tables[i]) {
				tablesToProcess = append(tablesToProcess, mctx.tables[i])
			}
		}
	}

	return tablesToProcess, skipped
}

// Vacuum performs VACUUM on tables in the schema.
// If no tables are specified, it automatically selects tables with high bloat or many dead tuples.
func (s *MaintenanceService) Vacuum(ctx context.Context, opts VacuumOptions) (*MaintenanceResponse, error) {
	response := &MaintenanceResponse{
		DryRun:     opts.DryRun,
		ExecutedAt: time.Now(),
		Results:    []MaintenanceResult{},
	}

	mctx, err := s.newMaintenanceContext(ctx)
	if err != nil {
		return nil, err
	}

	bloatThreshold := s.config.Maintenance.GetBloatThreshold()
	deadTupleThreshold := s.config.Maintenance.GetDeadTupleThreshold()

	autoSelect := func(t TableHealth) bool {
		return t.BloatPercent > bloatThreshold || t.DeadTuples > deadTupleThreshold
	}

	tablesToVacuum, skipped := mctx.selectTablesToProcess(opts.Tables, autoSelect)
	response.Results = append(response.Results, skipped...)
	response.TablesSkipped = len(skipped)
	response.TablesTotal = len(tablesToVacuum) + len(skipped)

	for i := range tablesToVacuum {
		result := MaintenanceResult{
			Table:        tablesToVacuum[i].Name,
			DeadTuples:   tablesToVacuum[i].DeadTuples,
			BloatPercent: tablesToVacuum[i].BloatPercent,
			Analyzed:     opts.Analyze,
		}

		if opts.DryRun {
			result.Success = true
			if opts.Analyze {
				result.Message = fmt.Sprintf("Zou VACUUM ANALYZE uitvoeren op '%s'", tablesToVacuum[i].Name)
			} else {
				result.Message = fmt.Sprintf("Zou VACUUM uitvoeren op '%s'", tablesToVacuum[i].Name)
			}
			response.TablesSuccess++
		} else {
			start := time.Now()
			err := s.executeVacuum(ctx, tablesToVacuum[i].Name, opts.Analyze)
			duration := time.Since(start)
			result.Duration = duration.Round(time.Millisecond).String()

			if err != nil {
				result.Success = false
				result.Message = fmt.Sprintf("VACUUM mislukt: %v", err)
				response.TablesFailed++
			} else {
				result.Success = true
				if opts.Analyze {
					result.Message = fmt.Sprintf("VACUUM ANALYZE succesvol uitgevoerd op '%s'", tablesToVacuum[i].Name)
				} else {
					result.Message = fmt.Sprintf("VACUUM succesvol uitgevoerd op '%s'", tablesToVacuum[i].Name)
				}
				response.TablesSuccess++
			}
		}

		response.Results = append(response.Results, result)
	}

	return response, nil
}

// executeVacuum runs VACUUM on a single table.
func (s *MaintenanceService) executeVacuum(ctx context.Context, tableName string, analyze bool) error {
	if !types.IsValidIdentifier(tableName) {
		return types.NewValidationError("table", fmt.Sprintf("ongeldige tabelnaam: %s", tableName))
	}

	schema := s.repo.Schema()
	var query string
	if analyze {
		query = fmt.Sprintf("VACUUM ANALYZE %s.%s", schema, tableName)
	} else {
		query = fmt.Sprintf("VACUUM %s.%s", schema, tableName)
	}

	_, err := s.repo.DB().ExecContext(ctx, query)
	return err
}

// --- Analyze operations ---

// Analyze performs ANALYZE on tables in the schema.
func (s *MaintenanceService) Analyze(ctx context.Context, tableNames []string) (*MaintenanceResponse, error) {
	response := &MaintenanceResponse{
		DryRun:     false,
		ExecutedAt: time.Now(),
		Results:    []MaintenanceResult{},
	}

	mctx, err := s.newMaintenanceContext(ctx)
	if err != nil {
		return nil, err
	}

	autoSelect := func(t TableHealth) bool {
		return t.LastAnalyze == nil && t.LastAutoanalyze == nil && t.RowCount > 0
	}

	tablesToAnalyze, skipped := mctx.selectTablesToProcess(tableNames, autoSelect)
	response.Results = append(response.Results, skipped...)
	response.TablesSkipped = len(skipped)
	response.TablesTotal = len(tablesToAnalyze) + len(skipped)

	for i := range tablesToAnalyze {
		result := MaintenanceResult{
			Table:        tablesToAnalyze[i].Name,
			DeadTuples:   tablesToAnalyze[i].DeadTuples,
			BloatPercent: tablesToAnalyze[i].BloatPercent,
			Analyzed:     true,
		}

		start := time.Now()
		err := s.executeAnalyze(ctx, tablesToAnalyze[i].Name)
		duration := time.Since(start)
		result.Duration = duration.Round(time.Millisecond).String()

		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("ANALYZE mislukt: %v", err)
			response.TablesFailed++
		} else {
			result.Success = true
			result.Message = fmt.Sprintf("ANALYZE succesvol uitgevoerd op '%s'", tablesToAnalyze[i].Name)
			response.TablesSuccess++
		}

		response.Results = append(response.Results, result)
	}

	return response, nil
}

// executeAnalyze runs ANALYZE on a single table.
func (s *MaintenanceService) executeAnalyze(ctx context.Context, tableName string) error {
	if !types.IsValidIdentifier(tableName) {
		return types.NewValidationError("table", fmt.Sprintf("ongeldige tabelnaam: %s", tableName))
	}

	schema := s.repo.Schema()
	query := fmt.Sprintf("ANALYZE %s.%s", schema, tableName)
	_, err := s.repo.DB().ExecContext(ctx, query)
	return err
}
