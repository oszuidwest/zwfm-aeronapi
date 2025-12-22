package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// AeronAPI represents the HTTP API server for the Aeron radio automation system.
// It provides REST endpoints for managing artist and track images in the Aeron database.
// The API supports image upload, retrieval, deletion, and statistics operations.
type AeronAPI struct {
	service *AeronService
	config  *Config
}

// ImageUploadRequest represents the JSON request body for image upload operations.
// Either URL or Image should be provided, but not both.
type ImageUploadRequest struct {
	URL   string `json:"url"`   // URL of the image to download and process
	Image string `json:"image"` // Base64-encoded image data
}

// VacuumRequest represents the JSON request body for vacuum operations.
type VacuumRequest struct {
	Tables  []string `json:"tables"`  // Specific tables to vacuum (empty = auto-select based on bloat)
	Analyze bool     `json:"analyze"` // Run ANALYZE after VACUUM
	DryRun  bool     `json:"dry_run"` // Only show what would be done, don't execute
}

// AnalyzeRequest represents the JSON request body for analyze operations.
type AnalyzeRequest struct {
	Tables []string `json:"tables"` // Specific tables to analyze (empty = auto-select)
}

// ImageStatsResponse represents the response format for statistics endpoints.
// It provides counts of entities with and without images.
type ImageStatsResponse struct {
	Total         int `json:"total"`          // Total number of entities
	WithImages    int `json:"with_images"`    // Number of entities with images
	WithoutImages int `json:"without_images"` // Number of entities without images
}

// NewAeronAPI creates a new AeronAPI instance with the provided service and configuration.
// The service handles business logic while config contains API authentication settings.
func NewAeronAPI(service *AeronService, config *Config) *AeronAPI {
	return &AeronAPI{
		service: service,
		config:  config,
	}
}

// Start initializes and starts the HTTP server on the specified port.
// It configures middleware, routes, and begins listening for incoming requests.
// Returns an error if the server fails to start.
func (s *AeronAPI) Start(port string) error {
	router := chi.NewRouter()

	// Add Chi built-in middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RealIP)
	router.Use(middleware.Compress(5))
	router.Use(middleware.Timeout(30 * time.Second))

	// API routes
	router.Route("/api", func(r chi.Router) {
		// JSON content type for all API routes (except images)
		r.Use(middleware.SetHeader("Content-Type", "application/json; charset=utf-8"))

		// Health check - no auth required
		r.Get("/health", s.handleHealth)

		// Protected routes - auth required
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			// Generic entity routes for both artists and tracks
			s.setupEntityRoutes(r, "/artists", ScopeArtist)
			s.setupEntityRoutes(r, "/tracks", ScopeTrack)

			// Playlist
			r.Get("/playlist", s.handlePlaylist)

			// Database maintenance
			r.Route("/db", func(r chi.Router) {
				r.Get("/health", s.handleDatabaseHealth)
				r.Post("/vacuum", s.handleVacuum)
				r.Post("/analyze", s.handleAnalyze)
			})
		})
	})

	// API server is now listening
	return http.ListenAndServe(":"+port, router)
}

// setupEntityRoutes configures routes for an entity type (artist/track)
func (s *AeronAPI) setupEntityRoutes(r chi.Router, path string, scope string) {
	r.Route(path, func(r chi.Router) {
		r.Get("/", s.handleStats(scope))
		r.Delete("/bulk-delete", s.handleBulkDelete(scope))

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", s.handleEntityByID(scope))
			r.Route("/image", func(r chi.Router) {
				r.Get("/", s.handleGetImage(scope))
				r.Post("/", s.handleImageUpload(scope))
				r.Delete("/", s.handleDeleteImage(scope))
			})
		})
	})
}

// authMiddleware provides API key authentication
func (s *AeronAPI) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config.API.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get API key
		apiKey := r.Header.Get("X-API-Key")

		if !s.isValidAPIKey(apiKey) {
			slog.Warn("Authenticatie mislukt",
				"reason", "ongeldige_api_key",
				"path", r.URL.Path,
				"method", r.Method,
				"remote_addr", r.RemoteAddr)

			respondError(w, http.StatusUnauthorized, "Niet geautoriseerd: ongeldige of ontbrekende API-sleutel")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *AeronAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":   "healthy",
		"version":  Version,
		"database": s.service.config.Database.Name,
	})
}

// handleStats returns a handler for statistics requests
func (s *AeronAPI) handleStats(scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.service.GetStatistics(scope)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		response := ImageStatsResponse{
			Total:         stats.Total,
			WithImages:    stats.WithImages,
			WithoutImages: stats.WithoutImages,
		}

		respondJSON(w, http.StatusOK, response)
	}
}

// handleEntityByID returns a handler for retrieving entity details
func (s *AeronAPI) handleEntityByID(scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := GetEntityType(scope)

		if err := ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		var data interface{}
		var err error
		if scope == ScopeArtist {
			data, err = getArtistByID(s.service.db, s.config.Database.Schema, entityID)
		} else {
			data, err = getTrackByID(s.service.db, s.config.Database.Schema, entityID)
		}

		if err != nil {
			statusCode := errorCode(err)
			respondError(w, statusCode, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, data)
	}
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

// handleBulkDelete returns a handler for bulk image deletion
func (s *AeronAPI) handleBulkDelete(scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const confirmHeader = "X-Confirm-Bulk-Delete"
		const confirmValue = "VERWIJDER ALLES"

		if r.Header.Get(confirmHeader) != confirmValue {
			respondError(w, http.StatusBadRequest, "Ontbrekende bevestigingsheader: "+confirmHeader)
			return
		}

		result, err := s.service.DeleteAllImages(scope)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		itemType := ItemTypeArtist
		if scope == ScopeTrack {
			itemType = ItemTypeTrack
		}

		message := strconv.FormatInt(result.Deleted, 10) + " " + itemType + "-afbeeldingen verwijderd"
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"deleted": result.Deleted,
			"message": message,
		})
	}
}

// handleGetImage returns a handler for retrieving images
func (s *AeronAPI) handleGetImage(scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := GetEntityType(scope)

		if err := ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		table := tableArtist
		if scope == ScopeTrack {
			table = tableTrack
		}
		imageData, err := getEntityImage(s.service.db, s.config.Database.Schema, table, entityID)

		if err != nil {
			statusCode := errorCode(err)
			respondError(w, statusCode, err.Error())
			return
		}

		// Override content type for image response
		w.Header().Del("Content-Type") // Remove JSON header set by middleware
		w.Header().Set("Content-Type", detectImageContentType(imageData))
		w.Header().Set("Content-Length", strconv.Itoa(len(imageData)))

		// Write image data directly
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(imageData); err != nil {
			slog.Debug("Schrijven afbeelding naar client mislukt", "error", err)
		}
	}
}

// handleImageUpload returns a handler for image uploads
func (s *AeronAPI) handleImageUpload(scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := GetEntityType(scope)

		if err := ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		var req ImageUploadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
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
				respondError(w, http.StatusBadRequest, "Ongeldige base64-afbeelding")
				return
			}
			params.ImageData = imageData
		}

		result, err := s.service.UploadImage(params)
		if err != nil {
			statusCode := errorCode(err)
			respondError(w, statusCode, err.Error())
			return
		}

		response := s.uploadResponse(result, scope)
		respondJSON(w, http.StatusOK, response)
	}
}

// handleDeleteImage returns a handler for deleting images
func (s *AeronAPI) handleDeleteImage(scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := GetEntityType(scope)

		if err := ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		table := tableArtist
		logMsg := "Artiest afbeelding verwijderen mislukt"
		logField := "artist_id"
		if scope == ScopeTrack {
			table = tableTrack
			logMsg = "Track afbeelding verwijderen mislukt"
			logField = "track_id"
		}

		err := deleteEntityImage(s.service.db, s.config.Database.Schema, table, entityID)
		if err != nil {
			slog.Error(logMsg, logField, entityID, "error", err)
		}

		if err != nil {
			statusCode := errorCode(err)
			respondError(w, statusCode, err.Error())
			return
		}

		idField := "artist_id"
		if scope == ScopeTrack {
			idField = "track_id"
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": entityType + "-afbeelding succesvol verwijderd",
			idField:   entityID,
		})
	}
}

func (s *AeronAPI) handleDatabaseHealth(w http.ResponseWriter, r *http.Request) {
	health, err := s.service.GetDatabaseHealth()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, health)
}

func (s *AeronAPI) handleVacuum(w http.ResponseWriter, r *http.Request) {
	var req VacuumRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
		return
	}

	opts := VacuumOptions{
		Tables:  req.Tables,
		Analyze: req.Analyze,
		DryRun:  req.DryRun,
	}

	result, err := s.service.VacuumTables(opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *AeronAPI) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
		return
	}

	result, err := s.service.AnalyzeTables(req.Tables)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *AeronAPI) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	blockID := query.Get("block_id")

	// If block_id is provided, return tracks for that specific block
	if blockID != "" {
		opts := defaultPlaylistOptions()
		opts.BlockID = blockID

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

		playlist, err := getPlaylist(s.service.db, s.config.Database.Schema, opts)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, playlist)
		return
	}

	// No block_id: return all blocks with their tracks for today (or specified date)
	date := query.Get("date")

	// Get all blocks and tracks in just 2 queries (optimized)
	blocks, tracksByBlock, err := getPlaylistBlocksWithTracks(s.service.db, s.config.Database.Schema, date)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build the response
	type BlockWithTracks struct {
		PlaylistBlock
		Tracks []PlaylistItem `json:"tracks"`
	}

	result := make([]BlockWithTracks, len(blocks))
	for i, block := range blocks {
		tracks := tracksByBlock[block.BlockID]
		if tracks == nil {
			tracks = []PlaylistItem{}
		}
		result[i] = BlockWithTracks{
			PlaylistBlock: block,
			Tracks:        tracks,
		}
	}

	respondJSON(w, http.StatusOK, result)
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
