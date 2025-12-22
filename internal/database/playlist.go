// Package database provides PostgreSQL data access for the Aeron database.
package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
)

// PlaylistBlock represents a programming block in the Aeron playlist system.
// Blocks group playlist items by time periods (e.g., morning show, afternoon music).
type PlaylistBlock struct {
	BlockID        string `db:"blockid" json:"blockid"`       // UUID of the playlist block
	Name           string `db:"name" json:"name"`             // Block name (e.g., "Morning Show")
	StartTimeOfDay string `db:"start_time" json:"start_time"` // Block start time of day (HH:MM:SS format)
	EndTimeOfDay   string `db:"end_time" json:"end_time"`     // Block end time of day (HH:MM:SS format)
	Date           string `db:"date" json:"date"`             // Block date (YYYY-MM-DD format)
}

// PlaylistItem represents a single item in the Aeron playlist.
// Items can be music tracks, voice tracks, commercials, or other content.
type PlaylistItem struct {
	TrackID        string `db:"trackid" json:"trackid"`                   // UUID of the track
	TrackTitle     string `db:"tracktitle" json:"tracktitle"`             // Track title
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

// DefaultPlaylistOptions returns default playlist options with sensible defaults.
// It excludes no export types and sorts by start time in ascending order.
func DefaultPlaylistOptions() PlaylistOptions {
	return PlaylistOptions{
		ExportTypes: []int{},
		SortBy:      "starttime",
	}
}

// BuildPlaylistQuery creates a parameterized SQL query for playlist items.
// It builds the query based on the provided options and returns the query string and parameters.
func BuildPlaylistQuery(schema string, opts PlaylistOptions) (string, []interface{}, error) {
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
		return "", []interface{}{}, nil
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
	if !types.IsValidIdentifier(schema) {
		return "", nil, types.NewValidationError("schema", fmt.Sprintf("ongeldige schema naam: %s", schema))
	}

	query := fmt.Sprintf(`
		SELECT
			pi.titleid as trackid,
			COALESCE(t.tracktitle, '') as tracktitle,
			COALESCE(t.artistid, '00000000-0000-0000-0000-000000000000') as artistid,
			COALESCE(t.artist, '') as artistname,
			TO_CHAR(pi.startdatetime, 'HH24:MI:SS') as start_time,
			TO_CHAR(pi.startdatetime + INTERVAL '1 millisecond' * COALESCE(t.knownlength, 0), 'HH24:MI:SS') as end_time,
			COALESCE(t.knownlength, 0) as duration,
			CASE WHEN t.picture IS NOT NULL THEN true ELSE false END as has_track_image,
			CASE WHEN a.picture IS NOT NULL THEN true ELSE false END as has_artist_image,
			COALESCE(t.exporttype, 0) as exporttype,
			COALESCE(pi.mode, 0) as mode,
			CASE WHEN t.userid = '%s' THEN true ELSE false END as is_voicetrack,
			CASE WHEN COALESCE(pi.commblock, 0) > 0 THEN true ELSE false END as is_commblock
		FROM %s.playlistitem pi
		LEFT JOIN %s.track t ON pi.titleid = t.titleid
		LEFT JOIN %s.artist a ON t.artistid = a.artistid
		WHERE %s
		ORDER BY %s
	`, types.VoicetrackUserID, schema, schema, schema, whereClause, orderBy)

	// Add limit/offset with parameters
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %s", nextParam())
		params = append(params, opts.Limit)
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %s", nextParam())
			params = append(params, opts.Offset)
		}
	}

	return query, params, nil
}

// ExecutePlaylistQuery executes a parameterized playlist query and returns the results.
// It takes a prepared query string and its parameters, returning playlist items.
func ExecutePlaylistQuery(ctx context.Context, db DB, query string, params []interface{}) ([]PlaylistItem, error) {
	var items []PlaylistItem
	err := db.SelectContext(ctx, &items, query, params...)
	if err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen van playlist", Err: err}
	}

	return items, nil
}

// GetPlaylist retrieves playlist items from the database based on the provided options.
// It builds and executes a query filtered by the playlist options.
func GetPlaylist(ctx context.Context, db DB, schema string, opts PlaylistOptions) ([]PlaylistItem, error) {
	query, params, err := BuildPlaylistQuery(schema, opts)
	if err != nil {
		return nil, err
	}
	return ExecutePlaylistQuery(ctx, db, query, params)
}

// GetPlaylistBlocks retrieves all playlist blocks for a specific date.
// If no date is provided, it returns blocks for the current date.
func GetPlaylistBlocks(ctx context.Context, db DB, schema string, date string) ([]PlaylistBlock, error) {
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
	err := db.SelectContext(ctx, &blocks, query, params...)
	if err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen van playlist blocks", Err: err}
	}

	return blocks, nil
}

// GetPlaylistBlocksWithTracks efficiently fetches all blocks and their tracks for a date.
// It uses only 2 database queries to retrieve all data and returns blocks with their associated tracks.
func GetPlaylistBlocksWithTracks(ctx context.Context, db DB, schema string, date string) ([]PlaylistBlock, map[string][]PlaylistItem, error) {
	// First get all blocks
	blocks, err := GetPlaylistBlocks(ctx, db, schema, date)
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
	type playlistItemWithBlockID struct {
		PlaylistItem
		TempBlockID string `db:"blockid"`
	}

	query := fmt.Sprintf(`
		SELECT
			pi.titleid as trackid,
			COALESCE(t.tracktitle, '') as tracktitle,
			COALESCE(t.artistid, '00000000-0000-0000-0000-000000000000') as artistid,
			COALESCE(t.artist, '') as artistname,
			TO_CHAR(pi.startdatetime, 'HH24:MI:SS') as start_time,
			TO_CHAR(pi.startdatetime + INTERVAL '1 millisecond' * COALESCE(t.knownlength, 0), 'HH24:MI:SS') as end_time,
			COALESCE(t.knownlength, 0) as duration,
			CASE WHEN t.picture IS NOT NULL THEN true ELSE false END as has_track_image,
			CASE WHEN a.picture IS NOT NULL THEN true ELSE false END as has_artist_image,
			COALESCE(t.exporttype, 0) as exporttype,
			COALESCE(pi.mode, 0) as mode,
			CASE WHEN t.userid = '%s' THEN true ELSE false END as is_voicetrack,
			CASE WHEN COALESCE(pi.commblock, 0) > 0 THEN true ELSE false END as is_commblock,
			COALESCE(pi.blockid::text, '') as blockid
		FROM %s.playlistitem pi
		LEFT JOIN %s.track t ON pi.titleid = t.titleid
		LEFT JOIN %s.artist a ON t.artistid = a.artistid
		WHERE %s AND pi.blockid IN (%s)
		ORDER BY pi.blockid, pi.startdatetime
	`, types.VoicetrackUserID, schema, schema, schema, dateFilter, strings.Join(placeholders, ","))

	var tempItems []playlistItemWithBlockID
	err = db.SelectContext(ctx, &tempItems, query, params...)
	if err != nil {
		return nil, nil, &types.DatabaseError{Operation: "ophalen van playlist items", Err: err}
	}

	// Group items by block ID
	tracksByBlock := make(map[string][]PlaylistItem)
	for _, temp := range tempItems {
		tracksByBlock[temp.TempBlockID] = append(tracksByBlock[temp.TempBlockID], temp.PlaylistItem)
	}

	return blocks, tracksByBlock, nil
}
