package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
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
	ExportType     int    `db:"exporttype" json:"exporttype"`
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
		return 0, fmt.Errorf("tellen van %s mislukt: %w", table, err)
	}

	return count, nil
}

// updateEntityImage updates image data for either artist or track
func updateEntityImage(db *sqlx.DB, schema, table, id string, imageData []byte) error {
	var query string
	var entityType string

	if table == tableArtist {
		query = fmt.Sprintf("UPDATE %s.artist SET picture = $1 WHERE artistid = $2", schema)
		entityType = "artiest"
	} else {
		query = fmt.Sprintf("UPDATE %s.track SET picture = $1 WHERE titleid = $2", schema)
		entityType = "track"
	}

	_, err := db.Exec(query, imageData, id)
	if err != nil {
		return fmt.Errorf("bijwerken van %s %s: %w", entityType, ErrSuffixFailed, err)
	}
	return nil
}

// deleteEntityImage removes image data for either artist or track
func deleteEntityImage(db *sqlx.DB, schema, table, id string) error {
	var query string
	var entityType string

	if table == tableArtist {
		query = fmt.Sprintf("UPDATE %s.artist SET picture = NULL WHERE artistid = $1", schema)
		entityType = "artiestafbeelding"
	} else {
		query = fmt.Sprintf("UPDATE %s.track SET picture = NULL WHERE titleid = $1", schema)
		entityType = "trackafbeelding"
	}

	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("verwijderen van %s %s: %w", entityType, ErrSuffixFailed, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verwijderen van %s %s: %w", entityType, ErrSuffixFailed, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("verwijderen van %s %s: geen record gevonden met ID %s", entityType, ErrSuffixFailed, id)
	}

	return nil
}

// isValidSchemaName validates schema name to prevent SQL injection
func isValidSchemaName(schema string) bool {
	if schema == "" {
		return false
	}
	for _, r := range schema {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// buildPlaylistQuery creates SQL for playlist queries with parameters
func buildPlaylistQuery(schema string, opts PlaylistOptions) (string, []interface{}) {
	var conditions []string
	var params []interface{}
	paramCount := 0

	// Helper function to get next parameter placeholder
	nextParam := func() string {
		paramCount++
		return fmt.Sprintf("$%d", paramCount)
	}

	// Date filter
	if opts.Date != "" {
		conditions = append(conditions, fmt.Sprintf("DATE(pi.startdatetime) = %s", nextParam()))
		params = append(params, opts.Date)
	} else {
		conditions = append(conditions, "DATE(pi.startdatetime) = CURRENT_DATE")
	}

	// Time range filter
	if opts.StartTime != "" {
		conditions = append(conditions, fmt.Sprintf("TO_CHAR(pi.startdatetime, 'HH24:MI') >= %s", nextParam()))
		params = append(params, opts.StartTime)
	}
	if opts.EndTime != "" {
		conditions = append(conditions, fmt.Sprintf("TO_CHAR(pi.startdatetime, 'HH24:MI') <= %s", nextParam()))
		params = append(params, opts.EndTime)
	}

	// Item type filter
	if len(opts.ItemTypes) > 0 {
		placeholders := make([]string, len(opts.ItemTypes))
		for i, t := range opts.ItemTypes {
			placeholders[i] = nextParam()
			params = append(params, t)
		}
		conditions = append(conditions, fmt.Sprintf("pi.itemtype IN (%s)", strings.Join(placeholders, ",")))
	}

	// Export type filter
	if len(opts.ExportTypes) > 0 {
		placeholders := make([]string, len(opts.ExportTypes))
		for i, t := range opts.ExportTypes {
			placeholders[i] = nextParam()
			params = append(params, t)
		}
		conditions = append(conditions, fmt.Sprintf("COALESCE(t.exporttype, 1) NOT IN (%s)", strings.Join(placeholders, ",")))
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

	// Sort order - validate to prevent injection
	orderBy := "pi.startdatetime"
	switch opts.SortBy {
	case "artist":
		orderBy = "t.artist"
	case "track":
		orderBy = "t.tracktitle"
	case "start_time":
		orderBy = "pi.startdatetime"
		// Only allow whitelisted sort columns
	}
	if opts.SortDesc {
		orderBy += " DESC"
	}

	// Validate schema name to prevent SQL injection
	// Schema names can't be parameterized in PostgreSQL, so we validate instead
	if !isValidSchemaName(schema) {
		schema = "aeron" // fallback to default
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
			CASE WHEN a.picture IS NOT NULL THEN true ELSE false END as has_artist_image,
			COALESCE(t.exporttype, 0) as exporttype
		FROM %s.playlistitem pi
		LEFT JOIN %s.track t ON pi.titleid = t.titleid
		LEFT JOIN %s.artist a ON t.artistid = a.artistid
		WHERE %s
		ORDER BY %s
	`, schema, schema, schema, whereClause, orderBy)

	// Add limit/offset with parameters
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %s", nextParam())
		params = append(params, opts.Limit)
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %s", nextParam())
			params = append(params, opts.Offset)
		}
	}

	return query, params
}

// executePlaylistQuery runs query with parameters and returns items
func executePlaylistQuery(db *sqlx.DB, query string, params []interface{}) ([]PlaylistItem, error) {
	var items []PlaylistItem
	err := db.Select(&items, query, params...)
	if err != nil {
		return nil, fmt.Errorf("ophalen van playlist mislukt: %w", err)
	}

	return items, nil
}

func getPlaylist(db *sqlx.DB, schema string, opts PlaylistOptions) ([]PlaylistItem, error) {
	query, params := buildPlaylistQuery(schema, opts)
	return executePlaylistQuery(db, query, params)
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
		return nil, fmt.Errorf("databasefout: %w", err)
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
		return nil, fmt.Errorf("databasefout: %w", err)
	}

	return &track, nil
}

// getEntityImage retrieves image data for either artist or track
func getEntityImage(db *sqlx.DB, schema, table, id string) ([]byte, error) {
	var query string
	var entityType string

	if table == tableArtist {
		query = fmt.Sprintf("SELECT picture FROM %s.artist WHERE artistid = $1", schema)
		entityType = "artiest"
	} else {
		query = fmt.Sprintf("SELECT picture FROM %s.track WHERE titleid = $1", schema)
		entityType = "track"
	}

	var imageData []byte
	err := db.Get(&imageData, query, id)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s-ID '%s' %s", entityType, id, ErrSuffixNotExists)
	}
	if err != nil {
		return nil, fmt.Errorf("databasefout: %w", err)
	}
	if imageData == nil {
		return nil, fmt.Errorf("%s heeft geen afbeelding", entityType)
	}

	return imageData, nil
}
