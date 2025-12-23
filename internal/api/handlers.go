// Package api provides the HTTP API server for the Aeron radio automation system.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/database"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/service"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/types"
	"github.com/oszuidwest/zwfm-aerontoolbox/internal/util"
)

// ImageUploadRequest represents the JSON request body for image upload operations.
type ImageUploadRequest struct {
	URL   string `json:"url"`
	Image string `json:"image"`
}

// ImageStatsResponse represents the response format for statistics endpoints.
type ImageStatsResponse struct {
	Total         int `json:"total"`
	WithImages    int `json:"with_images"`
	WithoutImages int `json:"without_images"`
}

// BlockWithTracks represents a playlist block with its associated tracks.
type BlockWithTracks struct {
	database.PlaylistBlock
	Tracks []database.PlaylistItem `json:"tracks"`
}

// HealthResponse represents the response for the health check endpoint.
type HealthResponse struct {
	Status         string `json:"status"`
	Version        string `json:"version"`
	Database       string `json:"database"`
	DatabaseStatus string `json:"database_status"`
}

// ImageUploadResponse represents the response for image upload operations.
type ImageUploadResponse struct {
	Artist               string  `json:"artist"`
	Track                string  `json:"track,omitempty"`
	OriginalSize         int     `json:"original_size"`
	OptimizedSize        int     `json:"optimized_size"`
	SizeReductionPercent float64 `json:"savings_percent"`
}

// BulkDeleteResponse represents the response for bulk delete operations.
type BulkDeleteResponse struct {
	Deleted int64  `json:"deleted"`
	Message string `json:"message"`
}

// ImageDeleteResponse represents the response for image delete operations.
type ImageDeleteResponse struct {
	Message  string `json:"message"`
	ArtistID string `json:"artist_id,omitempty"`
	TrackID  string `json:"track_id,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbStatus := "connected"
	if err := s.service.DB().PingContext(r.Context()); err != nil {
		dbStatus = "disconnected"
		slog.Warn("Database health check mislukt", "error", err)
	}

	respondJSON(w, http.StatusOK, HealthResponse{
		Status:         "healthy",
		Version:        Version,
		Database:       s.service.Config().Database.Name,
		DatabaseStatus: dbStatus,
	})
}

func (s *Server) handleStats(entityType types.EntityType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.service.GetStatistics(r.Context(), entityType)
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

func (s *Server) handleEntityByID(entityType types.EntityType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		label := types.LabelForEntityType(entityType)

		if err := util.ValidateEntityID(entityID, label); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		if entityType == types.EntityTypeArtist {
			artist, err := database.GetArtistByID(r.Context(), s.service.DB(), s.config.Database.Schema, entityID)
			if err != nil {
				statusCode := errorCode(err)
				respondError(w, statusCode, err.Error())
				return
			}
			respondJSON(w, http.StatusOK, artist)
		} else {
			track, err := database.GetTrackByID(r.Context(), s.service.DB(), s.config.Database.Schema, entityID)
			if err != nil {
				statusCode := errorCode(err)
				respondError(w, statusCode, err.Error())
				return
			}
			respondJSON(w, http.StatusOK, track)
		}
	}
}

func (s *Server) uploadResponse(result *service.ImageUploadResult, entityType types.EntityType) ImageUploadResponse {
	response := ImageUploadResponse{
		Artist:               result.ArtistName,
		OriginalSize:         result.OriginalSize,
		OptimizedSize:        result.OptimizedSize,
		SizeReductionPercent: result.SizeReductionPercent,
	}

	if entityType == types.EntityTypeTrack {
		response.Track = result.TrackTitle
	}

	return response
}

func (s *Server) handleBulkDelete(entityType types.EntityType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const confirmHeader = "X-Confirm-Bulk-Delete"
		const confirmValue = "VERWIJDER ALLES"

		if r.Header.Get(confirmHeader) != confirmValue {
			respondError(w, http.StatusBadRequest, "Ontbrekende bevestigingsheader: "+confirmHeader)
			return
		}

		result, err := s.service.DeleteAllImages(r.Context(), entityType)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		label := types.LabelForEntityType(entityType)

		message := strconv.FormatInt(result.DeletedCount, 10) + " " + label + "-afbeeldingen verwijderd"
		respondJSON(w, http.StatusOK, BulkDeleteResponse{
			Deleted: result.DeletedCount,
			Message: message,
		})
	}
}

func (s *Server) handleGetImage(entityType types.EntityType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		label := types.LabelForEntityType(entityType)

		if err := util.ValidateEntityID(entityID, label); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		table := types.TableForEntityType(entityType)
		imageData, err := database.GetEntityImage(r.Context(), s.service.DB(), s.config.Database.Schema, table, entityID)

		if err != nil {
			statusCode := errorCode(err)
			respondError(w, statusCode, err.Error())
			return
		}

		w.Header().Del("Content-Type")
		w.Header().Set("Content-Type", detectImageContentType(imageData))
		w.Header().Set("Content-Length", strconv.Itoa(len(imageData)))

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(imageData); err != nil {
			slog.Debug("Schrijven afbeelding naar client mislukt", "error", err)
		}
	}
}

func (s *Server) handleImageUpload(entityType types.EntityType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		label := types.LabelForEntityType(entityType)

		if err := util.ValidateEntityID(entityID, label); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		var req ImageUploadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Ongeldige aanvraaginhoud")
			return
		}

		params := &service.ImageUploadParams{
			EntityType: entityType,
			ID:         entityID,
			ImageURL:   req.URL,
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

		response := s.uploadResponse(result, entityType)
		respondJSON(w, http.StatusOK, response)
	}
}

func (s *Server) handleDeleteImage(entityType types.EntityType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "id")
		label := types.LabelForEntityType(entityType)

		if err := util.ValidateEntityID(entityID, label); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		table := types.TableForEntityType(entityType)

		err := database.DeleteEntityImage(r.Context(), s.service.DB(), s.config.Database.Schema, table, entityID)
		if err != nil {
			slog.Error("Afbeelding verwijderen mislukt", "entityType", entityType, "id", entityID, "error", err)
		}

		if err != nil {
			statusCode := errorCode(err)
			respondError(w, statusCode, err.Error())
			return
		}

		response := ImageDeleteResponse{
			Message: label + "-afbeelding succesvol verwijderd",
		}
		if entityType == types.EntityTypeArtist {
			response.ArtistID = entityID
		} else {
			response.TrackID = entityID
		}

		respondJSON(w, http.StatusOK, response)
	}
}

func parsePlaylistOptions(query url.Values) database.PlaylistOptions {
	opts := database.DefaultPlaylistOptions()
	opts.BlockID = query.Get("block_id")

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

	if trackImage := query.Get("track_image"); trackImage != "" {
		opts.TrackImage = parseQueryBoolParam(trackImage)
	}
	if artistImage := query.Get("artist_image"); artistImage != "" {
		opts.ArtistImage = parseQueryBoolParam(artistImage)
	}

	if sort := query.Get("sort"); sort != "" {
		opts.SortBy = sort
	}
	if query.Get("desc") == "true" {
		opts.SortDesc = true
	}

	return opts
}

func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Single block with items
	if query.Get("block_id") != "" {
		opts := parsePlaylistOptions(query)
		playlist, err := database.GetPlaylist(r.Context(), s.service.DB(), s.config.Database.Schema, &opts)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, playlist)
		return
	}

	// All blocks with tracks for a date
	blocks, tracksByBlock, err := database.GetPlaylistBlocksWithTracks(r.Context(), s.service.DB(), s.config.Database.Schema, query.Get("date"))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]BlockWithTracks, len(blocks))
	for i := range blocks {
		tracks := tracksByBlock[blocks[i].BlockID]
		if tracks == nil {
			tracks = []database.PlaylistItem{}
		}
		result[i] = BlockWithTracks{
			PlaylistBlock: blocks[i],
			Tracks:        tracks,
		}
	}

	respondJSON(w, http.StatusOK, result)
}
