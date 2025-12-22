// Package database provides PostgreSQL data access for the Aeron database.
package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"

	_ "github.com/lib/pq"
)

// DB defines the database interface for data access operations.
type DB interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Artist represents a basic artist entity from the database.
type Artist struct {
	ID         string `db:"artistid"`
	ArtistName string `db:"artist"`
	HasImage   bool   `db:"has_image"`
}

// ArtistDetails represents complete artist information from the database.
type ArtistDetails struct {
	ID          string `db:"artistid" json:"artistid"`
	ArtistName  string `db:"artist" json:"artist"`
	Info        string `db:"info" json:"info"`
	Website     string `db:"website" json:"website"`
	Twitter     string `db:"twitter" json:"twitter"`
	Instagram   string `db:"instagram" json:"instagram"`
	HasImage    bool   `db:"has_image" json:"has_image"`
	RepeatValue int    `db:"repeat_value" json:"repeat_value"`
}

// Track represents a basic track entity from the database.
type Track struct {
	ID         string `db:"titleid"`
	TrackTitle string `db:"tracktitle"`
	Artist     string `db:"artist"`
	HasImage   bool   `db:"has_image"`
}

// TrackDetails represents complete track information from the database.
type TrackDetails struct {
	ID            string `db:"titleid" json:"titleid"`
	TrackTitle    string `db:"tracktitle" json:"tracktitle"`
	Artist        string `db:"artist" json:"artist"`
	ArtistID      string `db:"artistid" json:"artistid"`
	Year          int    `db:"year" json:"year"`
	KnownLengthMs int    `db:"knownlength" json:"knownlength"`
	IntroTimeMs   int    `db:"introtime" json:"introtime"`
	OutroTimeMs   int    `db:"outrotime" json:"outrotime"`
	Tempo         int    `db:"tempo" json:"tempo"`
	BPM           int    `db:"bpm" json:"bpm"`
	Gender        int    `db:"gender" json:"gender"`
	Language      int    `db:"language" json:"language"`
	Mood          int    `db:"mood" json:"mood"`
	ExportType    int    `db:"exporttype" json:"exporttype"`
	RepeatValue   int    `db:"repeat_value" json:"repeat_value"`
	Rating        int    `db:"rating" json:"rating"`
	HasImage      bool   `db:"has_image" json:"has_image"`
	Website       string `db:"website" json:"website"`
	Conductor     string `db:"conductor" json:"conductor"`
	Orchestra     string `db:"orchestra" json:"orchestra"`
}

// CountItems counts entities by image presence in the specified table.
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

// UpdateEntityImage updates the image for an artist or track entity.
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

// DeleteEntityImage removes the image for an artist or track entity.
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

func getEntityByID[T any](ctx context.Context, db DB, query, id, label, operation string) (*T, error) {
	var entity T
	if err := db.GetContext(ctx, &entity, query, id); err == sql.ErrNoRows {
		return nil, types.NewNotFoundError(label, id)
	} else if err != nil {
		return nil, &types.DatabaseError{Operation: operation, Err: err}
	}
	return &entity, nil
}

// GetArtistByID retrieves complete artist details by UUID.
func GetArtistByID(ctx context.Context, db DB, schema, artistID string) (*ArtistDetails, error) {
	query := fmt.Sprintf(artistDetailsQuery, schema)
	return getEntityByID[ArtistDetails](ctx, db, query, artistID, "artiest", "ophalen artiest")
}

// GetTrackByID retrieves complete track details by UUID.
func GetTrackByID(ctx context.Context, db DB, schema, trackID string) (*TrackDetails, error) {
	query := fmt.Sprintf(trackDetailsQuery, schema)
	return getEntityByID[TrackDetails](ctx, db, query, trackID, "track", "ophalen track")
}

// GetEntityImage retrieves the image for an artist or track entity.
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
