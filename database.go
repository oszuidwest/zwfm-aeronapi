package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Artist represents a basic artist entity from the database.
// It contains minimal information used for listing operations.
type Artist struct {
	ID       string `db:"artistid"`  // UUID of the artist
	Name     string `db:"artist"`    // Artist name
	HasImage bool   `db:"has_image"` // Whether the artist has an associated image
}

// ArtistDetails represents complete artist information from the database.
// It includes all metadata fields available for an artist entity.
type ArtistDetails struct {
	ID          string `db:"artistid" json:"artistid"`         // UUID of the artist
	Name        string `db:"artist" json:"artist"`             // Artist name
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
	ID       string `db:"titleid"`    // UUID of the track
	Title    string `db:"tracktitle"` // Track title
	Artist   string `db:"artist"`     // Artist name
	HasImage bool   `db:"has_image"`  // Whether the track has an associated image
}

// TrackDetails represents complete track information from the database.
// It includes all metadata fields available for a track entity.
type TrackDetails struct {
	ID          string `db:"titleid" json:"titleid"`           // UUID of the track
	Title       string `db:"tracktitle" json:"tracktitle"`     // Track title
	Artist      string `db:"artist" json:"artist"`             // Artist name
	ArtistID    string `db:"artistid" json:"artistid"`         // UUID of the associated artist
	Year        int    `db:"year" json:"year"`                 // Release year
	KnownLength int    `db:"knownlength" json:"knownlength"`   // Track length in milliseconds
	IntroTime   int    `db:"introtime" json:"introtime"`       // Intro length in milliseconds
	OutroTime   int    `db:"outrotime" json:"outrotime"`       // Outro length in milliseconds
	Tempo       int    `db:"tempo" json:"tempo"`               // Tempo classification
	BPM         int    `db:"bpm" json:"bpm"`                   // Beats per minute
	Gender      int    `db:"gender" json:"gender"`             // Vocalist gender classification
	Language    int    `db:"language" json:"language"`         // Language classification
	Mood        int    `db:"mood" json:"mood"`                 // Mood classification
	ExportType  int    `db:"exporttype" json:"exporttype"`     // Export type (2 = excluded from operations)
	RepeatValue int    `db:"repeat_value" json:"repeat_value"` // Repeat restriction value
	Rating      int    `db:"rating" json:"rating"`             // Track rating
	HasImage    bool   `db:"has_image" json:"has_image"`       // Whether the track has an associated image
	Website     string `db:"website" json:"website"`           // Related website URL
	Conductor   string `db:"conductor" json:"conductor"`       // Conductor name (for classical music)
	Orchestra   string `db:"orchestra" json:"orchestra"`       // Orchestra name (for classical music)
}

// PlaylistBlock represents a programming block in the Aeron playlist system.
// Blocks group playlist items by time periods (e.g., morning show, afternoon music).
type PlaylistBlock struct {
	BlockID   string `db:"blockid" json:"blockid"`       // UUID of the playlist block
	Name      string `db:"name" json:"name"`             // Block name (e.g., "Morning Show")
	StartTime string `db:"start_time" json:"start_time"` // Block start time (HH:MM:SS format)
	EndTime   string `db:"end_time" json:"end_time"`     // Block end time (HH:MM:SS format)
	Date      string `db:"date" json:"date"`             // Block date (YYYY-MM-DD format)
}

// PlaylistItem represents a single item in the Aeron playlist.
// Items can be music tracks, voice tracks, commercials, or other content.
type PlaylistItem struct {
	SongID         string `db:"songid" json:"songid"`                     // UUID of the track
	SongName       string `db:"songname" json:"songname"`                 // Track title
	ArtistID       string `db:"artistid" json:"artistid"`                 // UUID of the artist
	ArtistName     string `db:"artistname" json:"artistname"`             // Artist name
	StartTime      string `db:"start_time" json:"start_time"`             // Scheduled start time (HH:MM:SS format)
	EndTime        string `db:"end_time" json:"end_time"`                 // Calculated end time (HH:MM:SS format)
	Duration       int    `db:"duration" json:"duration"`                 // Duration in milliseconds
	HasTrackImage  bool   `db:"has_track_image" json:"has_track_image"`   // Whether the track has an image
	HasArtistImage bool   `db:"has_artist_image" json:"has_artist_image"` // Whether the artist has an image
	ExportType     int    `db:"exporttype" json:"exporttype"`             // Export type classification
	Mode           int    `db:"mode" json:"mode"`                         // Playback mode
	IsVoicetrack   bool   `db:"is_voicetrack" json:"is_voicetrack"`       // Whether this is a voice track
	IsCommblock    bool   `db:"is_commblock" json:"is_commblock"`         // Whether this is a commercial block
}

// PlaylistOptions configures playlist queries with filtering and pagination options.
// It provides flexible control over which playlist items are returned.
type PlaylistOptions struct {
	BlockID     string // Filter by specific block ID (required for playlist items)
	Date        string // Specific date (YYYY-MM-DD) for blocks endpoint
	ExportTypes []int  // Export types to exclude from results
	Limit       int    // Max items to return (0 = all)
	Offset      int    // Pagination offset
	SortBy      string // Sort field (default: "starttime")
	SortDesc    bool   // Sort descending
	// Image filters
	TrackImage  *bool // Filter by track image: true (has), false (no), nil (all)
	ArtistImage *bool // Filter by artist image: true (has), false (no), nil (all)
}

// defaultPlaylistOptions returns default playlist options with sensible defaults.
// It excludes no export types and sorts by start time in ascending order.
func defaultPlaylistOptions() PlaylistOptions {
	return PlaylistOptions{
		ExportTypes: []int{},
		SortBy:      "starttime",
	}
}

// countItems counts entities in the specified table based on image presence.
// It returns the number of entities that either have or don't have images.
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

// updateEntityImage updates the image data for either an artist or track entity.
// The table parameter determines whether to update the artist or track table.
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

// deleteEntityImage removes the image data for either an artist or track entity.
// It sets the picture column to NULL and returns an error if the entity doesn't exist.
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
		return fmt.Errorf("%s met ID '%s' %s", entityType, id, ErrSuffixNotExists)
	}

	return nil
}

// isValidSchemaName validates that a schema name contains only safe characters.
// It prevents SQL injection by allowing only alphanumeric characters and underscores.
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

// buildPlaylistQuery creates a parameterized SQL query for playlist items.
// It builds the query based on the provided options and returns the query string and parameters.
func buildPlaylistQuery(schema string, opts PlaylistOptions) (string, []interface{}) {
	var conditions []string
	var params []interface{}
	paramCount := 0

	// Helper function to get next parameter placeholder
	nextParam := func() string {
		paramCount++
		return fmt.Sprintf("$%d", paramCount)
	}

	// Block filter is required - always use block-centric approach
	if opts.BlockID != "" {
		conditions = append(conditions, fmt.Sprintf("pi.blockid = %s", nextParam()))
		params = append(params, opts.BlockID)
	} else {
		// If no block specified, return empty result
		return "", []interface{}{}
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
			COALESCE(t.exporttype, 0) as exporttype,
			COALESCE(pi.mode, 0) as mode,
			CASE WHEN t.userid = '021F097E-B504-49BB-9B89-16B64D2E8422' THEN true ELSE false END as is_voicetrack,
			CASE WHEN COALESCE(pi.commblock, 0) > 0 THEN true ELSE false END as is_commblock
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

// executePlaylistQuery executes a parameterized playlist query and returns the results.
// It takes a prepared query string and its parameters, returning playlist items.
func executePlaylistQuery(db *sqlx.DB, query string, params []interface{}) ([]PlaylistItem, error) {
	var items []PlaylistItem
	err := db.Select(&items, query, params...)
	if err != nil {
		return nil, fmt.Errorf("ophalen van playlist mislukt: %w", err)
	}

	return items, nil
}

// getPlaylist retrieves playlist items from the database based on the provided options.
// It builds and executes a query filtered by the playlist options.
func getPlaylist(db *sqlx.DB, schema string, opts PlaylistOptions) ([]PlaylistItem, error) {
	query, params := buildPlaylistQuery(schema, opts)
	return executePlaylistQuery(db, query, params)
}

// getPlaylistBlocks retrieves all playlist blocks for a specific date.
// If no date is provided, it returns blocks for the current date.
func getPlaylistBlocks(db *sqlx.DB, schema string, date string) ([]PlaylistBlock, error) {
	var dateFilter string
	params := []interface{}{}

	if date != "" {
		// Use range query for better index usage
		dateFilter = "pb.startdatetime >= $1::date AND pb.startdatetime < $1::date + INTERVAL '1 day'"
		params = append(params, date)
	} else {
		// Use range query for current date
		dateFilter = "pb.startdatetime >= CURRENT_DATE AND pb.startdatetime < CURRENT_DATE + INTERVAL '1 day'"
	}

	query := fmt.Sprintf(`
		SELECT
			pb.blockid,
			COALESCE(pb.name, '') as name,
			DATE(pb.startdatetime)::text as date,
			TO_CHAR(pb.startdatetime, 'HH24:MI:SS') as start_time,
			TO_CHAR(pb.enddatetime, 'HH24:MI:SS') as end_time
		FROM %s.playlistblock pb
		WHERE %s
		ORDER BY pb.startdatetime
	`, schema, dateFilter)

	var blocks []PlaylistBlock
	err := db.Select(&blocks, query, params...)
	if err != nil {
		return nil, fmt.Errorf("ophalen van playlist blocks mislukt: %w", err)
	}

	return blocks, nil
}

// getPlaylistBlocksWithTracks efficiently fetches all blocks and their tracks for a date.
// It uses only 2 database queries to retrieve all data and returns blocks with their associated tracks.
func getPlaylistBlocksWithTracks(db *sqlx.DB, schema string, date string) ([]PlaylistBlock, map[string][]PlaylistItem, error) {
	// First get all blocks
	blocks, err := getPlaylistBlocks(db, schema, date)
	if err != nil {
		return nil, nil, err
	}

	if len(blocks) == 0 {
		return blocks, make(map[string][]PlaylistItem), nil
	}

	// Collect all block IDs
	blockIDs := make([]string, len(blocks))
	for i, block := range blocks {
		blockIDs[i] = block.BlockID
	}

	// Build the query for all tracks in all blocks
	var dateFilter string
	params := []interface{}{}
	paramCount := 0

	if date != "" {
		// Use range query for better index usage
		dateFilter = "pi.startdatetime >= $1::date AND pi.startdatetime < $1::date + INTERVAL '1 day'"
		params = append(params, date)
		paramCount = 1
	} else {
		// Use range query for current date
		dateFilter = "pi.startdatetime >= CURRENT_DATE AND pi.startdatetime < CURRENT_DATE + INTERVAL '1 day'"
	}

	// Build placeholders for block IDs
	placeholders := make([]string, len(blockIDs))
	for i, id := range blockIDs {
		paramCount++
		placeholders[i] = fmt.Sprintf("$%d", paramCount)
		params = append(params, id)
	}

	// Create a temporary struct that includes blockid for grouping
	type tempPlaylistItem struct {
		PlaylistItem
		TempBlockID string `db:"blockid"`
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
			COALESCE(t.exporttype, 0) as exporttype,
			COALESCE(pi.mode, 0) as mode,
			CASE WHEN t.userid = '021F097E-B504-49BB-9B89-16B64D2E8422' THEN true ELSE false END as is_voicetrack,
			CASE WHEN COALESCE(pi.commblock, 0) > 0 THEN true ELSE false END as is_commblock,
			COALESCE(pi.blockid::text, '') as blockid
		FROM %s.playlistitem pi
		LEFT JOIN %s.track t ON pi.titleid = t.titleid
		LEFT JOIN %s.artist a ON t.artistid = a.artistid
		WHERE %s AND pi.blockid IN (%s)
		ORDER BY pi.blockid, pi.startdatetime
	`, schema, schema, schema, dateFilter, strings.Join(placeholders, ","))

	var tempItems []tempPlaylistItem
	err = db.Select(&tempItems, query, params...)
	if err != nil {
		return nil, nil, fmt.Errorf("ophalen van playlist items mislukt: %w", err)
	}

	// Group items by block ID
	tracksByBlock := make(map[string][]PlaylistItem)
	for _, temp := range tempItems {
		tracksByBlock[temp.TempBlockID] = append(tracksByBlock[temp.TempBlockID], temp.PlaylistItem)
	}

	return blocks, tracksByBlock, nil
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

// getArtistByID retrieves complete artist details by UUID.
// It returns an error if the artist doesn't exist in the database.
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

// getTrackByID retrieves complete track details by UUID.
// It returns an error if the track doesn't exist in the database.
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

// getEntityImage retrieves the image data for either an artist or track entity.
// It returns the raw image bytes or an error if the entity doesn't exist or has no image.
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
