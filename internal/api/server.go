package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ideamans/lightfile6-insights-gateway/internal/cache"
	"github.com/ideamans/lightfile6-insights-gateway/internal/config"
	"github.com/ideamans/lightfile6-insights-gateway/internal/s3"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
)

// Server represents the HTTP server
type Server struct {
	echo         *echo.Echo
	port         int
	cacheManager *cache.Manager
	s3Client     *s3.Client
	config       *config.Config
}

// NewServer creates a new HTTP server
func NewServer(port int, cacheManager *cache.Manager, s3Client *s3.Client, cfg *config.Config) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(LoggerMiddleware())

	s := &Server{
		echo:         e,
		port:         port,
		cacheManager: cacheManager,
		s3Client:     s3Client,
		config:       cfg,
	}

	// Setup routes
	s.setupRoutes()

	return s
}

// setupRoutes configures the API routes
func (s *Server) setupRoutes() {
	// Health check
	s.echo.GET("/health", s.handleHealth)

	// Authenticated routes
	api := s.echo.Group("")
	api.Use(AuthMiddleware())

	api.PUT("/usage", s.handleUsage)
	api.PUT("/error", s.handleError)
	api.PUT("/specimen", s.handleSpecimen)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// handleUsage handles usage report uploads
func (s *Server) handleUsage(c echo.Context) error {
	user := c.Get("user").(string)
	
	// Read request body
	data, err := readRequestBody(c)
	if err != nil {
		log.Error().Err(err).Str("user", user).Msg("Failed to read usage request body")
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Save to cache
	if err := s.cacheManager.SaveUsage(user, data); err != nil {
		log.Error().Err(err).Str("user", user).Msg("Failed to save usage data")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save data")
	}

	log.Info().Str("user", user).Int("size", len(data)).Msg("Usage data saved")
	return c.NoContent(http.StatusNoContent)
}

// handleError handles error report uploads
func (s *Server) handleError(c echo.Context) error {
	user := c.Get("user").(string)
	
	// Read request body
	data, err := readRequestBody(c)
	if err != nil {
		log.Error().Err(err).Str("user", user).Msg("Failed to read error request body")
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Save to cache
	if err := s.cacheManager.SaveError(user, data); err != nil {
		log.Error().Err(err).Str("user", user).Msg("Failed to save error data")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save data")
	}

	log.Info().Str("user", user).Int("size", len(data)).Msg("Error data saved")
	return c.NoContent(http.StatusNoContent)
}

// handleSpecimen handles specimen file uploads
func (s *Server) handleSpecimen(c echo.Context) error {
	user := c.Get("user").(string)
	uri := c.QueryParam("uri")
	
	if uri == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "uri parameter is required")
	}

	// Read request body
	data, err := readRequestBody(c)
	if err != nil {
		log.Error().Err(err).Str("user", user).Str("uri", uri).Msg("Failed to read specimen request body")
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Save to cache for immediate upload
	if err := s.cacheManager.SaveSpecimen(uri, data); err != nil {
		log.Error().Err(err).Str("user", user).Str("uri", uri).Msg("Failed to save specimen data")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save data")
	}

	// Queue for immediate upload
	go s.uploadSpecimen(user, uri)

	log.Info().Str("user", user).Str("uri", uri).Int("size", len(data)).Msg("Specimen data saved")
	return c.NoContent(http.StatusNoContent)
}

// uploadSpecimen uploads a specimen file to S3
func (s *Server) uploadSpecimen(user, uri string) {
	if s.s3Client == nil {
		log.Warn().Str("user", user).Str("uri", uri).Msg("S3 client not configured, skipping upload")
		return
	}
	
	if err := s.s3Client.UploadSpecimen(user, uri); err != nil {
		log.Error().Err(err).Str("user", user).Str("uri", uri).Msg("Failed to upload specimen")
	} else {
		log.Info().Str("user", user).Str("uri", uri).Msg("Specimen uploaded successfully")
	}
}