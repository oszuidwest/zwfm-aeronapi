package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// DB is an interface that abstracts database operations used by the service.
// It wraps the essential sqlx.DB methods to allow for easier testing and future flexibility.
// The standard *sqlx.DB type satisfies this interface.
type DB interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PingContext(ctx context.Context) error
}

// AeronService provides business logic for managing images in the Aeron radio automation system.
// It orchestrates image processing, database operations, and validation workflows.
type AeronService struct {
	db     DB
	config *Config
	schema string // Cached schema name for convenience
}

// NewAeronService creates a new AeronService instance with the provided database connection and configuration.
// The service uses the database for entity operations and config for image processing settings.
func NewAeronService(db DB, config *Config) *AeronService {
	return &AeronService{
		db:     db,
		config: config,
		schema: config.Database.Schema,
	}
}

// ImageUploadParams contains the parameters required for uploading an image to an entity.
// Either URL or ImageData should be provided, but not both.
type ImageUploadParams struct {
	Scope     Scope  // Entity scope: ScopeArtist or ScopeTrack
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
func (s *AeronService) UploadImage(ctx context.Context, params *ImageUploadParams) (*ImageUploadResult, error) {
	if err := ValidateImageUploadParams(params); err != nil {
		return nil, err
	}

	// First check if the artist/track exists before downloading/processing the image
	var itemName, itemTitle string

	if params.Scope == ScopeArtist {
		artist, err := getArtistByID(ctx, s.db, s.schema, params.ID)
		if err != nil {
			slog.Error("Artiest ophalen mislukt", "artist_id", params.ID, "error", err)
			return nil, err
		}
		itemName = artist.Name
	} else {
		track, err := getTrackByID(ctx, s.db, s.schema, params.ID)
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
		imageData, err = downloadImage(params.URL, s.config.Image.GetMaxDownloadBytes())
		if err != nil {
			slog.Error("Afbeelding download mislukt", "url", params.URL, "error", err)
			return nil, &ImageProcessingError{Reason: fmt.Sprintf("downloaden mislukt: %v", err)}
		}
	} else {
		imageData = params.ImageData
	}

	// Process image
	processingResult, err := processImage(imageData, s.config.Image)
	if err != nil {
		slog.Error("Afbeelding verwerking mislukt", "error", err)
		return nil, &ImageProcessingError{Reason: fmt.Sprintf("verwerken mislukt: %v", err)}
	}

	// Update the database
	table := TableForScope(params.Scope)
	if err := updateEntityImage(ctx, s.db, s.schema, table, params.ID, processingResult.Data); err != nil {
		slog.Error("Afbeelding opslaan mislukt", "scope", params.Scope, "id", params.ID, "error", err)
		return nil, &DatabaseError{Operation: "afbeelding opslaan", Err: err}
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
func (s *AeronService) DeleteAllImages(ctx context.Context, scope Scope) (*DeleteResult, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	table := TableForScope(scope)
	qt, err := QualifiedTable(s.schema, table)
	if err != nil {
		slog.Error("Ongeldige tabel configuratie", "schema", s.schema, "table", table, "error", err)
		return nil, &ConfigurationError{Field: "tabel", Message: fmt.Sprintf("ongeldige tabel configuratie: %v", err)}
	}

	count, err := countItems(ctx, s.db, s.schema, table, true)
	if err != nil {
		slog.Error("Tellen items met afbeeldingen mislukt", "scope", scope, "error", err)
		return nil, err
	}

	if count == 0 {
		return &DeleteResult{Count: count}, nil
	}

	query := fmt.Sprintf(`UPDATE %s SET picture = NULL WHERE picture IS NOT NULL`, qt)

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		itemType := EntityTypeForScope(scope)
		slog.Error("Bulk delete query mislukt", "scope", scope, "query", query, "error", err)
		return nil, &DatabaseError{Operation: fmt.Sprintf("verwijderen van %s-afbeeldingen", itemType), Err: err}
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		slog.Warn("Kon aantal verwijderde rijen niet ophalen", "error", err)
		deleted = int64(count) // fallback naar count
	}
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
func (s *AeronService) GetStatistics(ctx context.Context, scope Scope) (*ImageStats, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	table := TableForScope(scope)

	// Get counts
	withImages, err := countItems(ctx, s.db, s.schema, table, true)
	if err != nil {
		slog.Error("Tellen items met afbeeldingen mislukt", "scope", scope, "error", err)
		return nil, &DatabaseError{Operation: "tellen items met afbeeldingen", Err: err}
	}

	withoutImages, err := countItems(ctx, s.db, s.schema, table, false)
	if err != nil {
		slog.Error("Tellen items zonder afbeeldingen mislukt", "scope", scope, "error", err)
		return nil, &DatabaseError{Operation: "tellen items zonder afbeeldingen", Err: err}
	}

	total := withImages + withoutImages

	return &ImageStats{
		Total:         total,
		WithImages:    withImages,
		WithoutImages: withoutImages,
	}, nil
}
