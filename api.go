package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type APIServer struct {
	db     *sql.DB
	config *Config
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

func NewAPIServer(db *sql.DB, config *Config) *APIServer {
	return &APIServer{
		db:     db,
		config: config,
	}
}

func (s *APIServer) Start(port string) error {
	mux := http.NewServeMux()

	// Middleware wrapper
	wrap := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			
			// Log request
			fmt.Printf("[%s] %s %s\n", r.Method, r.URL.Path, r.RemoteAddr)
			
			handler(w, r)
		}
	}

	// Routes
	mux.HandleFunc("/api/health", wrap(s.handleHealth))
	
	// Artist endpoints
	mux.HandleFunc("/api/artists", wrap(s.handleArtists))
	mux.HandleFunc("/api/artists/search", wrap(s.handleArtistSearch))
	mux.HandleFunc("/api/artists/upload", wrap(s.handleArtistImageUpload))
	mux.HandleFunc("/api/artists/nuke", wrap(s.handleArtistNuke))
	
	// Track endpoints
	mux.HandleFunc("/api/tracks", wrap(s.handleTracks))
	mux.HandleFunc("/api/tracks/search", wrap(s.handleTrackSearch))
	mux.HandleFunc("/api/tracks/upload", wrap(s.handleTrackImageUpload))
	mux.HandleFunc("/api/tracks/nuke", wrap(s.handleTrackNuke))

	fmt.Printf("%sAPI Server gestart op poort %s%s\n", Green, port, Reset)
	return http.ListenAndServe(":"+port, mux)
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.sendSuccess(w, map[string]string{
		"status": "healthy",
		"database": fmt.Sprintf("%s:%s/%s", s.config.Database.Host, s.config.Database.Port, s.config.Database.Name),
	})
}

func (s *APIServer) handleArtists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	withoutImages := r.URL.Query().Get("without_images") == "true"
	limit := 50
	
	artists, err := listArtists(s.db, s.config.Database.Schema, !withoutImages, limit)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		s.sendError(w, "Missing search query", http.StatusBadRequest)
		return
	}

	artists, err := findArtistsWithPartialName(s.db, s.config.Database.Schema, query)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	if r.Method != http.MethodPost {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Name == "" && req.ID == "" {
		s.sendError(w, "Either name or id must be provided", http.StatusBadRequest)
		return
	}
	if req.Name != "" && req.ID != "" {
		s.sendError(w, "Cannot specify both name and id", http.StatusBadRequest)
		return
	}
	if req.URL == "" && req.Image == "" {
		s.sendError(w, "Either url or image must be provided", http.StatusBadRequest)
		return
	}
	if req.URL != "" && req.Image != "" {
		s.sendError(w, "Cannot specify both url and image", http.StatusBadRequest)
		return
	}

	// Lookup artist
	artist, err := lookupArtist(s.db, s.config.Database.Schema, req.Name, req.ID)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusNotFound)
		return
	}

	// Load image data
	var imageData []byte
	if req.URL != "" {
		imageData, err = downloadImage(req.URL, s.config.Image.MaxFileSizeMB)
		if err != nil {
			s.sendError(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Decode base64 image
		imageData, err = decodeBase64Image(req.Image)
		if err != nil {
			s.sendError(w, "Invalid base64 image", http.StatusBadRequest)
			return
		}
	}

	// Process and optimize image
	result, err := processAndOptimizeImage(imageData, s.config.Image)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Save to database
	if err := updateArtistImageInDB(s.db, s.config.Database.Schema, artist.ID, result.Data); err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"artist": artist.Name,
		"original_size": result.Original.Size,
		"optimized_size": result.Optimized.Size,
		"savings_percent": result.Savings,
		"encoder": result.Encoder,
	})
}

func (s *APIServer) handleTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	withoutImages := r.URL.Query().Get("without_images") == "true"
	limit := 50
	
	tracks, err := listTracks(s.db, s.config.Database.Schema, !withoutImages, limit)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		s.sendError(w, "Missing search query", http.StatusBadRequest)
		return
	}

	tracks, err := findTracksWithPartialName(s.db, s.config.Database.Schema, query)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	if r.Method != http.MethodPost {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ImageUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Name == "" && req.ID == "" {
		s.sendError(w, "Either name or id must be provided", http.StatusBadRequest)
		return
	}
	if req.Name != "" && req.ID != "" {
		s.sendError(w, "Cannot specify both name and id", http.StatusBadRequest)
		return
	}
	if req.URL == "" && req.Image == "" {
		s.sendError(w, "Either url or image must be provided", http.StatusBadRequest)
		return
	}
	if req.URL != "" && req.Image != "" {
		s.sendError(w, "Cannot specify both url and image", http.StatusBadRequest)
		return
	}

	// Lookup track
	track, err := lookupTrack(s.db, s.config.Database.Schema, req.Name, req.ID)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusNotFound)
		return
	}

	// Load image data
	var imageData []byte
	if req.URL != "" {
		imageData, err = downloadImage(req.URL, s.config.Image.MaxFileSizeMB)
		if err != nil {
			s.sendError(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Decode base64 image
		imageData, err = decodeBase64Image(req.Image)
		if err != nil {
			s.sendError(w, "Invalid base64 image", http.StatusBadRequest)
			return
		}
	}

	// Process and optimize image
	result, err := processAndOptimizeImage(imageData, s.config.Image)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Save to database
	if err := saveTrackImageToDatabase(s.db, s.config.Database.Schema, track.ID, result.Data); err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.sendSuccess(w, map[string]interface{}{
		"track": track.Title,
		"artist": track.Artist,
		"original_size": result.Original.Size,
		"optimized_size": result.Optimized.Size,
		"savings_percent": result.Savings,
		"encoder": result.Encoder,
	})
}

func (s *APIServer) handleArtistNuke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// This is a dangerous operation, so we require a confirmation header
	if r.Header.Get("X-Confirm-Nuke") != "VERWIJDER ALLES" {
		s.sendError(w, "Missing confirmation header: X-Confirm-Nuke", http.StatusBadRequest)
		return
	}

	// Get count before deletion
	artists, err := listArtists(s.db, s.config.Database.Schema, true, 0)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	count := len(artists)

	if count == 0 {
		s.sendSuccess(w, map[string]interface{}{
			"deleted": 0,
			"message": "Geen artiesten met afbeeldingen gevonden",
		})
		return
	}

	// Delete all artist images
	query := fmt.Sprintf(`UPDATE %s.artist SET picture = NULL WHERE picture IS NOT NULL`, s.config.Database.Schema)
	result, err := s.db.Exec(query)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	s.sendSuccess(w, map[string]interface{}{
		"deleted": rowsAffected,
		"message": fmt.Sprintf("%d artiest afbeeldingen verwijderd", rowsAffected),
	})
}

func (s *APIServer) handleTrackNuke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// This is a dangerous operation, so we require a confirmation header
	if r.Header.Get("X-Confirm-Nuke") != "VERWIJDER ALLES" {
		s.sendError(w, "Missing confirmation header: X-Confirm-Nuke", http.StatusBadRequest)
		return
	}

	// Get count before deletion
	tracks, err := listTracks(s.db, s.config.Database.Schema, true, 0)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	count := len(tracks)

	if count == 0 {
		s.sendSuccess(w, map[string]interface{}{
			"deleted": 0,
			"message": "Geen tracks met afbeeldingen gevonden",
		})
		return
	}

	// Delete all track images
	query := fmt.Sprintf(`UPDATE %s.track SET picture = NULL WHERE picture IS NOT NULL`, s.config.Database.Schema)
	result, err := s.db.Exec(query)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	s.sendSuccess(w, map[string]interface{}{
		"deleted": rowsAffected,
		"message": fmt.Sprintf("%d track afbeeldingen verwijderd", rowsAffected),
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

func decodeBase64Image(data string) ([]byte, error) {
	// Remove data URL prefix if present
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}
	
	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))
}