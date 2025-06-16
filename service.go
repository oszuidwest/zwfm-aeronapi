package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/jmoiron/sqlx"
)

type AeronService struct {
	db     *sqlx.DB
	config *Config
}

func NewAeronService(db *sqlx.DB, config *Config) *AeronService {
	return &AeronService{
		db:     db,
		config: config,
	}
}

type ImageUploadParams struct {
	Scope     string
	ID        string
	URL       string
	ImageData []byte
}

type ImageUploadResult struct {
	ItemName       string // Artist name or Track artist
	ItemTitle      string // Track title (empty for artists)
	OriginalSize   int
	OptimizedSize  int
	SavingsPercent float64
	Encoder        string
}

func (s *AeronService) UploadImage(params *ImageUploadParams) (*ImageUploadResult, error) {
	if err := ValidateImageUploadParams(params); err != nil {
		return nil, err
	}

	// First check if the artist/track exists before downloading/processing the image
	var itemName, itemTitle string

	// Processing image upload

	if params.Scope == ScopeArtist {
		artist, err := getArtistByID(s.db, s.config.Database.Schema, params.ID)
		if err != nil {
			slog.Error("Artiest ophalen mislukt", "artist_id", params.ID, "error", err)
			return nil, err
		}
		itemName = artist.Name
	} else {
		track, err := getTrackByID(s.db, s.config.Database.Schema, params.ID)
		if err != nil {
			slog.Error("Track ophalen mislukt", "track_id", params.ID, "error", err)
			return nil, err
		}
		itemName = track.Artist
		itemTitle = track.Title
	}

	// Now download/get the image data
	var imageData []byte
	var err error
	if params.URL != "" {
		imageData, err = downloadImage(params.URL)
		if err != nil {
			slog.Error("Afbeelding download mislukt", "url", params.URL, "error", err)
			return nil, fmt.Errorf("downloaden %s: %w", ErrSuffixFailed, err)
		}
	} else {
		imageData = params.ImageData
	}

	// Process image
	processingResult, err := processImage(imageData, s.config.Image)
	if err != nil {
		slog.Error("Afbeelding verwerking mislukt", "error", err)
		return nil, fmt.Errorf("verwerken %s: %w", ErrSuffixFailed, err)
	}

	// Update the database
	if params.Scope == ScopeArtist {
		if err := updateEntityImage(s.db, s.config.Database.Schema, tableArtist, params.ID, processingResult.Data); err != nil {
			slog.Error("Artiest afbeelding opslaan mislukt", "artist_id", params.ID, "error", err)
			return nil, fmt.Errorf("opslaan %s: %w", ErrSuffixFailed, err)
		}
		// Artist image saved successfully
	} else {
		if err := updateEntityImage(s.db, s.config.Database.Schema, tableTrack, params.ID, processingResult.Data); err != nil {
			slog.Error("Track afbeelding opslaan mislukt", "track_id", params.ID, "error", err)
			return nil, fmt.Errorf("opslaan %s: %w", ErrSuffixFailed, err)
		}
		// Track image saved successfully
	}

	return &ImageUploadResult{
		OriginalSize:   processingResult.Original.Size,
		OptimizedSize:  processingResult.Optimized.Size,
		SavingsPercent: processingResult.Savings,
		Encoder:        processingResult.Encoder,
		ItemName:       itemName,
		ItemTitle:      itemTitle,
	}, nil
}

type ImageStats struct {
	Total         int
	WithImages    int
	WithoutImages int
}

type DeleteResult struct {
	Count   int
	Deleted int64
}

func (s *AeronService) DeleteAllImages(scope string) (*DeleteResult, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	var table string
	if scope == ScopeArtist {
		table = "artist"
	} else {
		table = "track"
	}

	// Starting bulk delete operation

	count, err := countItems(s.db, s.config.Database.Schema, table, true)
	if err != nil {
		slog.Error("Tellen items met afbeeldingen mislukt", "scope", scope, "error", err)
		return nil, err
	}

	if count == 0 {
		return &DeleteResult{Count: count}, nil
	}

	var query string
	if scope == ScopeArtist {
		query = fmt.Sprintf(`UPDATE %s.artist SET picture = NULL WHERE picture IS NOT NULL`, s.config.Database.Schema)
	} else {
		query = fmt.Sprintf(`UPDATE %s.track SET picture = NULL WHERE picture IS NOT NULL`, s.config.Database.Schema)
	}

	result, err := s.db.Exec(query)
	if err != nil {
		itemType := ItemTypeTrack
		if scope == ScopeArtist {
			itemType = ItemTypeArtist
		}
		slog.Error("Bulk delete query mislukt", "scope", scope, "query", query, "error", err)
		return nil, fmt.Errorf("verwijderen van %s-afbeeldingen mislukt: %w", itemType, err)
	}

	deleted, _ := result.RowsAffected()
	return &DeleteResult{Count: count, Deleted: deleted}, nil
}

func decodeBase64(data string) ([]byte, error) {
	// Remove data URL prefix if present (e.g., "data:image/jpeg;base64,")
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}

func (s *AeronService) GetStatistics(scope string) (*ImageStats, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	var table string
	if scope == ScopeArtist {
		table = "artist"
	} else {
		table = "track"
	}

	// Get counts
	withImages, err := countItems(s.db, s.config.Database.Schema, table, true)
	if err != nil {
		slog.Error("Tellen items met afbeeldingen mislukt", "scope", scope, "error", err)
		return nil, fmt.Errorf("tellen mislukt: %w", err)
	}

	withoutImages, err := countItems(s.db, s.config.Database.Schema, table, false)
	if err != nil {
		slog.Error("Tellen items zonder afbeeldingen mislukt", "scope", scope, "error", err)
		return nil, fmt.Errorf("tellen mislukt: %w", err)
	}

	total := withImages + withoutImages

	return &ImageStats{
		Total:         total,
		WithImages:    withImages,
		WithoutImages: withoutImages,
	}, nil
}
