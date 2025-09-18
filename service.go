package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/jmoiron/sqlx"
)

// AeronService provides business logic for managing images in the Aeron radio automation system.
// It orchestrates image processing, database operations, and validation workflows.
type AeronService struct {
	db     *sqlx.DB
	config *Config
}

// NewAeronService creates a new AeronService instance with the provided database connection and configuration.
// The service uses the database for entity operations and config for image processing settings.
func NewAeronService(db *sqlx.DB, config *Config) *AeronService {
	return &AeronService{
		db:     db,
		config: config,
	}
}

// ImageUploadParams contains the parameters required for uploading an image to an entity.
// Either URL or ImageData should be provided, but not both.
type ImageUploadParams struct {
	Scope     string // Entity scope: "artist" or "track"
	ID        string // UUID of the entity to update
	URL       string // URL of the image to download (optional)
	ImageData []byte // Raw image data (optional)
}

// ImageUploadResult contains the results of an image upload operation.
// It provides information about the processed image and optimization statistics.
type ImageUploadResult struct {
	ItemName       string  // Artist name or Track artist
	ItemTitle      string  // Track title (empty for artists)
	OriginalSize   int     // Size of original image in bytes
	OptimizedSize  int     // Size of optimized image in bytes
	SavingsPercent float64 // Percentage of size reduction achieved
	Encoder        string  // Name of encoder used for optimization
}

// UploadImage processes and uploads an image for the specified entity.
// It validates the entity exists, downloads/processes the image, optimizes it,
// and stores it in the database. Returns optimization statistics on success.
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

// ImageStats represents statistics about images in the database.
// It provides counts of entities with and without images.
type ImageStats struct {
	Total         int // Total number of entities
	WithImages    int // Number of entities with images
	WithoutImages int // Number of entities without images
}

// DeleteResult contains the results of a bulk image deletion operation.
type DeleteResult struct {
	Count   int   // Number of entities that had images before deletion
	Deleted int64 // Number of images actually deleted
}

// DeleteAllImages removes all images for entities of the specified scope.
// It returns the number of images that were deleted.
// The scope must be either "artist" or "track".
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

// decodeBase64 decodes a base64-encoded string into raw bytes.
// It automatically handles data URL prefixes (e.g., "data:image/jpeg;base64,").
func decodeBase64(data string) ([]byte, error) {
	// Remove data URL prefix if present (e.g., "data:image/jpeg;base64,")
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}

// GetStatistics returns image statistics for entities of the specified scope.
// It counts total entities, those with images, and those without images.
// The scope must be either "artist" or "track".
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
