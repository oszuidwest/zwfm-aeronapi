// Package service provides business logic for the Aeron Toolbox.
package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/oszuidwest/zwfm-aerontoolbox/internal/config"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/database"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/image"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
)

// DB defines the database interface required by the service layer.
type DB interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PingContext(ctx context.Context) error
}

// AeronService provides business logic for the Aeron radio automation system.
type AeronService struct {
	db         DB
	config     *config.Config
	schema     string
	backupRoot *os.Root
}

// New creates a new AeronService instance.
func New(db DB, cfg *config.Config) (*AeronService, error) {
	svc := &AeronService{
		db:     db,
		config: cfg,
		schema: cfg.Database.Schema,
	}

	if cfg.Backup.Enabled {
		backupPath := cfg.Backup.GetPath()
		if err := os.MkdirAll(backupPath, 0o750); err != nil {
			return nil, &types.ConfigurationError{Field: "backup.path", Message: fmt.Sprintf("backup directory niet toegankelijk: %v", err)}
		}

		root, err := os.OpenRoot(backupPath)
		if err != nil {
			return nil, &types.ConfigurationError{Field: "backup.path", Message: fmt.Sprintf("backup directory niet te openen: %v", err)}
		}
		svc.backupRoot = root
	}

	return svc, nil
}

// Config returns the service configuration.
func (s *AeronService) Config() *config.Config {
	return s.config
}

// DB returns the database interface.
func (s *AeronService) DB() DB {
	return s.db
}

// ImageUploadParams contains the parameters for image upload operations.
type ImageUploadParams struct {
	EntityType types.EntityType
	ID         string
	ImageURL   string
	ImageData  []byte
}

// ImageUploadResult contains the results of an image upload operation.
type ImageUploadResult struct {
	ArtistName           string
	TrackTitle           string
	OriginalSize         int
	OptimizedSize        int
	SizeReductionPercent float64
}

// UploadImage processes and uploads an image for the specified entity.
func (s *AeronService) UploadImage(ctx context.Context, params *ImageUploadParams) (*ImageUploadResult, error) {
	if err := validateImageUploadParams(params); err != nil {
		return nil, err
	}

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

	table := types.TableForEntityType(params.EntityType)
	if err := database.UpdateEntityImage(ctx, s.db, s.schema, table, params.ID, processingResult.Data); err != nil {
		slog.Error("Afbeelding opslaan mislukt", "entityType", params.EntityType, "id", params.ID, "error", err)
		return nil, &types.DatabaseError{Operation: "afbeelding opslaan", Err: err}
	}

	return &ImageUploadResult{
		OriginalSize:         processingResult.Original.Size,
		OptimizedSize:        processingResult.Optimized.Size,
		SizeReductionPercent: processingResult.Savings,
		ArtistName:           name,
		TrackTitle:           title,
	}, nil
}

// ImageStats represents statistics about images in the database.
type ImageStats struct {
	Total         int
	WithImages    int
	WithoutImages int
}

// DeleteResult contains the results of a bulk image deletion operation.
type DeleteResult struct {
	CountBefore  int
	DeletedCount int64
}

// DeleteAllImages removes all images for entities of the specified type.
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
func DecodeBase64(data string) ([]byte, error) {
	if _, after, found := strings.Cut(data, ","); found {
		data = after
	}
	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}

// GetStatistics returns image statistics for entities of the specified type.
func (s *AeronService) GetStatistics(ctx context.Context, entityType types.EntityType) (*ImageStats, error) {
	if err := validateEntityType(entityType); err != nil {
		return nil, err
	}

	table := types.TableForEntityType(entityType)

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

func validateEntityType(entityType types.EntityType) error {
	if entityType != types.EntityTypeArtist && entityType != types.EntityTypeTrack {
		return types.NewValidationError("entityType", fmt.Sprintf("ongeldig type: gebruik '%s' of '%s'", types.EntityTypeArtist, types.EntityTypeTrack))
	}
	return nil
}

func validateImageUploadParams(params *ImageUploadParams) error {
	if err := validateEntityType(params.EntityType); err != nil {
		return err
	}

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
