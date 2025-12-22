// Package service provides business logic for managing images in the Aeron radio automation system.
package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/oszuidwest/zwfm-aeronapi/internal/config"
	"github.com/oszuidwest/zwfm-aeronapi/internal/database"
	"github.com/oszuidwest/zwfm-aeronapi/internal/image"
	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
)

// DB defines the database interface required by the service layer.
// It extends the database package's interface with PingContext for health monitoring.
// This interface follows the Interface Segregation Principle - the service layer
// needs health checks while the database package does not.
// The *sqlx.DB type satisfies this interface.
type DB interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PingContext(ctx context.Context) error // Used by health endpoint to verify database connectivity
}

// AeronService provides business logic for managing images in the Aeron radio automation system.
// It orchestrates image processing, database operations, and validation workflows.
type AeronService struct {
	db     DB
	config *config.Config
	schema string // Cached schema name for convenience
}

// New creates a new AeronService instance with the provided database connection and configuration.
// The service uses the database for entity operations and config for image processing settings.
func New(db DB, cfg *config.Config) *AeronService {
	return &AeronService{
		db:     db,
		config: cfg,
		schema: cfg.Database.Schema,
	}
}

// Config returns the service configuration.
func (s *AeronService) Config() *config.Config {
	return s.config
}

// DB returns the database interface.
func (s *AeronService) DB() DB {
	return s.db
}

// ImageUploadParams contains the parameters required for uploading an image to an entity.
// Either ImageURL or ImageData should be provided, but not both.
type ImageUploadParams struct {
	EntityType types.EntityType // Entity type: EntityTypeArtist or EntityTypeTrack
	ID         string           // UUID of the entity to update
	ImageURL   string           // URL of the image to download (optional)
	ImageData  []byte           // Raw image data (optional)
}

// ImageUploadResult contains the results of an image upload operation.
// It provides information about the processed image and optimization statistics.
type ImageUploadResult struct {
	ArtistName          string  // Artist name (always populated)
	TrackTitle          string  // Track title (empty for artists)
	OriginalSize        int     // Size of original image in bytes
	OptimizedSize       int     // Size of optimized image in bytes
	SizeReductionPercent float64 // Percentage of size reduction achieved
	Encoder             string  // Name of encoder used for optimization
}

// UploadImage processes and uploads an image for the specified entity.
// It validates the entity exists, downloads/processes the image, optimizes it,
// and stores it in the database. Returns optimization statistics on success.
func (s *AeronService) UploadImage(ctx context.Context, params *ImageUploadParams) (*ImageUploadResult, error) {
	if err := validateImageUploadParams(params); err != nil {
		return nil, err
	}

	// First check if the artist/track exists before downloading/processing the image
	var name, title string

	if params.EntityType == types.EntityTypeArtist {
		artist, err := database.GetArtistByID(ctx, s.db, s.schema, params.ID)
		if err != nil {
			slog.Error("Artiest ophalen mislukt", "artist_id", params.ID, "error", err)
			return nil, err
		}
		name = artist.ArtistName
	} else {
		track, err := database.GetTrackByID(ctx, s.db, s.schema, params.ID)
		if err != nil {
			slog.Error("Track ophalen mislukt", "track_id", params.ID, "error", err)
			return nil, err
		}
		name = track.Artist
		title = track.TrackTitle
	}

	// Now download/get the image data
	var imageData []byte
	var err error
	if params.ImageURL != "" {
		imageData, err = image.DownloadImage(params.ImageURL, s.config.Image.GetMaxDownloadBytes())
		if err != nil {
			slog.Error("Afbeelding download mislukt", "url", params.ImageURL, "error", err)
			return nil, &types.ImageProcessingError{Message: fmt.Sprintf("downloaden mislukt: %v", err)}
		}
	} else {
		imageData = params.ImageData
	}

	// Process image
	imgConfig := image.Config{
		TargetWidth:   s.config.Image.TargetWidth,
		TargetHeight:  s.config.Image.TargetHeight,
		Quality:       s.config.Image.Quality,
		RejectSmaller: s.config.Image.RejectSmaller,
	}
	processingResult, err := image.Process(imageData, imgConfig)
	if err != nil {
		slog.Error("Afbeelding verwerking mislukt", "error", err)
		return nil, &types.ImageProcessingError{Message: fmt.Sprintf("verwerken mislukt: %v", err)}
	}

	// Update the database
	table := types.TableForEntityType(params.EntityType)
	if err := database.UpdateEntityImage(ctx, s.db, s.schema, table, params.ID, processingResult.Data); err != nil {
		slog.Error("Afbeelding opslaan mislukt", "entityType", params.EntityType, "id", params.ID, "error", err)
		return nil, &types.DatabaseError{Operation: "afbeelding opslaan", Err: err}
	}

	return &ImageUploadResult{
		OriginalSize:        processingResult.Original.Size,
		OptimizedSize:       processingResult.Optimized.Size,
		SizeReductionPercent: processingResult.Savings,
		Encoder:             processingResult.Encoder,
		ArtistName:          name,
		TrackTitle:          title,
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
	CountBefore  int   // Number of entities that had images before deletion
	DeletedCount int64 // Number of images actually deleted
}

// DeleteAllImages removes all images for entities of the specified type.
// It returns the number of images that were deleted.
func (s *AeronService) DeleteAllImages(ctx context.Context, entityType types.EntityType) (*DeleteResult, error) {
	if err := validateEntityType(entityType); err != nil {
		return nil, err
	}

	table := types.TableForEntityType(entityType)
	qt, err := types.QualifiedTable(s.schema, table)
	if err != nil {
		slog.Error("Ongeldige tabel configuratie", "schema", s.schema, "table", table, "error", err)
		return nil, &types.ConfigurationError{Field: "tabel", Message: fmt.Sprintf("ongeldige tabel configuratie: %v", err)}
	}

	count, err := database.CountItems(ctx, s.db, s.schema, table, true)
	if err != nil {
		slog.Error("Tellen items met afbeeldingen mislukt", "entityType", entityType, "error", err)
		return nil, err
	}

	if count == 0 {
		return &DeleteResult{CountBefore: count}, nil
	}

	query := fmt.Sprintf(`UPDATE %s SET picture = NULL WHERE picture IS NOT NULL`, qt)

	result, err := s.db.ExecContext(ctx, query)
	if err != nil {
		label := types.LabelForEntityType(entityType)
		slog.Error("Bulk delete query mislukt", "entityType", entityType, "query", query, "error", err)
		return nil, &types.DatabaseError{Operation: fmt.Sprintf("verwijderen van %s-afbeeldingen", label), Err: err}
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		slog.Warn("Kon aantal verwijderde rijen niet ophalen", "error", err)
		deleted = int64(count) // fallback naar count
	}
	return &DeleteResult{CountBefore: count, DeletedCount: deleted}, nil
}

// DecodeBase64 decodes a base64-encoded string into raw bytes.
// It automatically handles data URL prefixes (e.g., "data:image/jpeg;base64,").
func DecodeBase64(data string) ([]byte, error) {
	// Remove data URL prefix if present (e.g., "data:image/jpeg;base64,")
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}

// GetStatistics returns image statistics for entities of the specified type.
// It counts total entities, those with images, and those without images.
func (s *AeronService) GetStatistics(ctx context.Context, entityType types.EntityType) (*ImageStats, error) {
	if err := validateEntityType(entityType); err != nil {
		return nil, err
	}

	table := types.TableForEntityType(entityType)

	// Get counts
	withImages, err := database.CountItems(ctx, s.db, s.schema, table, true)
	if err != nil {
		slog.Error("Tellen items met afbeeldingen mislukt", "entityType", entityType, "error", err)
		return nil, &types.DatabaseError{Operation: "tellen items met afbeeldingen", Err: err}
	}

	withoutImages, err := database.CountItems(ctx, s.db, s.schema, table, false)
	if err != nil {
		slog.Error("Tellen items zonder afbeeldingen mislukt", "entityType", entityType, "error", err)
		return nil, &types.DatabaseError{Operation: "tellen items zonder afbeeldingen", Err: err}
	}

	total := withImages + withoutImages

	return &ImageStats{
		Total:         total,
		WithImages:    withImages,
		WithoutImages: withoutImages,
	}, nil
}

// validateEntityType checks if the provided entity type is valid.
func validateEntityType(entityType types.EntityType) error {
	if entityType != types.EntityTypeArtist && entityType != types.EntityTypeTrack {
		return types.NewValidationError("entityType", fmt.Sprintf("ongeldig type: gebruik '%s' of '%s'", types.EntityTypeArtist, types.EntityTypeTrack))
	}
	return nil
}

// validateImageUploadParams validates all parameters required for image upload operations.
func validateImageUploadParams(params *ImageUploadParams) error {
	if err := validateEntityType(params.EntityType); err != nil {
		return err
	}

	// Check that we have either ImageURL or image data, but not both
	hasURL := params.ImageURL != ""
	hasImageData := len(params.ImageData) > 0

	if !hasURL && !hasImageData {
		return types.NewValidationError("image", "afbeelding is verplicht")
	}

	if hasURL && hasImageData {
		return types.NewValidationError("image", "gebruik óf URL óf upload, niet beide")
	}

	return nil
}
