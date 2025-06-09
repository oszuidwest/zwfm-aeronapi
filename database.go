package main

import (
	"database/sql"
	"fmt"

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

func listArtists(db *sql.DB, schema string, hasImage bool, limit int) ([]Artist, error) {
	var condition string
	if hasImage {
		condition = "WHERE picture IS NOT NULL"
	} else {
		condition = "WHERE picture IS NULL"
	}

	var limitClause string
	if limit > 0 {
		limitClause = fmt.Sprintf("LIMIT %d", limit)
	}

	query := fmt.Sprintf(`SELECT artistid, artist FROM %s.artist 
	                      %s 
	                      ORDER BY artist 
	                      %s`, schema, condition, limitClause)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("kon artiesten niet opvragen: %w", err)
	}
	defer closeRows(rows)

	var artists []Artist

	for rows.Next() {
		var artist Artist
		if err := rows.Scan(&artist.ID, &artist.Name); err != nil {
			return nil, err
		}
		artist.HasImage = hasImage
		artists = append(artists, artist)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return artists, nil
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
		return 0, fmt.Errorf("kon %s niet tellen: %w", table, err)
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
			return nil, fmt.Errorf("artiest met ID '%s' niet gevonden", artistID)
		}
		return nil, fmt.Errorf("artiest '%s' niet gevonden", artistName)
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
		return fmt.Errorf("kon artiest afbeelding niet bijwerken: %w", err)
	}
	return nil
}

func scopeDesc(scope string) string {
	switch scope {
	case "artist":
		return "artiest"
	case "track":
		return "track"
	default:
		return "item"
	}
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
			return nil, fmt.Errorf("track met ID '%s' niet gevonden", trackID)
		}
		return nil, fmt.Errorf("track '%s' niet gevonden", trackTitle)
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
		return fmt.Errorf("kon track afbeelding niet bijwerken: %w", err)
	}
	return nil
}

func listTracks(db *sql.DB, schema string, hasImage bool, limit int) ([]Track, error) {
	var condition string
	if hasImage {
		condition = "WHERE picture IS NOT NULL"
	} else {
		condition = "WHERE picture IS NULL"
	}

	var limitClause string
	if limit > 0 {
		limitClause = fmt.Sprintf("LIMIT %d", limit)
	}

	query := fmt.Sprintf(`SELECT titleid, tracktitle, artist FROM %s.track 
	                      %s 
	                      ORDER BY artist, tracktitle 
	                      %s`, schema, condition, limitClause)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("kon tracks niet opvragen: %w", err)
	}
	defer closeRows(rows)

	var tracks []Track

	for rows.Next() {
		var track Track
		if err := rows.Scan(&track.ID, &track.Title, &track.Artist); err != nil {
			return nil, err
		}
		track.HasImage = hasImage
		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tracks, nil
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
		return 0, fmt.Errorf("kon orphaned artiesten niet tellen: %w", err)
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
		return 0, fmt.Errorf("kon orphaned tracks niet tellen: %w", err)
	}
	return count, nil
}
