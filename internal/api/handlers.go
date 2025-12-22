// Package api provides the HTTP API server for the Aeron radio automation system.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/oszuidwest/zwfm-aeronapi/internal/database"
	"github.com/oszuidwest/zwfm-aeronapi/internal/service"
	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
	"github.com/oszuidwest/zwfm-aeronapi/internal/util"
)

// ImageUploadRequest represents the JSON request body for image upload operations.
// Either URL or Image should be provided, but not both.
type ImageUploadRequest struct {
	URL   string `json:"url"`   // URL of the image to download and process
	Image string `json:"image"` // Base64-encoded image data
}

// ImageStatsResponse represents the response format for statistics endpoints.
// It provides counts of entities with and without images.
type ImageStatsResponse struct {
	Total         int `json:"total"`          // Total number of entities
	WithImages    int `json:"with_images"`    // Number of entities with images
	WithoutImages int `json:"without_images"` // Number of entities without images
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Ping database to verify connectivity
	dbStatus := "connected"
	if err := s.service.DB().PingContext(r.Context()); err != nil {
		dbStatus = "disconnected"
		slog.Warn("Database health check mislukt", "error", err)
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":          "healthy",
		"version":         Version,
		"database":        s.service.Config().Database.Name,
		"database_status": dbStatus,
	})
}

// handleStats returns a handler for statistics requests
func (s *Server) handleStats(scope types.Scope) http.HandlerFunc {
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
func (s *Server) handleEntityByID(scope types.Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := types.GetEntityType(string(scope))

		if err := util.ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		var data interface{}
		var err error
		if scope == types.ScopeArtist {
			data, err = database.GetArtistByID(r.Context(), s.service.DB(), s.config.Database.Schema, entityID)
		} else {
			data, err = database.GetTrackByID(r.Context(), s.service.DB(), s.config.Database.Schema, entityID)
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
func (s *Server) uploadResponse(result *service.ImageUploadResult, scope types.Scope) map[string]interface{} {
	response := map[string]interface{}{
		"original_size":   result.OriginalSize,
		"optimized_size":  result.OptimizedSize,
		"savings_percent": result.SavingsPercent,
		"encoder":         result.Encoder,
	}

	if scope == types.ScopeArtist {
		response["artist"] = result.ItemName
	} else {
		response["track"] = result.ItemTitle
		response["artist"] = result.ItemName
	}

	return response
}

// handleBulkDelete returns a handler for bulk image deletion
func (s *Server) handleBulkDelete(scope types.Scope) http.HandlerFunc {
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

		itemType := types.ItemTypeArtist
		if scope == types.ScopeTrack {
			itemType = types.ItemTypeTrack
		}

		message := strconv.FormatInt(result.Deleted, 10) + " " + itemType + "-afbeeldingen verwijderd"
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"deleted": result.Deleted,
			"message": message,
		})
	}
}

// handleGetImage returns a handler for retrieving images
func (s *Server) handleGetImage(scope types.Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := types.GetEntityType(string(scope))

		if err := util.ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		table := types.TableForScope(scope)
		imageData, err := database.GetEntityImage(r.Context(), s.service.DB(), s.config.Database.Schema, table, entityID)

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
func (s *Server) handleImageUpload(scope types.Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := types.GetEntityType(string(scope))

		if err := util.ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		var req ImageUploadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
			return
		}

		// For image endpoint, we only use ID - ignore any name field
		params := &service.ImageUploadParams{
			Scope: scope,
			ID:    entityID,
			URL:   req.URL,
		}

		if req.Image != "" {
			imageData, err := service.DecodeBase64(req.Image)
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
func (s *Server) handleDeleteImage(scope types.Scope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		entityType := types.GetEntityType(string(scope))

		if err := util.ValidateEntityID(entityID, entityType); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		table := types.TableForScope(scope)

		err := database.DeleteEntityImage(r.Context(), s.service.DB(), s.config.Database.Schema, table, entityID)
		if err != nil {
			slog.Error("Afbeelding verwijderen mislukt", "scope", scope, "id", entityID, "error", err)
		}

		if err != nil {
			statusCode := errorCode(err)
			respondError(w, statusCode, err.Error())
			return
		}

		idField := "artist_id"
		if scope == types.ScopeTrack {
			idField = "track_id"
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": entityType + "-afbeelding succesvol verwijderd",
			idField:   entityID,
		})
	}
}

func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	blockID := query.Get("block_id")

	// If block_id is provided, return tracks for that specific block
	if blockID != "" {
		opts := database.DefaultPlaylistOptions()
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

		playlist, err := database.GetPlaylist(r.Context(), s.service.DB(), s.config.Database.Schema, opts)
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
	blocks, tracksByBlock, err := database.GetPlaylistBlocksWithTracks(r.Context(), s.service.DB(), s.config.Database.Schema, date)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build the response
	type BlockWithTracks struct {
		database.PlaylistBlock
		Tracks []database.PlaylistItem `json:"tracks"`
	}

	result := make([]BlockWithTracks, len(blocks))
	for i, block := range blocks {
		tracks := tracksByBlock[block.BlockID]
		if tracks == nil {
			tracks = []database.PlaylistItem{}
		}
		result[i] = BlockWithTracks{
			PlaylistBlock: block,
			Tracks:        tracks,
		}
	}

	respondJSON(w, http.StatusOK, result)
}
