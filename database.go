package main

import (
	"database/sql"
	"fmt"
	"strings"

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

type Artist struct {
	ID       string
	Name     string
	HasImage bool
}

type Track struct {
	ID       string
	Title    string
	Artist   string
	HasImage bool
}

type PlaylistItem struct {
	SongID         string `json:"songid"`
	SongName       string `json:"songname"`
	ArtistID       string `json:"artistid"`
	ArtistName     string `json:"artistname"`
	StartTime      string `json:"start_time"`
	HasTrackImage  bool   `json:"has_track_image"`
	HasArtistImage bool   `json:"has_artist_image"`
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

func countItems(db *sql.DB, schema, table string, hasImage bool) (int, error) {
	var condition string
	if hasImage {
		condition = fmt.Sprintf("WHERE %s IS NOT NULL", columnPicture)
	} else {
		condition = fmt.Sprintf("WHERE %s IS NULL", columnPicture)
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s %s", schema, table, condition)

	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("telfout %s: %w", table, err)
	}

	return count, nil
}

// genericLookup handles common database lookup logic
// genericLookup handles common database lookup logic
func genericLookup(db *sql.DB, schema, table, idColumn, nameColumn, idValue, nameValue string) (string, string, bool, error) {
	var query string
	var searchValue string
	var scanID, scanName string
	var hasImage bool

	if idValue != "" {
		query = fmt.Sprintf(`SELECT %s, %s, CASE WHEN %s IS NOT NULL THEN true ELSE false END as has_image 
		                     FROM %s.%s WHERE %s = $1`, idColumn, nameColumn, columnPicture, schema, table, idColumn)
		searchValue = idValue
	} else {
		query = fmt.Sprintf(`SELECT %s, %s, CASE WHEN %s IS NOT NULL THEN true ELSE false END as has_image 
		                     FROM %s.%s WHERE %s = $1`, idColumn, nameColumn, columnPicture, schema, table, nameColumn)
		searchValue = nameValue
	}

	err := db.QueryRow(query, searchValue).Scan(&scanID, &scanName, &hasImage)
	if err != nil {
		return "", "", false, err
	}

	return scanID, scanName, hasImage, nil
}

func lookupArtist(db *sql.DB, schema, artistName, artistID string) (*Artist, error) {
	id, name, hasImage, err := genericLookup(db, schema, tableArtist, columnArtistID, columnArtist, artistID, artistName)

	if err == sql.ErrNoRows {
		if artistID != "" {
			return nil, fmt.Errorf("artiest ID '%s' %s", artistID, ErrSuffixNotExists)
		}
		return nil, fmt.Errorf("artiest '%s' %s", artistName, ErrSuffixNotExists)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}

	return &Artist{
		ID:       id,
		Name:     name,
		HasImage: hasImage,
	}, nil
}

// genericUpdateImage handles common image update logic
func genericUpdateImage(db *sql.DB, schema, table, idColumn, idValue string, imageData []byte) error {
	query := fmt.Sprintf(`UPDATE %s.%s SET %s = $1 WHERE %s = $2`, schema, table, columnPicture, idColumn)
	_, err := db.Exec(query, imageData, idValue)
	return err
}

func updateArtistImage(db *sql.DB, schema, artistID string, imageData []byte) error {
	if err := genericUpdateImage(db, schema, tableArtist, columnArtistID, artistID, imageData); err != nil {
		return fmt.Errorf("update artiest %s: %w", ErrSuffixFailed, err)
	}
	return nil
}

func lookupTrack(db *sql.DB, schema, trackTitle, trackID string) (*Track, error) {
	// First do the main lookup
	id, title, hasImage, err := genericLookup(db, schema, tableTrack, columnTitleID, "tracktitle", trackID, trackTitle)

	if err == sql.ErrNoRows {
		if trackID != "" {
			return nil, fmt.Errorf("track ID '%s' %s", trackID, ErrSuffixNotExists)
		}
		return nil, fmt.Errorf("track '%s' %s", trackTitle, ErrSuffixNotExists)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}

	// Need to get artist separately for tracks
	var artist string
	query := fmt.Sprintf("SELECT %s FROM %s.%s WHERE %s = $1", columnArtist, schema, tableTrack, columnTitleID)
	err = db.QueryRow(query, id).Scan(&artist)
	if err != nil {
		return nil, fmt.Errorf("artiest ophalen %s: %w", ErrSuffixFailed, err)
	}

	return &Track{
		ID:       id,
		Title:    title,
		Artist:   artist,
		HasImage: hasImage,
	}, nil
}

func updateTrackImage(db *sql.DB, schema, trackID string, imageData []byte) error {
	if err := genericUpdateImage(db, schema, tableTrack, columnTitleID, trackID, imageData); err != nil {
		return fmt.Errorf("update track %s: %w", ErrSuffixFailed, err)
	}
	return nil
}

func countOrphanedArtists(db *sql.DB, schema string) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(DISTINCT a.artistid) 
		FROM %s.artist a
		LEFT JOIN %s.track t ON a.artist = t.artist
		WHERE t.titleid IS NULL
	`, schema, schema)

	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("telfout wees-artiesten: %w", err)
	}
	return count, nil
}

func countOrphanedTracks(db *sql.DB, schema string) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(DISTINCT t.titleid) 
		FROM %s.track t
		LEFT JOIN %s.artist a ON t.artist = a.artist
		WHERE a.artistid IS NULL
	`, schema, schema)

	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("telfout wees-tracks: %w", err)
	}
	return count, nil
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

// scanPlaylistRow converts database row to PlaylistItem
func scanPlaylistRow(rows *sql.Rows) (PlaylistItem, error) {
	var item PlaylistItem
	err := rows.Scan(&item.SongID, &item.SongName, &item.ArtistID,
		&item.ArtistName, &item.StartTime, &item.HasTrackImage, &item.HasArtistImage)
	return item, err
}

// executePlaylistQuery runs query and returns items
func executePlaylistQuery(db *sql.DB, query string) ([]PlaylistItem, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("playlist ophalen mislukt: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []PlaylistItem
	for rows.Next() {
		item, err := scanPlaylistRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func getPlaylist(db *sql.DB, schema string, opts PlaylistOptions) ([]PlaylistItem, error) {
	query := buildPlaylistQuery(schema, opts)
	return executePlaylistQuery(db, query)
}
