// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/util"
)

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
func (s *AeronService) GetDatabaseHealth(ctx context.Context) (*DatabaseHealth, error) {
	health := &DatabaseHealth{
		DatabaseName: s.config.Database.Name,
		SchemaName:   s.schema,
		CheckedAt:    time.Now(),
	}

	// Get PostgreSQL version
	var version string
	if err := s.db.GetContext(ctx, &version, "SELECT version()"); err == nil {
		health.DatabaseVersion = version
	}

	// Get database size
	dbSize, dbSizeRaw, err := getDatabaseSize(ctx, s.db)
	if err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen database grootte", Err: err}
	}
	health.DatabaseSize = dbSize
	health.DatabaseSizeRaw = dbSizeRaw

	// Get table statistics (combined query)
	tables, err := getTableHealth(ctx, s.db, s.schema)
	if err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen tabel statistieken", Err: err}
	}
	health.Tables = tables

	// Generate recommendations using configurable thresholds
	health.Recommendations = s.generateRecommendations(tables)

	return health, nil
}

// getDatabaseSize returns the total database size.
func getDatabaseSize(ctx context.Context, db DB) (string, int64, error) {
	var sizeRaw int64
	err := db.GetContext(ctx, &sizeRaw, "SELECT pg_database_size(current_database())")
	if err != nil {
		return "", 0, err
	}

	var sizePretty string
	err = db.GetContext(ctx, &sizePretty, "SELECT pg_size_pretty(pg_database_size(current_database()))")
	if err != nil {
		return "", 0, err
	}

	return sizePretty, sizeRaw, nil
}

// getTableHealth retrieves health statistics for all tables in the schema using a single combined query.
func getTableHealth(ctx context.Context, db DB, schema string) ([]TableHealth, error) {
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
	if err := db.SelectContext(ctx, &rows, query, schema); err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen tabel statistieken", Err: err}
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
			TotalSize:       util.FormatBytes(row.TotalSize),
			TableSizeRaw:    row.TableSize,
			TableSize:       util.FormatBytes(row.TableSize),
			IndexSizeRaw:    row.IndexSize,
			IndexSize:       util.FormatBytes(row.IndexSize),
			ToastSizeRaw:    row.ToastSize,
			ToastSize:       util.FormatBytes(row.ToastSize),
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
		lastVacuumTime := t.LastAutovacuum
		if t.LastVacuum != nil && (lastVacuumTime == nil || t.LastVacuum.After(*lastVacuumTime)) {
			lastVacuumTime = t.LastVacuum
		}
		if lastVacuumTime != nil && time.Since(*lastVacuumTime) > 7*24*time.Hour && t.RowCount > 1000 {
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
