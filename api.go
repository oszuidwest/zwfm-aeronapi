package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// AeronAPI represents the HTTP API server for the Aeron radio automation system.
// It provides REST endpoints for managing artist and track images in the Aeron database.
// The API supports image upload, retrieval, deletion, and statistics operations.
type AeronAPI struct {
	service *AeronService
	config  *Config
	server  *http.Server
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
	router.Use(middleware.Timeout(s.config.API.GetRequestTimeout()))

	// Global 404 handler - returns JSON for consistency
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		respondError(w, http.StatusNotFound, "Endpoint niet gevonden")
	})

	// API routes
	router.Route("/api", func(r chi.Router) {
		// JSON content type for all API routes (except images)
		r.Use(middleware.SetHeader("Content-Type", "application/json; charset=utf-8"))

		// Custom 404 handler for API routes - returns JSON instead of plain text
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			respondError(w, http.StatusNotFound, "Endpoint niet gevonden")
		})

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

				// Backup endpoints
				r.Post("/backup", s.handleCreateBackup)
				r.Get("/backup/download", s.handleBackupDownload)
				r.Get("/backups", s.handleListBackups)
				r.Get("/backups/{filename}", s.handleDownloadBackupFile)
				r.Delete("/backups/{filename}", s.handleDeleteBackup)
			})
		})
	})

	// Create server for graceful shutdown support
	s.server = &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *AeronAPI) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// setupEntityRoutes configures routes for an entity type (artist/track)
func (s *AeronAPI) setupEntityRoutes(r chi.Router, path string, scope Scope) {
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
	// Ping database to verify connectivity
	dbStatus := "connected"
	if err := s.service.db.PingContext(r.Context()); err != nil {
		dbStatus = "disconnected"
		slog.Warn("Database health check mislukt", "error", err)
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":          "healthy",
		"version":         Version,
		"database":        s.service.config.Database.Name,
		"database_status": dbStatus,
	})
}

// handleStats returns a handler for statistics requests
func (s *AeronAPI) handleStats(scope Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.service.GetStatistics(r.Context(), scope)
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
func (s *AeronAPI) handleEntityByID(scope Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := GetEntityType(string(scope))

		if err := ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		var data interface{}
		var err error
		if scope == ScopeArtist {
			data, err = getArtistByID(r.Context(), s.service.db, s.config.Database.Schema, entityID)
		} else {
			data, err = getTrackByID(r.Context(), s.service.db, s.config.Database.Schema, entityID)
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
func (s *AeronAPI) uploadResponse(result *ImageUploadResult, scope Scope) map[string]interface{} {
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
func (s *AeronAPI) handleBulkDelete(scope Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const confirmHeader = "X-Confirm-Bulk-Delete"
		const confirmValue = "VERWIJDER ALLES"

		if r.Header.Get(confirmHeader) != confirmValue {
			respondError(w, http.StatusBadRequest, "Ontbrekende bevestigingsheader: "+confirmHeader)
			return
		}

		result, err := s.service.DeleteAllImages(r.Context(), scope)
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
func (s *AeronAPI) handleGetImage(scope Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := GetEntityType(string(scope))

		if err := ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		table := TableForScope(scope)
		imageData, err := getEntityImage(r.Context(), s.service.db, s.config.Database.Schema, table, entityID)

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
func (s *AeronAPI) handleImageUpload(scope Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := GetEntityType(string(scope))

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

		result, err := s.service.UploadImage(r.Context(), params)
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
func (s *AeronAPI) handleDeleteImage(scope Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := GetEntityType(string(scope))

		if err := ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		table := TableForScope(scope)

		err := deleteEntityImage(r.Context(), s.service.db, s.config.Database.Schema, table, entityID)
		if err != nil {
			slog.Error("Afbeelding verwijderen mislukt", "scope", scope, "id", entityID, "error", err)
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
	health, err := s.service.GetDatabaseHealth(r.Context())
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

	result, err := s.service.VacuumTables(r.Context(), VacuumOptions(req))
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

	result, err := s.service.AnalyzeTables(r.Context(), req.Tables)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *AeronAPI) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	var req BackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
		return
	}

	result, err := s.service.CreateBackup(r.Context(), req)
	if err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *AeronAPI) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	format := query.Get("format")
	if format == "" {
		format = s.config.Backup.GetDefaultFormat()
	}

	compression := s.config.Backup.GetDefaultCompression()
	if c := query.Get("compression"); c != "" {
		if val, err := strconv.Atoi(c); err == nil {
			compression = val
		}
	}

	// Generate filename for download
	timestamp := "download"
	var ext string
	if format == "custom" {
		ext = "dump"
	} else {
		ext = "sql"
	}
	filename := "aeron-backup-" + timestamp + "." + ext

	// Set headers for file download
	w.Header().Del("Content-Type")
	if format == "custom" {
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		w.Header().Set("Content-Type", "application/sql")
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)

	if err := s.service.StreamBackup(r.Context(), w, format, compression); err != nil {
		// Headers already sent, can't send error JSON
		slog.Error("Backup stream mislukt", "error", err)
	}
}

func (s *AeronAPI) handleListBackups(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.ListBackups()
	if err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (s *AeronAPI) handleDownloadBackupFile(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	filePath, err := s.service.GetBackupFilePath(filename)
	if err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	// Determine content type
	w.Header().Del("Content-Type")
	if len(filename) > 5 && filename[len(filename)-5:] == ".dump" {
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		w.Header().Set("Content-Type", "application/sql")
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)

	http.ServeFile(w, r, filePath)
}

func (s *AeronAPI) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	// Require confirmation header
	const confirmHeader = "X-Confirm-Delete"
	if r.Header.Get(confirmHeader) != filename {
		respondError(w, http.StatusBadRequest, "Bevestigingsheader ontbreekt: "+confirmHeader+" moet de bestandsnaam bevatten")
		return
	}

	if err := s.service.DeleteBackup(filename); err != nil {
		statusCode := errorCode(err)
		respondError(w, statusCode, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":  "Backup succesvol verwijderd",
		"filename": filename,
	})
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

		playlist, err := getPlaylist(r.Context(), s.service.db, s.config.Database.Schema, opts)
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
	blocks, tracksByBlock, err := getPlaylistBlocksWithTracks(r.Context(), s.service.db, s.config.Database.Schema, date)
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
