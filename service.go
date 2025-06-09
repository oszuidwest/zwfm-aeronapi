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
	ErrInvalidScope     = fmt.Errorf("ongeldige scope: moet 'artist' of 'track' zijn")
	ErrNoNameOrID       = fmt.Errorf("moet naam of id specificeren")
	ErrBothNameAndID    = fmt.Errorf("kan niet zowel naam als id specificeren")
	ErrNoImageSource    = fmt.Errorf("moet url of afbeelding data specificeren")
	ErrBothImageSources = fmt.Errorf("kan niet zowel url als afbeelding data specificeren")
)

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

	var imageData []byte
	var err error
	if params.URL != "" {
		imageData, err = downloadImage(params.URL)
		if err != nil {
			return nil, fmt.Errorf("kon afbeelding niet downloaden: %w", err)
		}
	} else {
		imageData = params.ImageData
	}

	processingResult, err := processImage(imageData, s.config.Image)
	if err != nil {
		return nil, fmt.Errorf("kon afbeelding niet verwerken: %w", err)
	}

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

		if err := updateArtistImage(s.db, s.config.Database.Schema, artist.ID, processingResult.Data); err != nil {
			return nil, fmt.Errorf("kon database niet bijwerken: %w", err)
		}

		result.ItemName = artist.Name
	} else {
		track, err := lookupTrack(s.db, s.config.Database.Schema, params.Name, params.ID)
		if err != nil {
			return nil, err
		}

		if err := updateTrackImage(s.db, s.config.Database.Schema, track.ID, processingResult.Data); err != nil {
			return nil, fmt.Errorf("kon database niet bijwerken: %w", err)
		}

		result.ItemName = track.Artist
		result.ItemTitle = track.Title
	}

	return result, nil
}

type ListResult struct {
	Items interface{}
	Total int
}

type Statistics struct {
	Total         int
	WithImages    int
	WithoutImages int
	Orphaned      int
}

func (s *ImageService) ListWithFilter(scope string, withImages bool, limit int) (*ListResult, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	var table string
	if scope == ScopeArtist {
		table = "artist"
	} else {
		table = "track"
	}

	// Get total count
	totalCount, err := countItems(s.db, s.config.Database.Schema, table, withImages)
	if err != nil {
		return nil, err
	}

	// Get items
	var items interface{}
	if scope == ScopeArtist {
		items, err = listArtists(s.db, s.config.Database.Schema, withImages, limit)
	} else {
		items, err = listTracks(s.db, s.config.Database.Schema, withImages, limit)
	}
	if err != nil {
		return nil, err
	}

	return &ListResult{
		Items: items,
		Total: totalCount,
	}, nil
}

func (s *ImageService) Search(scope, searchTerm string) (interface{}, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	if scope == ScopeArtist {
		return searchArtists(s.db, s.config.Database.Schema, searchTerm)
	}
	return searchTracks(s.db, s.config.Database.Schema, searchTerm)
}

type NukeResult struct {
	Count   int
	Deleted int64
}

func (s *ImageService) CountForNuke(scope string) (*NukeResult, error) {
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

	return &NukeResult{Count: count}, nil
}

func (s *ImageService) NukeImages(scope string) (*NukeResult, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	countResult, err := s.CountForNuke(scope)
	if err != nil {
		return nil, err
	}

	if countResult.Count == 0 {
		return countResult, nil
	}

	var query string
	if scope == ScopeArtist {
		query = fmt.Sprintf(`UPDATE %s.artist SET picture = NULL WHERE picture IS NOT NULL`, s.config.Database.Schema)
	} else {
		query = fmt.Sprintf(`UPDATE %s.track SET picture = NULL WHERE picture IS NOT NULL`, s.config.Database.Schema)
	}

	result, err := s.db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("kon %s afbeeldingen niet verwijderen: %w", scopeDesc(scope), err)
	}

	countResult.Deleted, _ = result.RowsAffected()
	return countResult, nil
}

func DecodeBase64Image(data string) ([]byte, error) {
	// Remove data URL prefix if present (e.g., "data:image/jpeg;base64,")
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}

func (s *ImageService) GetStatistics(scope string) (*Statistics, error) {
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
		return nil, fmt.Errorf("kon items met afbeeldingen niet tellen: %w", err)
	}

	withoutImages, err := countItems(s.db, s.config.Database.Schema, table, false)
	if err != nil {
		return nil, fmt.Errorf("kon items zonder afbeeldingen niet tellen: %w", err)
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

	return &Statistics{
		Total:         total,
		WithImages:    withImages,
		WithoutImages: withoutImages,
		Orphaned:      orphaned,
	}, nil
}

func (s *ImageService) GetPreviewItems(scope string, limit int) ([]string, error) {
	result, err := s.ListWithFilter(scope, true, limit)
	if err != nil {
		return nil, err
	}

	var items []string
	if scope == ScopeArtist {
		artists := result.Items.([]Artist)
		for _, artist := range artists {
			items = append(items, artist.Name)
		}
	} else {
		tracks := result.Items.([]Track)
		for _, track := range tracks {
			items = append(items, fmt.Sprintf("%s - %s", track.Artist, track.Title))
		}
	}

	return items, nil
}
