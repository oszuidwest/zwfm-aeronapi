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
	ItemName       string
	ItemTitle      string
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

func (s *ImageService) ValidateImageUploadParams(params *ImageUploadParams) error {
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
	if err := s.ValidateImageUploadParams(params); err != nil {
		return nil, err
	}

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

	processingResult, err := processAndOptimizeImage(imageData, s.config.Image)
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

func (s *ImageService) ListWithoutImages(scope string, limit int) (interface{}, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	if scope == ScopeArtist {
		return listArtists(s.db, s.config.Database.Schema, false, limit)
	}
	return listTracks(s.db, s.config.Database.Schema, false, limit)
}

func (s *ImageService) Search(scope, searchTerm string) (interface{}, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	if scope == ScopeArtist {
		return findArtistsWithPartialName(s.db, s.config.Database.Schema, searchTerm)
	}
	return findTracksWithPartialName(s.db, s.config.Database.Schema, searchTerm)
}

type NukeResult struct {
	Count   int
	Deleted int64
}

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

func (s *ImageService) NukeImages(scope string) (*NukeResult, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, err
	}

	countResult, err := s.CountImagesForNuke(scope)
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
		return nil, fmt.Errorf("kon %s afbeeldingen niet verwijderen: %w", getScopeDescription(scope), err)
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
