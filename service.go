package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// ImageService provides unified image management operations
type ImageService struct {
	db     *sql.DB
	config *Config
}

// NewImageService creates a new image service instance
func NewImageService(db *sql.DB, config *Config) *ImageService {
	return &ImageService{
		db:     db,
		config: config,
	}
}

// ImageUploadParams contains parameters for image upload
type ImageUploadParams struct {
	Scope     string
	Name      string
	ID        string
	URL       string
	ImageData []byte
}

// ImageUploadResult contains the result of an image upload
type ImageUploadResult struct {
	ItemName       string
	ItemTitle      string // for tracks
	OriginalSize   int
	OptimizedSize  int
	SavingsPercent float64
	Encoder        string
}

// Common error types
var (
	ErrInvalidScope     = fmt.Errorf("ongeldige scope: moet 'artist' of 'track' zijn")
	ErrNoNameOrID       = fmt.Errorf("moet naam of id specificeren")
	ErrBothNameAndID    = fmt.Errorf("kan niet zowel naam als id specificeren")
	ErrNoImageSource    = fmt.Errorf("moet url of afbeelding data specificeren")
	ErrBothImageSources = fmt.Errorf("kan niet zowel url als afbeelding data specificeren")
)

// ValidateScope validates the scope parameter
func ValidateScope(scope string) error {
	if scope != ScopeArtist && scope != ScopeTrack {
		return ErrInvalidScope
	}
	return nil
}

// ValidateImageUploadParams validates image upload parameters
func (s *ImageService) ValidateImageUploadParams(params *ImageUploadParams) error {
	// Validate scope
	if err := ValidateScope(params.Scope); err != nil {
		return err
	}

	// Validate identification
	if params.Name == "" && params.ID == "" {
		return ErrNoNameOrID
	}
	if params.Name != "" && params.ID != "" {
		return ErrBothNameAndID
	}

	// Validate image source
	if params.URL == "" && len(params.ImageData) == 0 {
		return ErrNoImageSource
	}
	if params.URL != "" && len(params.ImageData) > 0 {
		return ErrBothImageSources
	}

	return nil
}

// UploadImage handles image upload for both artists and tracks
func (s *ImageService) UploadImage(params *ImageUploadParams) (*ImageUploadResult, error) {
	// Validate parameters
	if err := s.ValidateImageUploadParams(params); err != nil {
		return nil, err
	}

	// Load image data if URL provided
	var imageData []byte
	var err error
	if params.URL != "" {
		imageData, err = downloadImage(params.URL, s.config.Image.MaxFileSizeMB)
		if err != nil {
			return nil, fmt.Errorf("kon afbeelding niet downloaden: %w", err)
		}
	} else {
		imageData = params.ImageData
	}

	// Process and optimize image
	processingResult, err := processAndOptimizeImage(imageData, s.config.Image)
	if err != nil {
		return nil, fmt.Errorf("kon afbeelding niet verwerken: %w", err)
	}

	// Handle based on scope
	result := &ImageUploadResult{
		OriginalSize:   processingResult.Original.Size,
		OptimizedSize:  processingResult.Optimized.Size,
		SavingsPercent: processingResult.Savings,
		Encoder:        processingResult.Encoder,
	}

	if params.Scope == ScopeArtist {
		artist, err := lookupArtist(s.db, s.config.Database.Schema, params.Name, params.ID)
		if err != nil {
			return nil, err
		}

		if err := updateArtistImageInDB(s.db, s.config.Database.Schema, artist.ID, processingResult.Data); err != nil {
			return nil, fmt.Errorf("kon database niet bijwerken: %w", err)
		}

		result.ItemName = artist.Name
	} else {
		track, err := lookupTrack(s.db, s.config.Database.Schema, params.Name, params.ID)
		if err != nil {
			return nil, err
		}

		if err := saveTrackImageToDatabase(s.db, s.config.Database.Schema, track.ID, processingResult.Data); err != nil {
			return nil, fmt.Errorf("kon database niet bijwerken: %w", err)
		}

		result.ItemName = track.Artist
		result.ItemTitle = track.Title
	}

	return result, nil
}

// ListWithoutImages returns items without images
func (s *ImageService) ListWithoutImages(scope string, limit int) (interface{}, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	if scope == ScopeArtist {
		return listArtists(s.db, s.config.Database.Schema, false, limit)
	}
	return listTracks(s.db, s.config.Database.Schema, false, limit)
}

// Search performs a search for items
func (s *ImageService) Search(scope, searchTerm string) (interface{}, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	if scope == ScopeArtist {
		return findArtistsWithPartialName(s.db, s.config.Database.Schema, searchTerm)
	}
	return findTracksWithPartialName(s.db, s.config.Database.Schema, searchTerm)
}

// NukeResult contains the result of a nuke operation
type NukeResult struct {
	Count   int
	Deleted int64
}

// CountImagesForNuke counts images that would be deleted
func (s *ImageService) CountImagesForNuke(scope string) (*NukeResult, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	result := &NukeResult{}

	if scope == ScopeArtist {
		artists, err := listArtists(s.db, s.config.Database.Schema, true, 0)
		if err != nil {
			return nil, err
		}
		result.Count = len(artists)
	} else {
		tracks, err := listTracks(s.db, s.config.Database.Schema, true, 0)
		if err != nil {
			return nil, err
		}
		result.Count = len(tracks)
	}

	return result, nil
}

// NukeImages deletes all images for the given scope
func (s *ImageService) NukeImages(scope string) (*NukeResult, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	// Get count first
	countResult, err := s.CountImagesForNuke(scope)
	if err != nil {
		return nil, err
	}

	if countResult.Count == 0 {
		return countResult, nil
	}

	// Delete images
	var query string
	if scope == ScopeArtist {
		query = fmt.Sprintf(`UPDATE %s.artist SET picture = NULL WHERE picture IS NOT NULL`, s.config.Database.Schema)
	} else {
		query = fmt.Sprintf(`UPDATE %s.track SET picture = NULL WHERE picture IS NOT NULL`, s.config.Database.Schema)
	}

	result, err := s.db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("kon %s afbeeldingen niet verwijderen: %w", getScopeDescription(scope), err)
	}

	countResult.Deleted, _ = result.RowsAffected()
	return countResult, nil
}

// DecodeBase64Image decodes a base64 encoded image
func DecodeBase64Image(data string) ([]byte, error) {
	// Remove data URL prefix if present
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}

// GetPreviewItems returns a preview of items for nuke operation
func (s *ImageService) GetPreviewItems(scope string, limit int) ([]string, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	var items []string

	if scope == ScopeArtist {
		artists, err := listArtists(s.db, s.config.Database.Schema, true, limit)
		if err != nil {
			return nil, err
		}
		for _, artist := range artists {
			items = append(items, artist.Name)
		}
	} else {
		tracks, err := listTracks(s.db, s.config.Database.Schema, true, limit)
		if err != nil {
			return nil, err
		}
		for _, track := range tracks {
			items = append(items, fmt.Sprintf("%s - %s", track.Artist, track.Title))
		}
	}

	return items, nil
}
