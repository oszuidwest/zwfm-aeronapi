package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type APIServer struct {
	service *ImageService
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
	Image string `json:"image"` // base64 encoded image
}

type ItemResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	HasImage bool   `json:"has_image"`
}

func NewAPIServer(service *ImageService) *APIServer {
	return &APIServer{
		service: service,
	}
}

func (s *APIServer) Start(port string) error {
	mux := http.NewServeMux()

	// Middleware wrapper
	wrap := func(method string, handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Log request
			fmt.Printf("[%s] %s %s\n", r.Method, r.URL.Path, r.RemoteAddr)

			// Method validation
			if r.Method != method {
				s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			handler(w, r)
		}
	}

	// Routes
	mux.HandleFunc("/api/health", wrap(http.MethodGet, s.handleHealth))

	// Artist endpoints
	mux.HandleFunc("/api/artists", wrap(http.MethodGet, s.handleArtists))
	mux.HandleFunc("/api/artists/search", wrap(http.MethodGet, s.handleArtistSearch))
	mux.HandleFunc("/api/artists/upload", wrap(http.MethodPost, s.handleArtistImageUpload))
	mux.HandleFunc("/api/artists/nuke", wrap(http.MethodDelete, s.handleArtistNuke))

	// Track endpoints
	mux.HandleFunc("/api/tracks", wrap(http.MethodGet, s.handleTracks))
	mux.HandleFunc("/api/tracks/search", wrap(http.MethodGet, s.handleTrackSearch))
	mux.HandleFunc("/api/tracks/upload", wrap(http.MethodPost, s.handleTrackImageUpload))
	mux.HandleFunc("/api/tracks/nuke", wrap(http.MethodDelete, s.handleTrackNuke))

	fmt.Printf("%sAPI Server gestart op poort %s%s\n", Green, port, Reset)
	return http.ListenAndServe(":"+port, mux)
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {

	s.sendSuccess(w, map[string]string{
		"status": "healthy",
		"database": fmt.Sprintf("%s:%s/%s", s.service.config.Database.Host,
			s.service.config.Database.Port, s.service.config.Database.Name),
	})
}

func (s *APIServer) handleArtists(w http.ResponseWriter, r *http.Request) {

	withoutImages := r.URL.Query().Get("without_images") == "true"
	limit := 50

	var items interface{}
	var err error
	if withoutImages {
		items, err = s.service.ListWithoutImages(ScopeArtist, limit)
	} else {
		// For now, we'll use the same function but with true for hasImage
		// This would need a separate method in service if we want to list with images
		items, err = s.service.ListWithoutImages(ScopeArtist, limit)
	}

	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	artists := items.([]Artist)
	response := make([]ItemResponse, len(artists))
	for i, artist := range artists {
		response[i] = ItemResponse{
			ID:       artist.ID,
			Name:     artist.Name,
			HasImage: artist.HasImage,
		}
	}

	s.sendSuccess(w, response)
}

func (s *APIServer) handleArtistSearch(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query().Get("q")
	if query == "" {
		s.sendError(w, "Missing search query", http.StatusBadRequest)
		return
	}

	items, err := s.service.Search(ScopeArtist, query)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	artists := items.([]Artist)
	response := make([]ItemResponse, len(artists))
	for i, artist := range artists {
		response[i] = ItemResponse{
			ID:       artist.ID,
			Name:     artist.Name,
			HasImage: artist.HasImage,
		}
	}

	s.sendSuccess(w, response)
}

func (s *APIServer) handleArtistImageUpload(w http.ResponseWriter, r *http.Request) {

	var req ImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	params := &ImageUploadParams{
		Scope: ScopeArtist,
		Name:  req.Name,
		ID:    req.ID,
		URL:   req.URL,
	}

	// Decode base64 image if provided
	if req.Image != "" {
		imageData, err := DecodeBase64Image(req.Image)
		if err != nil {
			s.sendError(w, "Invalid base64 image", http.StatusBadRequest)
			return
		}
		params.ImageData = imageData
	}

	result, err := s.service.UploadImage(params)
	if err != nil {
		if err.Error() == "moet naam of id specificeren" ||
			err.Error() == "kan niet zowel naam als id specificeren" {
			s.sendError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.sendError(w, err.Error(), http.StatusInternalServerError)
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

	withoutImages := r.URL.Query().Get("without_images") == "true"
	limit := 50

	var items interface{}
	var err error
	if withoutImages {
		items, err = s.service.ListWithoutImages(ScopeTrack, limit)
	} else {
		items, err = s.service.ListWithoutImages(ScopeTrack, limit)
	}

	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tracks := items.([]Track)
	response := make([]ItemResponse, len(tracks))
	for i, track := range tracks {
		response[i] = ItemResponse{
			ID:       track.ID,
			Title:    track.Title,
			Artist:   track.Artist,
			HasImage: track.HasImage,
		}
	}

	s.sendSuccess(w, response)
}

func (s *APIServer) handleTrackSearch(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query().Get("q")
	if query == "" {
		s.sendError(w, "Missing search query", http.StatusBadRequest)
		return
	}

	items, err := s.service.Search(ScopeTrack, query)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tracks := items.([]Track)
	response := make([]ItemResponse, len(tracks))
	for i, track := range tracks {
		response[i] = ItemResponse{
			ID:       track.ID,
			Title:    track.Title,
			Artist:   track.Artist,
			HasImage: track.HasImage,
		}
	}

	s.sendSuccess(w, response)
}

func (s *APIServer) handleTrackImageUpload(w http.ResponseWriter, r *http.Request) {

	var req ImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	params := &ImageUploadParams{
		Scope: ScopeTrack,
		Name:  req.Name,
		ID:    req.ID,
		URL:   req.URL,
	}

	// Decode base64 image if provided
	if req.Image != "" {
		imageData, err := DecodeBase64Image(req.Image)
		if err != nil {
			s.sendError(w, "Invalid base64 image", http.StatusBadRequest)
			return
		}
		params.ImageData = imageData
	}

	result, err := s.service.UploadImage(params)
	if err != nil {
		if err.Error() == "moet naam of id specificeren" ||
			err.Error() == "kan niet zowel naam als id specificeren" {
			s.sendError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.sendError(w, err.Error(), http.StatusInternalServerError)
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

	// This is a dangerous operation, so we require a confirmation header
	if r.Header.Get("X-Confirm-Nuke") != "VERWIJDER ALLES" {
		s.sendError(w, "Missing confirmation header: X-Confirm-Nuke", http.StatusBadRequest)
		return
	}

	result, err := s.service.NukeImages(ScopeArtist)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"deleted": result.Deleted,
		"message": fmt.Sprintf("%d artiest afbeeldingen verwijderd", result.Deleted),
	})
}

func (s *APIServer) handleTrackNuke(w http.ResponseWriter, r *http.Request) {

	// This is a dangerous operation, so we require a confirmation header
	if r.Header.Get("X-Confirm-Nuke") != "VERWIJDER ALLES" {
		s.sendError(w, "Missing confirmation header: X-Confirm-Nuke", http.StatusBadRequest)
		return
	}

	result, err := s.service.NukeImages(ScopeTrack)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
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
	json.NewEncoder(w).Encode(response)
}

func (s *APIServer) sendError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	json.NewEncoder(w).Encode(response)
}
