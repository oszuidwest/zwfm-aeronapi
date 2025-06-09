package main

import (
	"database/sql"
	"fmt"
)

// Common database operations that can be used for both artists and tracks

type EntityType int

const (
	EntityArtist EntityType = iota
	EntityTrack
)

type QueryConfig struct {
	Schema       string
	Table        string
	IDColumn     string
	NameColumn   string
	ImageColumn  string
	ExtraColumns []string
}

func (e EntityType) QueryConfig(schema string) QueryConfig {
	switch e {
	case EntityArtist:
		return QueryConfig{
			Schema:      schema,
			Table:       "artist",
			IDColumn:    "artistid",
			NameColumn:  "artist",
			ImageColumn: "picture",
		}
	case EntityTrack:
		return QueryConfig{
			Schema:       schema,
			Table:        "track",
			IDColumn:     "titleid",
			NameColumn:   "tracktitle",
			ImageColumn:  "picture",
			ExtraColumns: []string{"artist"},
		}
	default:
		panic("unknown entity type")
	}
}

// Generic function to count items with images
func countItemsWithImages(db *sql.DB, config QueryConfig) (int, error) {
	query := fmt.Sprintf(
		`SELECT COUNT(*) FROM %s.%s WHERE %s IS NOT NULL`,
		config.Schema, config.Table, config.ImageColumn,
	)

	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("kon items met afbeeldingen niet tellen: %w", err)
	}
	return count, nil
}

// Generic function to delete all images
func deleteAllImages(db *sql.DB, config QueryConfig) (int64, error) {
	query := fmt.Sprintf(
		`UPDATE %s.%s SET %s = NULL WHERE %s IS NOT NULL`,
		config.Schema, config.Table, config.ImageColumn, config.ImageColumn,
	)

	result, err := db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("kon afbeeldingen niet verwijderen: %w", err)
	}

	return result.RowsAffected()
}
