// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
)

// AnalyzeTables performs ANALYZE on tables in the schema.
func (s *AeronService) AnalyzeTables(ctx context.Context, tableNames []string) (*VacuumResponse, error) {
	response := &VacuumResponse{
		DryRun:     false,
		ExecutedAt: time.Now(),
		Results:    []MaintenanceResult{},
	}

	mctx, err := s.newMaintenanceContext(ctx)
	if err != nil {
		return nil, err
	}

	// Auto-select criteria for analyze: tables never analyzed with data
	autoSelect := func(t TableHealth) bool {
		return t.LastAnalyze == nil && t.LastAutoanalyze == nil && t.RowCount > 0
	}

	tablesToAnalyze, skipped := mctx.selectTablesToProcess(tableNames, autoSelect)
	response.Results = append(response.Results, skipped...)
	response.TablesSkipped = len(skipped)
	response.TablesTotal = len(tablesToAnalyze) + len(skipped)

	// Process each table
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
func (s *AeronService) executeAnalyze(ctx context.Context, tableName string) error {
	if !types.IsValidIdentifier(tableName) {
		return types.NewValidationError("table", fmt.Sprintf("ongeldige tabelnaam: %s", tableName))
	}

	query := fmt.Sprintf("ANALYZE %s.%s", s.schema, tableName)
	_, err := s.db.ExecContext(ctx, query)
	return err
}
