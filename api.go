package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type AeronAPI struct {
	service *ImageService
	config  *Config
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type ImageUploadRequest struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	URL   string `json:"url"`
	Image string `json:"image"`
}

type ImageStatsResponse struct {
	Total         int `json:"total"`
	WithImages    int `json:"with_images"`
	WithoutImages int `json:"without_images"`
	Orphaned      int `json:"orphaned"`
}

func NewAeronAPI(service *ImageService, config *Config) *AeronAPI {
	return &AeronAPI{
		service: service,
		config:  config,
	}
}

func (s *AeronAPI) Start(port string) error {
	mux := http.NewServeMux()

	wrap := func(method string, handler http.HandlerFunc, requireAuth bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			fmt.Printf("[%s] %s\n", r.Method, r.URL.Path)
			if r.Method != method {
				s.sendError(w, "Methode niet toegestaan", http.StatusMethodNotAllowed)
				return
			}

			if requireAuth && s.config.API.Enabled {
				apiKey := r.Header.Get("X-API-Key")
				if apiKey == "" {
					apiKey = r.URL.Query().Get("key")
				}

				if !s.isValidAPIKey(apiKey) {
					s.sendError(w, "Niet geautoriseerd: Ongeldige of ontbrekende API-sleutel", http.StatusUnauthorized)
					return
				}
			}

			handler(w, r)
		}
	}

	mux.HandleFunc("/api/health", wrap(http.MethodGet, s.handleHealth, false))
	mux.HandleFunc("/api/artists", wrap(http.MethodGet, s.handleArtists, true))
	mux.HandleFunc("/api/artists/upload", wrap(http.MethodPost, s.handleArtistUpload, true))
	mux.HandleFunc("/api/artists/bulk-delete", wrap(http.MethodDelete, s.handleArtistBulkDelete, true))

	mux.HandleFunc("/api/tracks", wrap(http.MethodGet, s.handleTracks, true))
	mux.HandleFunc("/api/tracks/upload", wrap(http.MethodPost, s.handleTrackUpload, true))
	mux.HandleFunc("/api/tracks/bulk-delete", wrap(http.MethodDelete, s.handleTrackBulkDelete, true))

	mux.HandleFunc("/api/playlist", wrap(http.MethodGet, s.handlePlaylist, true))

	fmt.Printf("API Server gestart op poort %s\n", port)
	return http.ListenAndServe(":"+port, mux)
}

func (s *AeronAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendSuccess(w, map[string]string{
		"status":   "healthy",
		"version":  Version,
		"database": s.service.config.Database.Name,
	})
}

// stats handles statistics requests for any scope
func (s *AeronAPI) stats(w http.ResponseWriter, r *http.Request, scope string) {
	stats, err := s.service.GetStatistics(scope)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := ImageStatsResponse{
		Total:         stats.Total,
		WithImages:    stats.WithImages,
		WithoutImages: stats.WithoutImages,
		Orphaned:      stats.Orphaned,
	}

	s.sendSuccess(w, response)
}

func (s *AeronAPI) handleArtists(w http.ResponseWriter, r *http.Request) {
	s.stats(w, r, ScopeArtist)
}

// upload handles image upload requests for any scope
func (s *AeronAPI) upload(w http.ResponseWriter, r *http.Request, scope string) {
	var req ImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Ongeldige aanvraag body", http.StatusBadRequest)
		return
	}

	params := &ImageUploadParams{
		Scope: scope,
		Name:  req.Name,
		ID:    req.ID,
		URL:   req.URL,
	}

	if req.Image != "" {
		imageData, err := decodeBase64(req.Image)
		if err != nil {
			s.sendError(w, "Ongeldige base64 afbeelding", http.StatusBadRequest)
			return
		}
		params.ImageData = imageData
	}

	result, err := s.service.UploadImage(params)
	if err != nil {
		statusCode := s.errorCode(err)
		s.sendError(w, err.Error(), statusCode)
		return
	}

	response := s.uploadResponse(result, scope)
	s.sendSuccess(w, response)
}

// errorCode returns the appropriate HTTP status code for an error
func (s *AeronAPI) errorCode(err error) int {
	errorMsg := err.Error()

	// 404 Not Found errors
	if strings.Contains(errorMsg, ErrSuffixNotExists) {
		return http.StatusNotFound
	}

	// 400 Bad Request errors
	if errorMsg == "moet naam of id specificeren" ||
		errorMsg == "kan niet zowel naam als id specificeren" ||
		errorMsg == "afbeelding verplicht" ||
		errorMsg == "gebruik URL of upload, niet beide" ||
		strings.Contains(errorMsg, "ongeldig type") ||
		strings.Contains(errorMsg, "te klein") ||
		strings.Contains(errorMsg, "niet ondersteund") ||
		strings.Contains(errorMsg, "ongeldige") {
		return http.StatusBadRequest
	}

	// 500 Internal Server Error for everything else
	return http.StatusInternalServerError
}

// uploadResponse creates the response object for upload results
func (s *AeronAPI) uploadResponse(result *ImageUploadResult, scope string) map[string]interface{} {
	response := map[string]interface{}{
		"original_size":   result.OriginalSize,
		"optimized_size":  result.OptimizedSize,
		"savings_percent": result.SavingsPercent,
		"encoder":         result.Encoder,
	}

	if scope == ScopeArtist {
		response["artist"] = result.ItemName
	} else {
		response["track"] = result.ItemTitle
		response["artist"] = result.ItemName
	}

	return response
}

func (s *AeronAPI) handleArtistUpload(w http.ResponseWriter, r *http.Request) {
	s.upload(w, r, ScopeArtist)
}

func (s *AeronAPI) handleTracks(w http.ResponseWriter, r *http.Request) {
	s.stats(w, r, ScopeTrack)
}

func (s *AeronAPI) handleTrackUpload(w http.ResponseWriter, r *http.Request) {
	s.upload(w, r, ScopeTrack)
}

// bulkDelete handles image deletion requests for any scope
func (s *AeronAPI) bulkDelete(w http.ResponseWriter, r *http.Request, scope string) {
	const confirmHeader = "X-Confirm-Bulk-Delete"
	const confirmValue = "VERWIJDER ALLES"

	if r.Header.Get(confirmHeader) != confirmValue {
		s.sendError(w, "Ontbrekende bevestigingsheader: "+confirmHeader, http.StatusBadRequest)
		return
	}

	result, err := s.service.DeleteAllImages(scope)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	itemType := ItemTypeArtist
	if scope == ScopeTrack {
		itemType = ItemTypeTrack
	}

	s.sendSuccess(w, map[string]interface{}{
		"deleted": result.Deleted,
		"message": fmt.Sprintf("%d %s afbeeldingen verwijderd", result.Deleted, itemType),
	})
}

func (s *AeronAPI) handleArtistBulkDelete(w http.ResponseWriter, r *http.Request) {
	s.bulkDelete(w, r, ScopeArtist)
}

func (s *AeronAPI) handleTrackBulkDelete(w http.ResponseWriter, r *http.Request) {
	s.bulkDelete(w, r, ScopeTrack)
}

func (s *AeronAPI) sendSuccess(w http.ResponseWriter, data interface{}) {
	response := APIResponse{
		Success: true,
		Data:    data,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *AeronAPI) sendError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *AeronAPI) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := defaultPlaylistOptions()

	// Date parameter
	if date := r.URL.Query().Get("date"); date != "" {
		opts.Date = date
	}

	// Time range
	if from := r.URL.Query().Get("from"); from != "" {
		opts.StartTime = from
	}
	if to := r.URL.Query().Get("to"); to != "" {
		opts.EndTime = to
	}

	// Limit and offset
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			opts.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			opts.Offset = o
		}
	}

	// Track image filter
	if trackImage := r.URL.Query().Get("track_image"); trackImage != "" {
		opts.TrackImage = parseBoolParam(trackImage)
	}

	// Artist image filter
	if artistImage := r.URL.Query().Get("artist_image"); artistImage != "" {
		opts.ArtistImage = parseBoolParam(artistImage)
	}

	// Sort options
	if sort := r.URL.Query().Get("sort"); sort != "" {
		opts.SortBy = sort
	}
	if r.URL.Query().Get("desc") == "true" {
		opts.SortDesc = true
	}

	playlist, err := s.service.GetPlaylist(opts)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.sendSuccess(w, playlist)
}

func (s *AeronAPI) isValidAPIKey(key string) bool {
	if key == "" {
		return false
	}

	for _, validKey := range s.config.API.Keys {
		if key == validKey {
			return true
		}
	}
	return false
}

// parseBoolParam parses a boolean query parameter
func parseBoolParam(value string) *bool {
	switch value {
	case "yes", "true", "1":
		val := true
		return &val
	case "no", "false", "0":
		val := false
		return &val
	}
	return nil
}
