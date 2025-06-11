package main

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

// Helper function for closing rows with error handling
func closeRows(rows *sql.Rows) {
	if rows != nil {
		_ = rows.Close()
	}
}

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
	SongID     string `json:"songid"`
	SongName   string `json:"songname"`
	ArtistID   string `json:"artistid"`
	ArtistName string `json:"artistname"`
	StartTime  string `json:"start_time"`
	HasImage   bool   `json:"has_image"`
}

// PlaylistOptions configures playlist queries
type PlaylistOptions struct {
	Date        string // Specific date (YYYY-MM-DD), empty = today
	StartTime   string // Filter from time (HH:MM)
	EndTime     string // Filter until time (HH:MM)
	ItemTypes   []int  // Item types to include (default: [1])
	ExportTypes []int  // Export types to exclude (default: [0])
	WithImages  *bool  // Filter by image presence (nil = all)
	Limit       int    // Max items to return (0 = all)
	Offset      int    // Pagination offset
	SortBy      string // Sort field (default: "starttime")
	SortDesc    bool   // Sort descending
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
		condition = "WHERE picture IS NOT NULL"
	} else {
		condition = "WHERE picture IS NULL"
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s %s", schema, table, condition)

	var count int
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("telfout %s: %w", table, err)
	}

	return count, nil
}

func lookupArtist(db *sql.DB, schema, artistName, artistID string) (*Artist, error) {
	var query string
	var searchValue string
	var scanID, scanName string
	var hasImage bool

	if artistID != "" {
		query = fmt.Sprintf(`SELECT artistid, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		                     FROM %s.artist WHERE artistid = $1`, schema)
		searchValue = artistID
	} else {
		query = fmt.Sprintf(`SELECT artistid, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		                     FROM %s.artist WHERE artist = $1`, schema)
		searchValue = artistName
	}

	err := db.QueryRow(query, searchValue).Scan(&scanID, &scanName, &hasImage)

	if err == sql.ErrNoRows {
		if artistID != "" {
			return nil, fmt.Errorf("artiest ID '%s' bestaat niet", artistID)
		}
		return nil, fmt.Errorf("artiest '%s' bestaat niet", artistName)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}

	return &Artist{
		ID:       scanID,
		Name:     scanName,
		HasImage: hasImage,
	}, nil
}

func updateArtistImage(db *sql.DB, schema, artistID string, imageData []byte) error {
	query := fmt.Sprintf(`UPDATE %s.artist SET picture = $1 WHERE artistid = $2`, schema)
	_, err := db.Exec(query, imageData, artistID)
	if err != nil {
		return fmt.Errorf("update artiest mislukt: %w", err)
	}
	return nil
}

func lookupTrack(db *sql.DB, schema, trackTitle, trackID string) (*Track, error) {
	var query string
	var searchValue string
	var scanID, scanTitle, scanArtist string
	var hasImage bool

	if trackID != "" {
		query = fmt.Sprintf(`SELECT titleid, tracktitle, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		                     FROM %s.track WHERE titleid = $1`, schema)
		searchValue = trackID
	} else {
		query = fmt.Sprintf(`SELECT titleid, tracktitle, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		                     FROM %s.track WHERE tracktitle = $1`, schema)
		searchValue = trackTitle
	}

	err := db.QueryRow(query, searchValue).Scan(&scanID, &scanTitle, &scanArtist, &hasImage)

	if err == sql.ErrNoRows {
		if trackID != "" {
			return nil, fmt.Errorf("track ID '%s' bestaat niet", trackID)
		}
		return nil, fmt.Errorf("track '%s' bestaat niet", trackTitle)
	}
	if err != nil {
		return nil, fmt.Errorf("database fout: %w", err)
	}

	return &Track{
		ID:       scanID,
		Title:    scanTitle,
		Artist:   scanArtist,
		HasImage: hasImage,
	}, nil
}

func updateTrackImage(db *sql.DB, schema, trackID string, imageData []byte) error {
	query := fmt.Sprintf(`UPDATE %s.track SET picture = $1 WHERE titleid = $2`, schema)
	_, err := db.Exec(query, imageData, trackID)
	if err != nil {
		return fmt.Errorf("update track mislukt: %w", err)
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

	// Image filter
	if opts.WithImages != nil {
		if *opts.WithImages {
			conditions = append(conditions, "t.picture IS NOT NULL")
		} else {
			conditions = append(conditions, "t.picture IS NULL")
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
			CASE WHEN t.picture IS NOT NULL THEN true ELSE false END as has_image
		FROM %s.playlistitem pi
		LEFT JOIN %s.track t ON pi.titleid = t.titleid
		WHERE %s
		ORDER BY %s
	`, schema, schema, whereClause, orderBy)

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
		&item.ArtistName, &item.StartTime, &item.HasImage)
	return item, err
}

// executePlaylistQuery runs query and returns items
func executePlaylistQuery(db *sql.DB, query string) ([]PlaylistItem, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("playlist ophalen mislukt: %w", err)
	}
	defer closeRows(rows)

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

func getTodayPlaylist(db *sql.DB, schema string) ([]PlaylistItem, error) {
	opts := defaultPlaylistOptions()
	return getPlaylist(db, schema, opts)
}

func getPlaylist(db *sql.DB, schema string, opts PlaylistOptions) ([]PlaylistItem, error) {
	query := buildPlaylistQuery(schema, opts)
	return executePlaylistQuery(db, query)
}
