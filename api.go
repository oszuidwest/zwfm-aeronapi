package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTP status codes for clarity
const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusUnauthorized        = http.StatusUnauthorized
	StatusNotFound            = http.StatusNotFound
	StatusMethodNotAllowed    = http.StatusMethodNotAllowed
	StatusInternalServerError = http.StatusInternalServerError
)

type APIServer struct {
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

type ItemResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	HasImage bool   `json:"has_image"`
}

type StatisticsResponse struct {
	Total         int `json:"total"`
	WithImages    int `json:"with_images"`
	WithoutImages int `json:"without_images"`
	Orphaned      int `json:"orphaned"`
}

func NewAPIServer(service *ImageService, config *Config) *APIServer {
	return &APIServer{
		service: service,
		config:  config,
	}
}

func (s *APIServer) Start(port string) error {
	mux := http.NewServeMux()

	wrap := func(method string, handler http.HandlerFunc, requireAuth bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			fmt.Printf("[%s] %s\n", r.Method, r.URL.Path)
			if r.Method != method {
				s.sendError(w, "Method not allowed", StatusMethodNotAllowed)
				return
			}

			if requireAuth && s.config.API.Enabled {
				apiKey := r.Header.Get("X-API-Key")
				if apiKey == "" {
					apiKey = r.URL.Query().Get("key")
				}

				if !s.isValidAPIKey(apiKey) {
					s.sendError(w, "Unauthorized: Invalid or missing API key", StatusUnauthorized)
					return
				}
			}

			handler(w, r)
		}
	}

	mux.HandleFunc("/api/health", wrap(http.MethodGet, s.handleHealth, false))
	mux.HandleFunc("/api/artists", wrap(http.MethodGet, s.handleArtists, true))
	mux.HandleFunc("/api/artists/upload", wrap(http.MethodPost, s.handleArtistUpload, true))
	mux.HandleFunc("/api/artists/nuke", wrap(http.MethodDelete, s.handleArtistNuke, true))

	mux.HandleFunc("/api/tracks", wrap(http.MethodGet, s.handleTracks, true))
	mux.HandleFunc("/api/tracks/upload", wrap(http.MethodPost, s.handleTrackUpload, true))
	mux.HandleFunc("/api/tracks/nuke", wrap(http.MethodDelete, s.handleTrackNuke, true))

	fmt.Printf("%sAPI Server gestart op poort %s%s\n", Green, port, Reset)
	return http.ListenAndServe(":"+port, mux)
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendSuccess(w, map[string]string{
		"status":   "healthy",
		"version":  Version,
		"database": s.service.config.Database.Name,
	})
}

func (s *APIServer) handleArtists(w http.ResponseWriter, r *http.Request) {
	// Default: return statistics
	stats, err := s.service.GetStatistics(ScopeArtist)
	if err != nil {
		s.sendError(w, err.Error(), StatusInternalServerError)
		return
	}

	response := StatisticsResponse{
		Total:         stats.Total,
		WithImages:    stats.WithImages,
		WithoutImages: stats.WithoutImages,
		Orphaned:      stats.Orphaned,
	}

	s.sendSuccess(w, response)
}

func (s *APIServer) handleArtistUpload(w http.ResponseWriter, r *http.Request) {
	var req ImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", StatusBadRequest)
		return
	}

	params := &ImageUploadParams{
		Scope: ScopeArtist,
		Name:  req.Name,
		ID:    req.ID,
		URL:   req.URL,
	}

	if req.Image != "" {
		imageData, err := DecodeBase64Image(req.Image)
		if err != nil {
			s.sendError(w, "Invalid base64 image", StatusBadRequest)
			return
		}
		params.ImageData = imageData
	}

	result, err := s.service.UploadImage(params)
	if err != nil {
		if err.Error() == "moet naam of id specificeren" ||
			err.Error() == "kan niet zowel naam als id specificeren" {
			s.sendError(w, err.Error(), StatusBadRequest)
			return
		}
		s.sendError(w, err.Error(), StatusInternalServerError)
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"artist":          result.ItemName,
		"original_size":   result.OriginalSize,
		"optimized_size":  result.OptimizedSize,
		"savings_percent": result.SavingsPercent,
		"encoder":         result.Encoder,
	})
}

func (s *APIServer) handleTracks(w http.ResponseWriter, r *http.Request) {
	// Default: return statistics
	stats, err := s.service.GetStatistics(ScopeTrack)
	if err != nil {
		s.sendError(w, err.Error(), StatusInternalServerError)
		return
	}

	response := StatisticsResponse{
		Total:         stats.Total,
		WithImages:    stats.WithImages,
		WithoutImages: stats.WithoutImages,
		Orphaned:      stats.Orphaned,
	}

	s.sendSuccess(w, response)
}

func (s *APIServer) handleTrackUpload(w http.ResponseWriter, r *http.Request) {
	var req ImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", StatusBadRequest)
		return
	}

	params := &ImageUploadParams{
		Scope: ScopeTrack,
		Name:  req.Name,
		ID:    req.ID,
		URL:   req.URL,
	}

	if req.Image != "" {
		imageData, err := DecodeBase64Image(req.Image)
		if err != nil {
			s.sendError(w, "Invalid base64 image", StatusBadRequest)
			return
		}
		params.ImageData = imageData
	}

	result, err := s.service.UploadImage(params)
	if err != nil {
		if err.Error() == "moet naam of id specificeren" ||
			err.Error() == "kan niet zowel naam als id specificeren" {
			s.sendError(w, err.Error(), StatusBadRequest)
			return
		}
		s.sendError(w, err.Error(), StatusInternalServerError)
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"track":           result.ItemTitle,
		"artist":          result.ItemName,
		"original_size":   result.OriginalSize,
		"optimized_size":  result.OptimizedSize,
		"savings_percent": result.SavingsPercent,
		"encoder":         result.Encoder,
	})
}

func (s *APIServer) handleArtistNuke(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Confirm-Nuke") != "VERWIJDER ALLES" {
		s.sendError(w, "Missing confirmation header: X-Confirm-Nuke", StatusBadRequest)
		return
	}

	result, err := s.service.NukeImages(ScopeArtist)
	if err != nil {
		s.sendError(w, err.Error(), StatusInternalServerError)
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"deleted": result.Deleted,
		"message": fmt.Sprintf("%d artiest afbeeldingen verwijderd", result.Deleted),
	})
}

func (s *APIServer) handleTrackNuke(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Confirm-Nuke") != "VERWIJDER ALLES" {
		s.sendError(w, "Missing confirmation header: X-Confirm-Nuke", StatusBadRequest)
		return
	}

	result, err := s.service.NukeImages(ScopeTrack)
	if err != nil {
		s.sendError(w, err.Error(), StatusInternalServerError)
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"deleted": result.Deleted,
		"message": fmt.Sprintf("%d track afbeeldingen verwijderd", result.Deleted),
	})
}

func (s *APIServer) sendSuccess(w http.ResponseWriter, data interface{}) {
	response := APIResponse{
		Success: true,
		Data:    data,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *APIServer) sendError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *APIServer) isValidAPIKey(key string) bool {
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
