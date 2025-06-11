package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

type ImageService struct {
	db     *sql.DB
	config *Config
}

func NewImageService(db *sql.DB, config *Config) *ImageService {
	return &ImageService{
		db:     db,
		config: config,
	}
}

type ImageUploadParams struct {
	Scope     string
	Name      string
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

var (
	ErrInvalidScope     = fmt.Errorf("ongeldig type: gebruik 'artist' of 'track'")
	ErrNoNameOrID       = fmt.Errorf("naam of ID verplicht")
	ErrBothNameAndID    = fmt.Errorf("gebruik naam of ID, niet beide")
	ErrNoImageSource    = fmt.Errorf("afbeelding verplicht")
	ErrBothImageSources = fmt.Errorf("gebruik URL of upload, niet beide")
)

func scopeDesc(scope string) string {
	if scope == ScopeArtist {
		return "artiest"
	}
	return "track"
}

func ValidateScope(scope string) error {
	if scope != ScopeArtist && scope != ScopeTrack {
		return ErrInvalidScope
	}
	return nil
}

func (s *ImageService) ValidateUploadParams(params *ImageUploadParams) error {
	if err := ValidateScope(params.Scope); err != nil {
		return err
	}

	if params.Name == "" && params.ID == "" {
		return ErrNoNameOrID
	}
	if params.Name != "" && params.ID != "" {
		return ErrBothNameAndID
	}

	if params.URL == "" && len(params.ImageData) == 0 {
		return ErrNoImageSource
	}
	if params.URL != "" && len(params.ImageData) > 0 {
		return ErrBothImageSources
	}

	return nil
}

func (s *ImageService) UploadImage(params *ImageUploadParams) (*ImageUploadResult, error) {
	if err := s.ValidateUploadParams(params); err != nil {
		return nil, err
	}

	// First check if the artist/track exists before downloading/processing the image
	var itemName, itemTitle string
	var itemID string

	if params.Scope == ScopeArtist {
		artist, err := lookupArtist(s.db, s.config.Database.Schema, params.Name, params.ID)
		if err != nil {
			return nil, err
		}
		itemName = artist.Name
		itemID = artist.ID
	} else {
		track, err := lookupTrack(s.db, s.config.Database.Schema, params.Name, params.ID)
		if err != nil {
			return nil, err
		}
		itemName = track.Artist
		itemTitle = track.Title
		itemID = track.ID
	}

	// Now download/get the image data
	var imageData []byte
	var err error
	if params.URL != "" {
		imageData, err = downloadImage(params.URL)
		if err != nil {
			return nil, fmt.Errorf("download mislukt: %w", err)
		}
	} else {
		imageData = params.ImageData
	}

	// Process image
	processingResult, err := processImage(imageData, s.config.Image)
	if err != nil {
		return nil, fmt.Errorf("verwerking mislukt: %w", err)
	}

	// Update the database
	if params.Scope == ScopeArtist {
		if err := updateArtistImage(s.db, s.config.Database.Schema, itemID, processingResult.Data); err != nil {
			return nil, fmt.Errorf("opslaan mislukt: %w", err)
		}
	} else {
		if err := updateTrackImage(s.db, s.config.Database.Schema, itemID, processingResult.Data); err != nil {
			return nil, fmt.Errorf("opslaan mislukt: %w", err)
		}
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
	Orphaned      int
}

type DeleteResult struct {
	Count   int
	Deleted int64
}

func (s *ImageService) DeleteAllImages(scope string) (*DeleteResult, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	var table string
	if scope == ScopeArtist {
		table = "artist"
	} else {
		table = "track"
	}

	count, err := countItems(s.db, s.config.Database.Schema, table, true)
	if err != nil {
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
		return nil, fmt.Errorf("verwijderen %s mislukt: %w", scopeDesc(scope), err)
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

func (s *ImageService) GetStatistics(scope string) (*ImageStats, error) {
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
		return nil, fmt.Errorf("tellen mislukt: %w", err)
	}

	withoutImages, err := countItems(s.db, s.config.Database.Schema, table, false)
	if err != nil {
		return nil, fmt.Errorf("tellen mislukt: %w", err)
	}

	total := withImages + withoutImages

	// Get orphaned count
	var orphaned int
	if scope == ScopeArtist {
		orphaned, err = countOrphanedArtists(s.db, s.config.Database.Schema)
	} else {
		orphaned, err = countOrphanedTracks(s.db, s.config.Database.Schema)
	}
	if err != nil {
		return nil, err
	}

	return &ImageStats{
		Total:         total,
		WithImages:    withImages,
		WithoutImages: withoutImages,
		Orphaned:      orphaned,
	}, nil
}

func (s *ImageService) GetTodayPlaylist() ([]PlaylistItem, error) {
	return getTodayPlaylist(s.db, s.config.Database.Schema)
}

func (s *ImageService) GetPlaylist(opts PlaylistOptions) ([]PlaylistItem, error) {
	return getPlaylist(s.db, s.config.Database.Schema, opts)
}
