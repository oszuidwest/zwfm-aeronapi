package main

import (
	"database/sql"
	"fmt"

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

func findArtistByExactName(db *sql.DB, schema, artistName string) (string, bool, error) {
	query := fmt.Sprintf(`SELECT artistid, CASE WHEN picture IS NOT NULL THEN true ELSE false END as has_image 
	                      FROM %s.artist WHERE artist = $1`, schema)

	var artistID string
	var hasImage bool
	err := db.QueryRow(query, artistName).Scan(&artistID, &hasImage)

	if err == sql.ErrNoRows {
		return "", false, fmt.Errorf("artiest '%s' niet gevonden", artistName)
	}
	if err != nil {
		return "", false, fmt.Errorf("database fout: %w", err)
	}

	return artistID, hasImage, nil
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

	fmt.Printf("\033[1;31mWAARSCHUWING:\033[0m Alle afbeeldingen verwijderen van \033[1m%d\033[0m artiesten\n", len(artists))
	fmt.Println("═══════════════════════════════════════════════════════")

	// Toon eerste 20 artiesten, dan samenvatting als er meer zijn
	displayLimit := 20
	for i, artist := range artists {
		if i < displayLimit {
			fmt.Printf("  • %s\n", artist.Name)
		} else if i == displayLimit {
			fmt.Printf("  ... en %d meer artiesten\n", len(artists)-displayLimit)
			break
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("\033[1mTotaal:\033[0m \033[31m%d\033[0m artiesten verliezen hun afbeelding\n", len(artists))

	if dryRun {
		fmt.Println()
		fmt.Println("\033[33mDRY RUN:\033[0m Zou alle afbeeldingen verwijderen maar doet dit niet daadwerkelijk")
		return nil
	}

	fmt.Println()
	// Bevestiging vragen
	fmt.Print("\033[1mBen je ZEKER dat je ALLE afbeeldingen wilt verwijderen?\033[0m Type '\033[31mVERWIJDER ALLES\033[0m' om te bevestigen: ")
	var confirmation string
	fmt.Scanln(&confirmation)

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
		fmt.Printf("Afbeeldingen succesvol verwijderd (aantal onbekend)\n")
	} else {
		fmt.Printf("Succesvol %d afbeeldingen verwijderd uit de database\n", rowsAffected)
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

func searchArtists(db *sql.DB, schema, searchTerm string) error {
	artists, err := findArtistsWithPartialName(db, schema, searchTerm)
	if err != nil {
		return err
	}

	if len(artists) == 0 {
		fmt.Printf("\033[33mGeen artiesten gevonden met '%s' in hun naam\033[0m\n", searchTerm)
		return nil
	}

	fmt.Printf("\033[36mArtiesten met '%s' in hun naam\033[0m (\033[1m%d\033[0m gevonden):\n", searchTerm, len(artists))
	fmt.Println("─────────────────────────────────────────────────")
	for _, artist := range artists {
		imageStatus := "\033[31m✗ geen afbeelding\033[0m"
		if artist.HasImage {
			imageStatus = "\033[32m✓ heeft afbeelding\033[0m"
		}
		fmt.Printf("  \033[33m•\033[0m %s (%s)\n", artist.Name, imageStatus)
	}
	fmt.Println("─────────────────────────────────────────────────")

	return nil
}

func listArtistsWithoutImages(db *sql.DB, schema string) error {
	artists, err := listArtists(db, schema, false, 50) // false = zonder afbeeldingen, 50 = limit
	if err != nil {
		return err
	}

	fmt.Printf("\033[36mArtiesten zonder afbeeldingen\033[0m (\033[1m%d\033[0m gevonden, max 50 getoond):\n", len(artists))
	fmt.Println("─────────────────────────────────────────────────")
	for _, artist := range artists {
		fmt.Printf("  \033[33m•\033[0m %s\n", artist.Name)
	}
	fmt.Println("─────────────────────────────────────────────────")

	return nil
}
