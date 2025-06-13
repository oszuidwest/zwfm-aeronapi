package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Database constants - only used in this file
const (
	// Column names
	columnPicture  = "picture"
	columnArtistID = "artistid"
	columnTitleID  = "titleid"
	columnArtist   = "artist"

	// Table names
	tableArtist = "artist"
	tableTrack  = "track"
)

// Entity configuration for reusable patterns
type EntityConfig struct {
	Table    string
	IDColumn string
	IDName   string // for error messages
}

var (
	artistEntity = EntityConfig{Table: tableArtist, IDColumn: columnArtistID, IDName: "artiest"}
	trackEntity  = EntityConfig{Table: tableTrack, IDColumn: columnTitleID, IDName: "track"}
)

// Query templates using sqlx Named syntax
const (
	lookupArtistByIDQuery = `
		SELECT artistid, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		FROM %s.artist WHERE artistid = $1`

	lookupArtistByNameQuery = `
		SELECT artistid, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		FROM %s.artist WHERE artist = $1`

	lookupTrackByIDQuery = `
		SELECT titleid, tracktitle, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		FROM %s.track WHERE titleid = $1`

	lookupTrackByNameQuery = `
		SELECT titleid, tracktitle, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		FROM %s.track WHERE tracktitle = $1`
)

type Artist struct {
	ID       string `db:"artistid"`
	Name     string `db:"artist"`
	HasImage bool   `db:"has_image"`
}

type ArtistDetails struct {
	ID          string `db:"artistid" json:"artistid"`
	Name        string `db:"artist" json:"artist"`
	Info        string `db:"info" json:"info"`
	Website     string `db:"website" json:"website"`
	Twitter     string `db:"twitter" json:"twitter"`
	Instagram   string `db:"instagram" json:"instagram"`
	HasImage    bool   `db:"has_image" json:"has_image"`
	RepeatValue int    `db:"repeat_value" json:"repeat_value"`
}

type Track struct {
	ID       string `db:"titleid"`
	Title    string `db:"tracktitle"`
	Artist   string `db:"artist"`
	HasImage bool   `db:"has_image"`
}

type TrackDetails struct {
	ID          string `db:"titleid" json:"titleid"`
	Title       string `db:"tracktitle" json:"tracktitle"`
	Artist      string `db:"artist" json:"artist"`
	ArtistID    string `db:"artistid" json:"artistid"`
	Year        int    `db:"year" json:"year"`
	KnownLength int    `db:"knownlength" json:"knownlength"`
	IntroTime   int    `db:"introtime" json:"introtime"`
	OutroTime   int    `db:"outrotime" json:"outrotime"`
	Tempo       int    `db:"tempo" json:"tempo"`
	BPM         int    `db:"bpm" json:"bpm"`
	Gender      int    `db:"gender" json:"gender"`
	Language    int    `db:"language" json:"language"`
	Mood        int    `db:"mood" json:"mood"`
	ExportType  int    `db:"exporttype" json:"exporttype"`
	RepeatValue int    `db:"repeat_value" json:"repeat_value"`
	Rating      int    `db:"rating" json:"rating"`
	HasImage    bool   `db:"has_image" json:"has_image"`
	Website     string `db:"website" json:"website"`
	Conductor   string `db:"conductor" json:"conductor"`
	Orchestra   string `db:"orchestra" json:"orchestra"`
}

type PlaylistItem struct {
	SongID         string `db:"songid" json:"songid"`
	SongName       string `db:"songname" json:"songname"`
	ArtistID       string `db:"artistid" json:"artistid"`
	ArtistName     string `db:"artistname" json:"artistname"`
	StartTime      string `db:"start_time" json:"start_time"`
	EndTime        string `db:"end_time" json:"end_time"`
	Duration       int    `db:"duration" json:"duration"`
	HasTrackImage  bool   `db:"has_track_image" json:"has_track_image"`
	HasArtistImage bool   `db:"has_artist_image" json:"has_artist_image"`
}

// PlaylistOptions configures playlist queries
type PlaylistOptions struct {
	Date        string // Specific date (YYYY-MM-DD), empty = today
	StartTime   string // Filter from time (HH:MM)
	EndTime     string // Filter until time (HH:MM)
	ItemTypes   []int  // Item types to include (default: [1])
	ExportTypes []int  // Export types to exclude (default: [0])
	Limit       int    // Max items to return (0 = all)
	Offset      int    // Pagination offset
	SortBy      string // Sort field (default: "starttime")
	SortDesc    bool   // Sort descending
	// Image filters
	TrackImage  *bool // Filter by track image: true (has), false (no), nil (all)
	ArtistImage *bool // Filter by artist image: true (has), false (no), nil (all)
}

// DefaultPlaylistOptions returns default playlist options
func defaultPlaylistOptions() PlaylistOptions {
	return PlaylistOptions{
		ItemTypes:   []int{1},
		ExportTypes: []int{0},
		SortBy:      "starttime",
	}
}

// Count items with flexible conditions using direct queries
func countItems(db *sqlx.DB, schema, table string, hasImage bool) (int, error) {
	condition := "IS NULL"
	if hasImage {
		condition = "IS NOT NULL"
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE picture %s", schema, table, condition)

	var count int
	err := db.Get(&count, query)
	if err != nil {
		return 0, fmt.Errorf("telfout %s: %w", table, err)
	}

	return count, nil
}

// Lookup artist using sqlx struct mapping
func lookupArtistByNameOrID(db *sqlx.DB, schema, artistName, artistID string) (*Artist, error) {
	var artist Artist
	var err error

	if artistID != "" {
		query := fmt.Sprintf(lookupArtistByIDQuery, schema)
		err = db.Get(&artist, query, artistID)
	} else {
		query := fmt.Sprintf(lookupArtistByNameQuery, schema)
		err = db.Get(&artist, query, artistName)
	}

	if err == sql.ErrNoRows {
		if artistID != "" {
			return nil, fmt.Errorf("artiest ID '%s' %s", artistID, ErrSuffixNotExists)
		}
		return nil, fmt.Errorf("artiest '%s' %s", artistName, ErrSuffixNotExists)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}

	return &artist, nil
}

func lookupTrackByNameOrID(db *sqlx.DB, schema, trackTitle, trackID string) (*Track, error) {
	var track Track
	var err error

	if trackID != "" {
		query := fmt.Sprintf(lookupTrackByIDQuery, schema)
		err = db.Get(&track, query, trackID)
	} else {
		query := fmt.Sprintf(lookupTrackByNameQuery, schema)
		err = db.Get(&track, query, trackTitle)
	}

	if err == sql.ErrNoRows {
		if trackID != "" {
			return nil, fmt.Errorf("track ID '%s' %s", trackID, ErrSuffixNotExists)
		}
		return nil, fmt.Errorf("track '%s' %s", trackTitle, ErrSuffixNotExists)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}

	return &track, nil
}

func lookupArtist(db *sqlx.DB, schema, artistName, artistID string) (*Artist, error) {
	return lookupArtistByNameOrID(db, schema, artistName, artistID)
}

// Unified image operations using direct queries
func updateEntityImage(db *sqlx.DB, schema string, entity EntityConfig, entityID string, imageData []byte) error {
	query := fmt.Sprintf("UPDATE %s.%s SET picture = $1 WHERE %s = $2", schema, entity.Table, entity.IDColumn)
	_, err := db.Exec(query, imageData, entityID)
	return err
}

func updateArtistImage(db *sqlx.DB, schema, artistID string, imageData []byte) error {
	if err := updateEntityImage(db, schema, artistEntity, artistID, imageData); err != nil {
		return fmt.Errorf("update artiest %s: %w", ErrSuffixFailed, err)
	}
	return nil
}

func lookupTrack(db *sqlx.DB, schema, trackTitle, trackID string) (*Track, error) {
	return lookupTrackByNameOrID(db, schema, trackTitle, trackID)
}

func updateTrackImage(db *sqlx.DB, schema, trackID string, imageData []byte) error {
	if err := updateEntityImage(db, schema, trackEntity, trackID, imageData); err != nil {
		return fmt.Errorf("update track %s: %w", ErrSuffixFailed, err)
	}
	return nil
}

// Unified delete operations using direct queries
func deleteEntityImage(db *sqlx.DB, schema string, entity EntityConfig, entityID string) error {
	query := fmt.Sprintf("UPDATE %s.%s SET picture = NULL WHERE %s = $1", schema, entity.Table, entity.IDColumn)

	result, err := db.Exec(query, entityID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("geen rij gevonden met ID %s", entityID)
	}

	return nil
}

func deleteArtistImage(db *sqlx.DB, schema, artistID string) error {
	if err := deleteEntityImage(db, schema, artistEntity, artistID); err != nil {
		return fmt.Errorf("delete artiest afbeelding %s: %w", ErrSuffixFailed, err)
	}
	return nil
}

func deleteTrackImage(db *sqlx.DB, schema, trackID string) error {
	if err := deleteEntityImage(db, schema, trackEntity, trackID); err != nil {
		return fmt.Errorf("delete track afbeelding %s: %w", ErrSuffixFailed, err)
	}
	return nil
}

// buildPlaylistQuery creates SQL for playlist queries
func buildPlaylistQuery(schema string, opts PlaylistOptions) string {
	var conditions []string

	// Date filter
	if opts.Date != "" {
		conditions = append(conditions, fmt.Sprintf("DATE(pi.startdatetime) = '%s'", opts.Date))
	} else {
		conditions = append(conditions, "DATE(pi.startdatetime) = CURRENT_DATE")
	}

	// Time range filter
	if opts.StartTime != "" {
		conditions = append(conditions, fmt.Sprintf("TO_CHAR(pi.startdatetime, 'HH24:MI') >= '%s'", opts.StartTime))
	}
	if opts.EndTime != "" {
		conditions = append(conditions, fmt.Sprintf("TO_CHAR(pi.startdatetime, 'HH24:MI') <= '%s'", opts.EndTime))
	}

	// Item type filter
	if len(opts.ItemTypes) > 0 {
		types := make([]string, len(opts.ItemTypes))
		for i, t := range opts.ItemTypes {
			types[i] = fmt.Sprintf("%d", t)
		}
		conditions = append(conditions, fmt.Sprintf("pi.itemtype IN (%s)", strings.Join(types, ",")))
	}

	// Export type filter
	if len(opts.ExportTypes) > 0 {
		types := make([]string, len(opts.ExportTypes))
		for i, t := range opts.ExportTypes {
			types[i] = fmt.Sprintf("%d", t)
		}
		conditions = append(conditions, fmt.Sprintf("COALESCE(t.exporttype, 1) NOT IN (%s)", strings.Join(types, ",")))
	}

	// Track image filter
	if opts.TrackImage != nil {
		if *opts.TrackImage {
			conditions = append(conditions, "t.picture IS NOT NULL")
		} else {
			conditions = append(conditions, "t.picture IS NULL")
		}
	}

	// Artist image filter
	if opts.ArtistImage != nil {
		if *opts.ArtistImage {
			conditions = append(conditions, "a.picture IS NOT NULL")
		} else {
			conditions = append(conditions, "a.picture IS NULL")
		}
	}

	// Build WHERE clause
	whereClause := strings.Join(conditions, " AND ")

	// Sort order
	orderBy := "pi.startdatetime"
	switch opts.SortBy {
	case "artist":
		orderBy = "t.artist"
	case "track":
		orderBy = "t.tracktitle"
	}
	if opts.SortDesc {
		orderBy += " DESC"
	}

	query := fmt.Sprintf(`
		SELECT 
			pi.titleid as songid,
			COALESCE(t.tracktitle, '') as songname,
			COALESCE(t.artistid, '00000000-0000-0000-0000-000000000000') as artistid,
			COALESCE(t.artist, '') as artistname,
			TO_CHAR(pi.startdatetime, 'HH24:MI:SS') as start_time,
			TO_CHAR(pi.startdatetime + INTERVAL '1 millisecond' * COALESCE(t.knownlength, 0), 'HH24:MI:SS') as end_time,
			COALESCE(t.knownlength, 0) as duration,
			CASE WHEN t.picture IS NOT NULL THEN true ELSE false END as has_track_image,
			CASE WHEN a.picture IS NOT NULL THEN true ELSE false END as has_artist_image
		FROM %s.playlistitem pi
		LEFT JOIN %s.track t ON pi.titleid = t.titleid
		LEFT JOIN %s.artist a ON t.artistid = a.artistid
		WHERE %s
		ORDER BY %s
	`, schema, schema, schema, whereClause, orderBy)

	// Add limit/offset
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	}

	return query
}

// executePlaylistQuery runs query and returns items
func executePlaylistQuery(db *sqlx.DB, query string) ([]PlaylistItem, error) {
	var items []PlaylistItem
	err := db.Select(&items, query)
	if err != nil {
		return nil, fmt.Errorf("playlist ophalen mislukt: %w", err)
	}

	return items, nil
}

func getPlaylist(db *sqlx.DB, schema string, opts PlaylistOptions) ([]PlaylistItem, error) {
	query := buildPlaylistQuery(schema, opts)
	return executePlaylistQuery(db, query)
}

const getArtistDetailsQuery = `
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

func getArtistByID(db *sqlx.DB, schema, artistID string) (*ArtistDetails, error) {
	query := fmt.Sprintf(getArtistDetailsQuery, schema)

	var artist ArtistDetails
	err := db.Get(&artist, query, artistID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artiest ID '%s' %s", artistID, ErrSuffixNotExists)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}

	return &artist, nil
}

const getTrackDetailsQuery = `
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

func getTrackByID(db *sqlx.DB, schema, trackID string) (*TrackDetails, error) {
	query := fmt.Sprintf(getTrackDetailsQuery, schema)

	var track TrackDetails
	err := db.Get(&track, query, trackID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("track ID '%s' %s", trackID, ErrSuffixNotExists)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}

	return &track, nil
}

// Unified image retrieval using direct queries
func getEntityImage(db *sqlx.DB, schema, entityID string, entity EntityConfig) ([]byte, error) {
	query := fmt.Sprintf("SELECT picture FROM %s.%s WHERE %s = $1", schema, entity.Table, entity.IDColumn)

	var imageData []byte
	err := db.Get(&imageData, query, entityID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s ID '%s' %s", entity.IDName, entityID, ErrSuffixNotExists)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}
	if imageData == nil {
		return nil, fmt.Errorf("%s heeft geen afbeelding", entity.IDName)
	}

	return imageData, nil
}

// Convenience wrappers using the smart function
func getArtistImage(db *sqlx.DB, schema, artistID string) ([]byte, error) {
	return getEntityImage(db, schema, artistID, artistEntity)
}

func getTrackImage(db *sqlx.DB, schema, trackID string) ([]byte, error) {
	return getEntityImage(db, schema, trackID, trackEntity)
}
