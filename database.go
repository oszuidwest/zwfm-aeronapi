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

func nukeAllImages(db *sql.DB, schema string, dryRun bool) error {
	// Eerst tellen en tonen welke artiesten geraakt worden
	artists, err := listArtists(db, schema, true, 0) // true = met afbeeldingen, 0 = geen limit
	if err != nil {
		return fmt.Errorf("kon artiesten met afbeeldingen niet ophalen: %w", err)
	}

	if len(artists) == 0 {
		fmt.Println("Geen artiesten met afbeeldingen gevonden.")
		return nil
	}

	fmt.Printf("%s%sWAARSCHUWING:%s %d afbeeldingen verwijderen\n\n", Bold, Red, Reset, len(artists))

	// Toon eerste 20 artiesten
	displayLimit := 20
	for i, artist := range artists {
		if i < displayLimit {
			fmt.Printf("  • %s\n", artist.Name)
		} else if i == displayLimit {
			fmt.Printf("  ... en %d meer\n", len(artists)-displayLimit)
			break
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

	// Alle afbeeldingen verwijderen
	query := fmt.Sprintf(`UPDATE %s.artist SET picture = NULL WHERE picture IS NOT NULL`, schema)
	result, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("kon afbeeldingen niet verwijderen: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		fmt.Printf("%s✓%s Afbeeldingen verwijderd\n", Green, Reset)
	} else {
		fmt.Printf("%s✓%s %d afbeeldingen verwijderd\n", Green, Reset, rowsAffected)
	}

	return nil
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
