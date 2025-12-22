// Package api provides the HTTP API server for the Aeron radio automation system.
package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/oszuidwest/zwfm-aeronapi/internal/config"
	"github.com/oszuidwest/zwfm-aeronapi/internal/service"
	"github.com/oszuidwest/zwfm-aeronapi/internal/types"
)

// Version is the application version, set at build time.
var Version = "dev"

// Server represents the HTTP API server for the Aeron radio automation system.
// It provides REST endpoints for managing artist and track images in the Aeron database.
// The API supports image upload, retrieval, deletion, and statistics operations.
type Server struct {
	service *service.AeronService
	config  *config.Config
	server  *http.Server
}

// New creates a new Server instance with the provided service and configuration.
// The service handles business logic while config contains API authentication settings.
func New(svc *service.AeronService, cfg *config.Config) *Server {
	return &Server{
		service: svc,
		config:  cfg,
	}
}

// Start initializes and starts the HTTP server on the specified port.
// It configures middleware, routes, and begins listening for incoming requests.
// Returns an error if the server fails to start.
func (s *Server) Start(port string) error {
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
			s.setupEntityRoutes(r, "/artists", types.ScopeArtist)
			s.setupEntityRoutes(r, "/tracks", types.ScopeTrack)

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
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// setupEntityRoutes configures routes for an entity type (artist/track)
func (s *Server) setupEntityRoutes(r chi.Router, path string, scope types.Scope) {
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
func (s *Server) authMiddleware(next http.Handler) http.Handler {
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

func (s *Server) isValidAPIKey(key string) bool {
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
