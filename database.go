package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
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
	defer rows.Close()

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

func lookupArtist(db *sql.DB, schema, artistName, artistID string) (*Artist, error) {
	var query string
	var searchValue string
	var scanID, scanName string
	var hasImage bool

	if artistID != "" {
		// Lookup by ID
		query = fmt.Sprintf(`SELECT artistid, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		                     FROM %s.artist WHERE artistid = $1`, schema)
		searchValue = artistID
	} else {
		// Lookup by name
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

func updateArtistImageInDB(db *sql.DB, schema, artistID string, imageData []byte) error {
	query := fmt.Sprintf(`UPDATE %s.artist SET picture = $1 WHERE artistid = $2`, schema)
	_, err := db.Exec(query, imageData, artistID)
	if err != nil {
		return fmt.Errorf("kon artiest afbeelding niet bijwerken: %w", err)
	}
	return nil
}

func nukeAllImages(db *sql.DB, schema string, scope string, dryRun bool) error {
	var totalCount int

	// Count affected items based on scope
	if scope == "artist" {
		artists, err := listArtists(db, schema, true, 0)
		if err != nil {
			return fmt.Errorf("kon artiesten met afbeeldingen niet ophalen: %w", err)
		}
		totalCount = len(artists)
	} else if scope == "track" {
		tracks, err := listTracks(db, schema, true, 0)
		if err != nil {
			return fmt.Errorf("kon tracks met afbeeldingen niet ophalen: %w", err)
		}
		totalCount = len(tracks)
	} else {
		return fmt.Errorf("ongeldige scope: %s", scope)
	}

	if totalCount == 0 {
		fmt.Printf("Geen %s met afbeeldingen gevonden.\n", getScopeDescription(scope))
		return nil
	}

	// Show warning
	fmt.Printf("%s%sWAARSCHUWING:%s %d %s afbeeldingen verwijderen\n\n", Bold, Red, Reset, totalCount, getScopeDescription(scope))

	// Show sample items
	if scope == "artist" {
		artists, _ := listArtists(db, schema, true, 20)
		for i, artist := range artists {
			if i < 20 {
				fmt.Printf("  • %s\n", artist.Name)
			} else if i == 20 {
				fmt.Printf("  ... en %d meer\n", totalCount-20)
				break
			}
		}
	} else if scope == "track" {
		tracks, _ := listTracks(db, schema, true, 20)
		for i, track := range tracks {
			if i < 20 {
				fmt.Printf("  • %s - %s\n", track.Artist, track.Title)
			} else if i == 20 {
				fmt.Printf("  ... en %d meer\n", totalCount-20)
				break
			}
		}
	}

	if dryRun {
		fmt.Printf("\n%sDRY RUN:%s Zou verwijderen maar doet dit niet\n", Yellow, Reset)
		return nil
	}

	fmt.Printf("\nBevestig met '%sVERWIJDER ALLES%s': ", Red, Reset)
	reader := bufio.NewReader(os.Stdin)
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(confirmation)

	if confirmation != "VERWIJDER ALLES" {
		fmt.Println("Operatie geannuleerd.")
		return nil
	}

	// Delete images based on scope
	var query string
	if scope == "artist" {
		query = fmt.Sprintf(`UPDATE %s.artist SET picture = NULL WHERE picture IS NOT NULL`, schema)
	} else {
		query = fmt.Sprintf(`UPDATE %s.track SET picture = NULL WHERE picture IS NOT NULL`, schema)
	}

	result, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("kon %s afbeeldingen niet verwijderen: %w", getScopeDescription(scope), err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		fmt.Printf("%s✓%s Afbeeldingen verwijderd\n", Green, Reset)
	} else {
		fmt.Printf("%s✓%s %d %s afbeeldingen verwijderd\n", Green, Reset, rowsAffected, getScopeDescription(scope))
	}

	return nil
}

func getScopeDescription(scope string) string {
	switch scope {
	case "artist":
		return "artiest"
	case "track":
		return "track"
	default:
		return "item"
	}
}

func findArtistsWithPartialName(db *sql.DB, schema, partialName string) ([]Artist, error) {
	query := fmt.Sprintf(`SELECT artistid, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
	                      FROM %s.artist WHERE LOWER(artist) LIKE LOWER($1) 
	                      ORDER BY artist`, schema)

	rows, err := db.Query(query, "%"+partialName+"%")
	if err != nil {
		return nil, fmt.Errorf("kon artiesten niet zoeken: %w", err)
	}
	defer rows.Close()

	var artists []Artist
	for rows.Next() {
		var artist Artist
		if err := rows.Scan(&artist.ID, &artist.Name, &artist.HasImage); err != nil {
			return nil, err
		}
		artists = append(artists, artist)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return artists, nil
}

func displayArtistList(title string, artists []Artist, showImageStatus bool, maxNote string) {
	if len(artists) == 0 {
		fmt.Printf("%sGeen artiesten gevonden%s\n", Yellow, Reset)
		return
	}

	// Title with count
	fmt.Printf("%s%s:%s %d", Cyan, title, Reset, len(artists))
	if maxNote != "" {
		fmt.Printf(" (%s)", maxNote)
	}
	fmt.Println()

	for _, artist := range artists {
		if showImageStatus {
			if artist.HasImage {
				fmt.Printf("  %s✓%s %s\n", Green, Reset, artist.Name)
			} else {
				fmt.Printf("  %s✗%s %s\n", Red, Reset, artist.Name)
			}
		} else {
			fmt.Printf("  • %s\n", artist.Name)
		}
	}
}

func searchArtists(db *sql.DB, schema, searchTerm string) error {
	artists, err := findArtistsWithPartialName(db, schema, searchTerm)
	if err != nil {
		return err
	}

	if len(artists) == 0 {
		fmt.Printf("%sGeen artiesten gevonden met '%s' in hun naam%s\n", Yellow, searchTerm, Reset)
		return nil
	}

	displayArtistList(fmt.Sprintf("Artiesten met '%s' in hun naam", searchTerm), artists, true, "")
	return nil
}

func listArtistsWithoutImages(db *sql.DB, schema string) error {
	artists, err := listArtists(db, schema, false, 50) // false = zonder afbeeldingen, 50 = limit
	if err != nil {
		return err
	}

	displayArtistList("Artiesten zonder afbeeldingen", artists, false, "max 50 getoond")
	return nil
}

// Track-related functions

func lookupTrack(db *sql.DB, schema, trackTitle, trackID string) (*Track, error) {
	var query string
	var searchValue string
	var scanID, scanTitle, scanArtist string
	var hasImage bool

	if trackID != "" {
		// Lookup by ID
		query = fmt.Sprintf(`SELECT titleid, tracktitle, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
		                     FROM %s.track WHERE titleid = $1`, schema)
		searchValue = trackID
	} else {
		// Lookup by title
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

func saveTrackImageToDatabase(db *sql.DB, schema, trackID string, imageData []byte) error {
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
	defer rows.Close()

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

func listTracksWithoutImages(db *sql.DB, schema string) error {
	tracks, err := listTracks(db, schema, false, 50) // false = zonder afbeeldingen, 50 = limit
	if err != nil {
		return err
	}

	displayTrackList("Tracks zonder afbeeldingen", tracks, false, "max 50 getoond")
	return nil
}

func displayTrackList(title string, tracks []Track, showImageStatus bool, maxNote string) {
	if len(tracks) == 0 {
		fmt.Printf("%sGeen tracks gevonden%s\n", Yellow, Reset)
		return
	}

	// Title with count
	fmt.Printf("%s%s:%s %d", Cyan, title, Reset, len(tracks))
	if maxNote != "" {
		fmt.Printf(" (%s)", maxNote)
	}
	fmt.Println()

	for _, track := range tracks {
		if showImageStatus {
			if track.HasImage {
				fmt.Printf("  %s✓%s %s - %s\n", Green, Reset, track.Artist, track.Title)
			} else {
				fmt.Printf("  %s✗%s %s - %s\n", Red, Reset, track.Artist, track.Title)
			}
		} else {
			fmt.Printf("  • %s - %s\n", track.Artist, track.Title)
		}
	}
}

func findTracksWithPartialName(db *sql.DB, schema, partialName string) ([]Track, error) {
	query := fmt.Sprintf(`SELECT titleid, tracktitle, artist, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
	                      FROM %s.track WHERE LOWER(tracktitle) LIKE LOWER($1) OR LOWER(artist) LIKE LOWER($1)
	                      ORDER BY artist, tracktitle`, schema)

	rows, err := db.Query(query, "%"+partialName+"%")
	if err != nil {
		return nil, fmt.Errorf("kon tracks niet zoeken: %w", err)
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var track Track
		if err := rows.Scan(&track.ID, &track.Title, &track.Artist, &track.HasImage); err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tracks, nil
}

func searchTracks(db *sql.DB, schema, searchTerm string) error {
	tracks, err := findTracksWithPartialName(db, schema, searchTerm)
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		fmt.Printf("%sGeen tracks gevonden met '%s' in titel of artiest%s\n", Yellow, searchTerm, Reset)
		return nil
	}

	displayTrackList(fmt.Sprintf("Tracks met '%s' in titel of artiest", searchTerm), tracks, true, "")
	return nil
}
