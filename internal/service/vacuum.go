// Package service provides business logic for managing images in the Aeron radio automation system.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
)

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
func (s *AeronService) newMaintenanceContext(ctx context.Context) (*maintenanceContext, error) {
	tables, err := getTableHealth(ctx, s.db, s.schema)
	if err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen tabel statistieken", Err: err}
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
func (mctx *maintenanceContext) resolveTables(requestedTables []string, autoSelectFn func(TableHealth) bool) ([]TableHealth, []VacuumResult) {
	var tablesToProcess []TableHealth
	var skipped []VacuumResult

	if len(requestedTables) > 0 {
		// User specified specific tables
		for _, tableName := range requestedTables {
			if t, exists := mctx.tableMap[tableName]; exists {
				tablesToProcess = append(tablesToProcess, t)
			} else {
				skipped = append(skipped, VacuumResult{
					Table:         tableName,
					Success:       false,
					Message:       fmt.Sprintf("Tabel '%s' niet gevonden in schema '%s'", tableName, mctx.schema),
					Skipped:       true,
					SkippedReason: "niet gevonden",
				})
			}
		}
	} else {
		// Auto-select tables based on criteria
		for _, t := range mctx.tables {
			if autoSelectFn(t) {
				tablesToProcess = append(tablesToProcess, t)
			}
		}
	}

	return tablesToProcess, skipped
}

// VacuumTables performs VACUUM on tables in the schema.
// If no tables are specified, it automatically selects tables with high bloat or many dead tuples.
func (s *AeronService) VacuumTables(ctx context.Context, opts VacuumOptions) (*VacuumResponse, error) {
	response := &VacuumResponse{
		DryRun:     opts.DryRun,
		ExecutedAt: time.Now(),
		Results:    []VacuumResult{},
	}

	mctx, err := s.newMaintenanceContext(ctx)
	if err != nil {
		return nil, err
	}

	bloatThreshold := s.config.Maintenance.GetBloatThreshold()
	deadTupleThreshold := s.config.Maintenance.GetDeadTupleThreshold()

	// Auto-select criteria for vacuum
	autoSelect := func(t TableHealth) bool {
		return t.BloatPercent > bloatThreshold || t.DeadTuples > deadTupleThreshold
	}

	tablesToVacuum, skipped := mctx.resolveTables(opts.Tables, autoSelect)
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
			err := s.executeVacuum(ctx, table.Name, opts.Analyze)
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
func (s *AeronService) executeVacuum(ctx context.Context, tableName string, analyze bool) error {
	if !types.IsValidIdentifier(tableName) {
		return types.NewValidationError("table", fmt.Sprintf("ongeldige tabelnaam: %s", tableName))
	}

	var query string
	if analyze {
		query = fmt.Sprintf("VACUUM ANALYZE %s.%s", s.schema, tableName)
	} else {
		query = fmt.Sprintf("VACUUM %s.%s", s.schema, tableName)
	}

	_, err := s.db.ExecContext(ctx, query)
	return err
}
