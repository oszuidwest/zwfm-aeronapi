// Package database provides PostgreSQL data access for the Aeron database.
package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oszuidwest/zwfm-aeronapi/internal/types"

	_ "github.com/lib/pq"
)

// DB defines the minimal database interface required for data access operations.
// This interface follows the Interface Segregation Principle - it only includes
// the methods needed by this package (query and exec), not health monitoring.
// The *sqlx.DB type satisfies this interface.
type DB interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// Artist represents a basic artist entity from the database.
// It contains minimal information used for listing operations.
type Artist struct {
	ID         string `db:"artistid"`  // UUID of the artist
	ArtistName string `db:"artist"`    // Artist name
	HasImage   bool   `db:"has_image"` // Whether the artist has an associated image
}

// ArtistDetails represents complete artist information from the database.
// It includes all metadata fields available for an artist entity.
type ArtistDetails struct {
	ID          string `db:"artistid" json:"artistid"`         // UUID of the artist
	ArtistName  string `db:"artist" json:"artist"`             // Artist name
	Info        string `db:"info" json:"info"`                 // Artist biography or information
	Website     string `db:"website" json:"website"`           // Artist website URL
	Twitter     string `db:"twitter" json:"twitter"`           // Twitter handle
	Instagram   string `db:"instagram" json:"instagram"`       // Instagram handle
	HasImage    bool   `db:"has_image" json:"has_image"`       // Whether the artist has an associated image
	RepeatValue int    `db:"repeat_value" json:"repeat_value"` // Repeat restriction value
}

// Track represents a basic track entity from the database.
// It contains minimal information used for listing operations.
type Track struct {
	ID         string `db:"titleid"`    // UUID of the track
	TrackTitle string `db:"tracktitle"` // Track title
	Artist     string `db:"artist"`     // Artist name
	HasImage   bool   `db:"has_image"`  // Whether the track has an associated image
}

// TrackDetails represents complete track information from the database.
// It includes all metadata fields available for a track entity.
type TrackDetails struct {
	ID             string `db:"titleid" json:"titleid"`           // UUID of the track
	TrackTitle     string `db:"tracktitle" json:"tracktitle"`     // Track title
	Artist         string `db:"artist" json:"artist"`             // Artist name
	ArtistID       string `db:"artistid" json:"artistid"`         // UUID of the associated artist
	Year           int    `db:"year" json:"year"`                 // Release year
	KnownLengthMs  int    `db:"knownlength" json:"knownlength"`   // Track length in milliseconds
	IntroTimeMs    int    `db:"introtime" json:"introtime"`       // Intro length in milliseconds
	OutroTimeMs    int    `db:"outrotime" json:"outrotime"`       // Outro length in milliseconds
	Tempo          int    `db:"tempo" json:"tempo"`               // Tempo classification (values defined in Aeron system)
	BPM            int    `db:"bpm" json:"bpm"`                   // Beats per minute
	Gender         int    `db:"gender" json:"gender"`             // Vocalist gender classification
	Language       int    `db:"language" json:"language"`         // Language classification
	Mood           int    `db:"mood" json:"mood"`                 // Mood classification
	ExportType     int    `db:"exporttype" json:"exporttype"`     // Export type (2 = excluded from operations)
	RepeatValue    int    `db:"repeat_value" json:"repeat_value"` // Repeat restriction value
	Rating         int    `db:"rating" json:"rating"`             // Track rating
	HasImage       bool   `db:"has_image" json:"has_image"`       // Whether the track has an associated image
	Website        string `db:"website" json:"website"`           // Related website URL
	Conductor      string `db:"conductor" json:"conductor"`       // Conductor name (for classical music)
	Orchestra      string `db:"orchestra" json:"orchestra"`       // Orchestra name (for classical music)
}

// CountItems counts entities in the specified table based on image presence.
// It returns the number of entities that either have or don't have images.
func CountItems(ctx context.Context, db DB, schema string, table types.Table, hasImage bool) (int, error) {
	condition := "IS NULL"
	if hasImage {
		condition = "IS NOT NULL"
	}

	qualifiedTableName, err := types.QualifiedTable(schema, table)
	if err != nil {
		return 0, types.NewValidationError("table", fmt.Sprintf("ongeldige tabel configuratie: %v", err))
	}
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE picture %s", qualifiedTableName, condition)

	var count int
	err = db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, &types.DatabaseError{Operation: fmt.Sprintf("tellen van %s", table), Err: err}
	}

	return count, nil
}

// UpdateEntityImage updates the image data for either an artist or track entity.
// The table parameter determines whether to update the artist or track table.
func UpdateEntityImage(ctx context.Context, db DB, schema string, table types.Table, id string, imageData []byte) error {
	qualifiedTableName, err := types.QualifiedTable(schema, table)
	if err != nil {
		return types.NewValidationError("table", fmt.Sprintf("ongeldige tabel configuratie: %v", err))
	}
	label := types.LabelForTable(table)
	idCol := types.IDColumnForTable(table)

	query := fmt.Sprintf("UPDATE %s SET picture = $1 WHERE %s = $2", qualifiedTableName, idCol)

	_, err = db.ExecContext(ctx, query, imageData, id)
	if err != nil {
		return &types.DatabaseError{Operation: fmt.Sprintf("bijwerken van %s", label), Err: err}
	}
	return nil
}

// DeleteEntityImage removes the image data for either an artist or track entity.
// It sets the picture column to NULL and returns an error if the entity doesn't exist.
func DeleteEntityImage(ctx context.Context, db DB, schema string, table types.Table, id string) error {
	qualifiedTableName, err := types.QualifiedTable(schema, table)
	if err != nil {
		return types.NewValidationError("table", fmt.Sprintf("ongeldige tabel configuratie: %v", err))
	}
	label := types.LabelForTable(table)
	idCol := types.IDColumnForTable(table)

	query := fmt.Sprintf("UPDATE %s SET picture = NULL WHERE %s = $1", qualifiedTableName, idCol)

	result, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return &types.DatabaseError{Operation: fmt.Sprintf("verwijderen van %s-afbeelding", label), Err: err}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.DatabaseError{Operation: fmt.Sprintf("verwijderen van %s-afbeelding", label), Err: err}
	}

	if rowsAffected == 0 {
		return types.NewNotFoundError(label+"-afbeelding", id)
	}

	return nil
}

const artistDetailsQuery = `
	SELECT
		artistid,
		COALESCE(artist, '') as artist,
		COALESCE(info, '') as info,
		COALESCE(website, '') as website,
		COALESCE(twitter, '') as twitter,
		COALESCE(instagram, '') as instagram,
		CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image,
		COALESCE(repeatvalue, 0) as repeat_value
	FROM %s.artist
	WHERE artistid = $1`

// GetArtistByID retrieves complete artist details by UUID.
// It returns an error if the artist doesn't exist in the database.
func GetArtistByID(ctx context.Context, db DB, schema, artistID string) (*ArtistDetails, error) {
	query := fmt.Sprintf(artistDetailsQuery, schema)

	var artist ArtistDetails
	err := db.GetContext(ctx, &artist, query, artistID)

	if err == sql.ErrNoRows {
		return nil, types.NewNotFoundError("artiest", artistID)
	}
	if err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen artiest", Err: err}
	}

	return &artist, nil
}

// trackDetailsQuery retrieves all track metadata from the Aeron database.
// Note: "Year" and "Language" columns require quotes because they are case-sensitive
// identifiers in the Aeron PostgreSQL schema (mixed-case column names).
const trackDetailsQuery = `
	SELECT
		titleid,
		COALESCE(tracktitle, '') as tracktitle,
		COALESCE(artist, '') as artist,
		COALESCE(artistid, '00000000-0000-0000-0000-000000000000') as artistid,
		COALESCE("Year", 0) as year,
		COALESCE(knownlength, 0) as knownlength,
		COALESCE(introtime, 0) as introtime,
		COALESCE(outrotime, 0) as outrotime,
		COALESCE(tempo, 0) as tempo,
		COALESCE(bpm, 0) as bpm,
		COALESCE(gender, 0) as gender,
		COALESCE("Language", 0) as language,
		COALESCE(mood, 0) as mood,
		COALESCE(exporttype, 0) as exporttype,
		COALESCE(repeatvalue, 0) as repeat_value,
		COALESCE(rating, 0) as rating,
		CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image,
		COALESCE(website, '') as website,
		COALESCE(conductor, '') as conductor,
		COALESCE(orchestra, '') as orchestra
	FROM %s.track
	WHERE titleid = $1`

// GetTrackByID retrieves complete track details by UUID.
// It returns an error if the track doesn't exist in the database.
func GetTrackByID(ctx context.Context, db DB, schema, trackID string) (*TrackDetails, error) {
	query := fmt.Sprintf(trackDetailsQuery, schema)

	var track TrackDetails
	err := db.GetContext(ctx, &track, query, trackID)

	if err == sql.ErrNoRows {
		return nil, types.NewNotFoundError("track", trackID)
	}
	if err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen track", Err: err}
	}

	return &track, nil
}

// GetEntityImage retrieves the image data for either an artist or track entity.
// It returns the raw image bytes or an error if the entity doesn't exist or has no image.
func GetEntityImage(ctx context.Context, db DB, schema string, table types.Table, id string) ([]byte, error) {
	qualifiedTableName, err := types.QualifiedTable(schema, table)
	if err != nil {
		return nil, types.NewValidationError("table", fmt.Sprintf("ongeldige tabel configuratie: %v", err))
	}
	label := types.LabelForTable(table)
	idCol := types.IDColumnForTable(table)

	query := fmt.Sprintf("SELECT picture FROM %s WHERE %s = $1", qualifiedTableName, idCol)

	var imageData []byte
	err = db.GetContext(ctx, &imageData, query, id)

	if err == sql.ErrNoRows {
		return nil, types.NewNotFoundError(label, id)
	}
	if err != nil {
		return nil, &types.DatabaseError{Operation: fmt.Sprintf("ophalen %s afbeelding", label), Err: err}
	}
	if imageData == nil {
		return nil, types.NewNoImageError(label, id)
	}

	return imageData, nil
}
