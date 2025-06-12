package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type AeronAPI struct {
	service *AeronService
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
}

func NewAeronAPI(service *AeronService, config *Config) *AeronAPI {
	return &AeronAPI{
		service: service,
		config:  config,
	}
}

func (s *AeronAPI) Start(port string) error {
	router := chi.NewRouter()

	// Add Chi built-in middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RealIP)
	router.Use(middleware.Compress(5))               // Replaces our gzip middleware
	router.Use(middleware.Timeout(30 * time.Second)) // Replaces our timeout middleware

	// Add custom middleware
	router.Use(s.corsMiddleware)

	// API routes
	router.Route("/api", func(r chi.Router) {
		// JSON content type for all API routes (except images)
		r.Use(middleware.SetHeader("Content-Type", "application/json; charset=utf-8"))

		// Health check - no auth required
		r.Get("/health", s.handleHealth)

		// Protected routes - auth required
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			// Artists subroute
			r.Route("/artists", func(r chi.Router) {
				r.Get("/", s.handleArtists)
				r.Delete("/bulk-delete", s.handleArtistBulkDelete)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", s.handleArtistByID)
					r.Route("/image", func(r chi.Router) {
						r.Get("/", s.handleGetArtistImage)
						r.Post("/", s.handlePostArtistImage)
						r.Delete("/", s.handleDeleteArtistImage)
					})
				})
			})

			// Tracks subroute
			r.Route("/tracks", func(r chi.Router) {
				r.Get("/", s.handleTracks)
				r.Post("/upload", s.handleTrackUpload)
				r.Delete("/bulk-delete", s.handleTrackBulkDelete)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", s.handleTrackByID)
					r.Route("/image", func(r chi.Router) {
						r.Get("/", s.handleTrackImage)
						r.Post("/", s.handleTrackImageUpload)
						r.Delete("/", s.handleDeleteTrackImage)
					})
				})
			})

			// Playlist
			r.Get("/playlist", s.handlePlaylist)
		})
	})

	fmt.Printf("API Server gestart op poort %s\n", port)
	return http.ListenAndServe(":"+port, router)
}

// authMiddleware provides API key authentication
func (s *AeronAPI) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config.API.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get API key from header or query parameter
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("key")
		}

		if !s.isValidAPIKey(apiKey) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(APIResponse{
				Success: false,
				Error:   "Niet geautoriseerd: Ongeldige of ontbrekende API-sleutel",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Helper functions for JSON responses
func (s *AeronAPI) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *AeronAPI) sendSuccess(w http.ResponseWriter, data interface{}) {
	s.sendJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

func (s *AeronAPI) sendError(w http.ResponseWriter, message string, code int) {
	s.sendJSON(w, code, APIResponse{
		Success: false,
		Error:   message,
	})
}

// corsMiddleware adds CORS headers for API access
func (s *AeronAPI) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, X-API-Key, X-Confirm-Bulk-Delete")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
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
	}

	s.sendSuccess(w, response)
}

func (s *AeronAPI) handleArtists(w http.ResponseWriter, r *http.Request) {
	s.stats(w, r, ScopeArtist)
}

func (s *AeronAPI) handleArtistByID(w http.ResponseWriter, r *http.Request) {
	artistID := chi.URLParam(r, "id")
	if artistID == "" {
		s.sendError(w, "Artiest ID verplicht", http.StatusBadRequest)
		return
	}

	artist, err := s.service.GetArtistByID(artistID)
	if err != nil {
		statusCode := s.errorCode(err)
		s.sendError(w, err.Error(), statusCode)
		return
	}

	s.sendSuccess(w, artist)
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
	if strings.Contains(errorMsg, ErrSuffixNotExists) ||
		strings.Contains(errorMsg, "heeft geen afbeelding") {
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

func (s *AeronAPI) handleTracks(w http.ResponseWriter, r *http.Request) {
	s.stats(w, r, ScopeTrack)
}

func (s *AeronAPI) handleTrackByID(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "id")
	if trackID == "" {
		s.sendError(w, "Track ID verplicht", http.StatusBadRequest)
		return
	}

	track, err := s.service.GetTrackByID(trackID)
	if err != nil {
		statusCode := s.errorCode(err)
		s.sendError(w, err.Error(), statusCode)
		return
	}

	s.sendSuccess(w, track)
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

// handleGetImage is a generic handler for retrieving artist and track images
func (s *AeronAPI) handleGetImage(w http.ResponseWriter, r *http.Request, scope string) {
	entityID := chi.URLParam(r, "id")
	if entityID == "" {
		entityType := "Artiest"
		if scope == ScopeTrack {
			entityType = "Track"
		}
		s.sendError(w, entityType+" ID verplicht", http.StatusBadRequest)
		return
	}

	var imageData []byte
	var err error
	if scope == ScopeArtist {
		imageData, err = s.service.GetArtistImage(entityID)
	} else {
		imageData, err = s.service.GetTrackImage(entityID)
	}

	if err != nil {
		statusCode := s.errorCode(err)
		s.sendError(w, err.Error(), statusCode)
		return
	}

	// Override content type for image response
	w.Header().Del("Content-Type") // Remove JSON header set by middleware
	w.Header().Set("Content-Type", detectImageContentType(imageData))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(imageData)))

	// Write image data directly
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(imageData)
}

func (s *AeronAPI) handleGetArtistImage(w http.ResponseWriter, r *http.Request) {
	s.handleGetImage(w, r, ScopeArtist)
}

// handleImageUpload is a generic handler for both artist and track image uploads
func (s *AeronAPI) handleImageUpload(w http.ResponseWriter, r *http.Request, scope string) {
	entityID := chi.URLParam(r, "id")
	if entityID == "" {
		entityType := "Artiest"
		if scope == ScopeTrack {
			entityType = "Track"
		}
		s.sendError(w, entityType+" ID verplicht", http.StatusBadRequest)
		return
	}

	var req ImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Ongeldige aanvraag body", http.StatusBadRequest)
		return
	}

	// For image endpoint, we only use ID - ignore any name field
	params := &ImageUploadParams{
		Scope: scope,
		ID:    entityID,
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

func (s *AeronAPI) handlePostArtistImage(w http.ResponseWriter, r *http.Request) {
	s.handleImageUpload(w, r, ScopeArtist)
}

func (s *AeronAPI) handleTrackImageUpload(w http.ResponseWriter, r *http.Request) {
	s.handleImageUpload(w, r, ScopeTrack)
}

func (s *AeronAPI) handleTrackImage(w http.ResponseWriter, r *http.Request) {
	s.handleGetImage(w, r, ScopeTrack)
}

// handleDeleteImage is a generic handler for deleting artist and track images
func (s *AeronAPI) handleDeleteImage(w http.ResponseWriter, r *http.Request, scope string) {
	entityID := chi.URLParam(r, "id")
	if entityID == "" {
		entityType := "Artiest"
		if scope == ScopeTrack {
			entityType = "Track"
		}
		s.sendError(w, entityType+" ID verplicht", http.StatusBadRequest)
		return
	}

	var err error
	if scope == ScopeArtist {
		err = s.service.DeleteArtistImage(entityID)
	} else {
		err = s.service.DeleteTrackImage(entityID)
	}

	if err != nil {
		statusCode := s.errorCode(err)
		s.sendError(w, err.Error(), statusCode)
		return
	}

	entityType := "Artiest"
	idField := "artist_id"
	if scope == ScopeTrack {
		entityType = "Track"
		idField = "track_id"
	}

	s.sendSuccess(w, map[string]string{
		"message": entityType + " afbeelding succesvol verwijderd",
		idField:   entityID,
	})
}

func (s *AeronAPI) handleDeleteArtistImage(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteImage(w, r, ScopeArtist)
}

func (s *AeronAPI) handleDeleteTrackImage(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteImage(w, r, ScopeTrack)
}

func (s *AeronAPI) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters manually (lighter than validation)
	opts := defaultPlaylistOptions()
	query := r.URL.Query()

	// Date parameter
	if date := query.Get("date"); date != "" {
		opts.Date = date
	}

	// Time range
	if from := query.Get("from"); from != "" {
		opts.StartTime = from
	}
	if to := query.Get("to"); to != "" {
		opts.EndTime = to
	}

	// Limit and offset
	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			opts.Limit = l
		}
	}
	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			opts.Offset = o
		}
	}

	// Track image filter
	if trackImage := query.Get("track_image"); trackImage != "" {
		opts.TrackImage = parseBoolParam(trackImage)
	}

	// Artist image filter
	if artistImage := query.Get("artist_image"); artistImage != "" {
		opts.ArtistImage = parseBoolParam(artistImage)
	}

	// Sort options
	if sort := query.Get("sort"); sort != "" {
		opts.SortBy = sort
	}
	if query.Get("desc") == "true" {
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

// detectImageContentType detects the content type from image data using Go's built-in detection
func detectImageContentType(data []byte) string {
	return http.DetectContentType(data)
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
