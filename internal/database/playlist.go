// Package database provides PostgreSQL data access for the Aeron database.
package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
)

// PlaylistBlock represents a programming block in the Aeron playlist system.
type PlaylistBlock struct {
	BlockID        string `db:"blockid" json:"blockid"`
	Name           string `db:"name" json:"name"`
	StartTimeOfDay string `db:"start_time" json:"start_time"`
	EndTimeOfDay   string `db:"end_time" json:"end_time"`
	Date           string `db:"date" json:"date"`
}

// PlaylistItem represents a single item in the Aeron playlist.
type PlaylistItem struct {
	TrackID        string `db:"trackid" json:"trackid"`
	TrackTitle     string `db:"tracktitle" json:"tracktitle"`
	ArtistID       string `db:"artistid" json:"artistid"`
	ArtistName     string `db:"artistname" json:"artistname"`
	StartTime      string `db:"start_time" json:"start_time"`
	EndTime        string `db:"end_time" json:"end_time"`
	Duration       int    `db:"duration" json:"duration"`
	HasTrackImage  bool   `db:"has_track_image" json:"has_track_image"`
	HasArtistImage bool   `db:"has_artist_image" json:"has_artist_image"`
	ExportType     int    `db:"exporttype" json:"exporttype"`
	Mode           int    `db:"mode" json:"mode"`
	IsVoicetrack   bool   `db:"is_voicetrack" json:"is_voicetrack"`
	IsCommblock    bool   `db:"is_commblock" json:"is_commblock"`
}

// PlaylistOptions configures playlist queries with filtering and pagination.
type PlaylistOptions struct {
	BlockID     string
	Date        string
	ExportTypes []int
	Limit       int
	Offset      int
	SortBy      string
	SortDesc    bool
	TrackImage  *bool
	ArtistImage *bool
}

// DefaultPlaylistOptions returns default playlist query options.
func DefaultPlaylistOptions() PlaylistOptions {
	return PlaylistOptions{
		ExportTypes: []int{},
		SortBy:      "starttime",
	}
}

// BuildPlaylistQuery creates a parameterized SQL query for playlist items.
func BuildPlaylistQuery(schema string, opts PlaylistOptions) (string, []any, error) {
	var conditions []string
	var params []any
	paramCount := 0

	nextParam := func() string {
		paramCount++
		return fmt.Sprintf("$%d", paramCount)
	}

	if opts.BlockID != "" {
		conditions = append(conditions, fmt.Sprintf("pi.blockid = %s", nextParam()))
		params = append(params, opts.BlockID)
	} else {
		return "", []any{}, nil
	}

	if len(opts.ExportTypes) > 0 {
		placeholders := make([]string, len(opts.ExportTypes))
		for i, t := range opts.ExportTypes {
			placeholders[i] = nextParam()
			params = append(params, t)
		}
		conditions = append(conditions, fmt.Sprintf("COALESCE(t.exporttype, 1) NOT IN (%s)", strings.Join(placeholders, ",")))
	}

	if opts.TrackImage != nil {
		if *opts.TrackImage {
			conditions = append(conditions, "t.picture IS NOT NULL")
		} else {
			conditions = append(conditions, "t.picture IS NULL")
		}
	}

	if opts.ArtistImage != nil {
		if *opts.ArtistImage {
			conditions = append(conditions, "a.picture IS NOT NULL")
		} else {
			conditions = append(conditions, "a.picture IS NULL")
		}
	}

	whereClause := strings.Join(conditions, " AND ")

	orderBy := "pi.startdatetime"
	switch opts.SortBy {
	case "artist":
		orderBy = "t.artist"
	case "track":
		orderBy = "t.tracktitle"
	case "start_time":
		orderBy = "pi.startdatetime"
	}
	if opts.SortDesc {
		orderBy += " DESC"
	}

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

// ExecutePlaylistQuery executes a playlist query and returns items.
func ExecutePlaylistQuery(ctx context.Context, db DB, query string, params []any) ([]PlaylistItem, error) {
	var items []PlaylistItem
	err := db.SelectContext(ctx, &items, query, params...)
	if err != nil {
		return nil, &types.DatabaseError{Operation: "ophalen van playlist", Err: err}
	}

	return items, nil
}

// GetPlaylist retrieves playlist items based on the provided options.
func GetPlaylist(ctx context.Context, db DB, schema string, opts PlaylistOptions) ([]PlaylistItem, error) {
	query, params, err := BuildPlaylistQuery(schema, opts)
	if err != nil {
		return nil, err
	}
	return ExecutePlaylistQuery(ctx, db, query, params)
}

// GetPlaylistBlocks retrieves all playlist blocks for a specific date.
func GetPlaylistBlocks(ctx context.Context, db DB, schema string, date string) ([]PlaylistBlock, error) {
	var dateFilter string
	params := []any{}

	if date != "" {
		dateFilter = "pb.startdatetime >= $1::date AND pb.startdatetime < $1::date + INTERVAL '1 day'"
		params = append(params, date)
	} else {
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

// GetPlaylistBlocksWithTracks fetches all blocks and their tracks for a date.
func GetPlaylistBlocksWithTracks(ctx context.Context, db DB, schema string, date string) ([]PlaylistBlock, map[string][]PlaylistItem, error) {
	blocks, err := GetPlaylistBlocks(ctx, db, schema, date)
	if err != nil {
		return nil, nil, err
	}

	if len(blocks) == 0 {
		return blocks, make(map[string][]PlaylistItem), nil
	}

	blockIDs := make([]string, len(blocks))
	for i, block := range blocks {
		blockIDs[i] = block.BlockID
	}

	var dateFilter string
	params := []any{}
	paramCount := 0

	if date != "" {
		dateFilter = "pi.startdatetime >= $1::date AND pi.startdatetime < $1::date + INTERVAL '1 day'"
		params = append(params, date)
		paramCount = 1
	} else {
		dateFilter = "pi.startdatetime >= CURRENT_DATE AND pi.startdatetime < CURRENT_DATE + INTERVAL '1 day'"
	}
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

	tracksByBlock := make(map[string][]PlaylistItem)
	for _, temp := range tempItems {
		tracksByBlock[temp.TempBlockID] = append(tracksByBlock[temp.TempBlockID], temp.PlaylistItem)
	}

	return blocks, tracksByBlock, nil
}
