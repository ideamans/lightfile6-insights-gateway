package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ideamans/lightfile6-insights-gateway/internal/api"
	"github.com/ideamans/lightfile6-insights-gateway/internal/cache"
	"github.com/ideamans/lightfile6-insights-gateway/internal/config"
	"github.com/ideamans/lightfile6-insights-gateway/internal/s3"
	"github.com/ideamans/lightfile6-insights-gateway/internal/shutdown"
	"github.com/ideamans/lightfile6-insights-gateway/internal/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Initialize logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Parse command line arguments
	var port int
	var configPath string
	flag.IntVar(&port, "p", 0, "Port number (required)")
	flag.StringVar(&configPath, "c", "/etc/lightfile6/config.yml", "Config file path")
	flag.Parse()

	if port == 0 {
		log.Fatal().Msg("Port number is required (-p)")
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize cache manager
	cacheManager := cache.NewManager(cfg.CacheDir)
	if err := cacheManager.Init(); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize cache manager")
	}

	// Initialize S3 client
	s3Client, err := s3.NewClient(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create S3 client")
	}

	// Initialize worker
	workerManager := worker.NewManager(cacheManager, s3Client, cfg)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	workerManager.Start(ctx)

	// Initialize and start HTTP server
	server := api.NewServer(port, cacheManager, s3Client, cfg)
	
	// Setup graceful shutdown
	graceful := shutdown.NewGracefulShutdown()
	
	// Start HTTP server
	serverErr := make(chan error, 1)
	go func() {
		log.Info().Int("port", port).Msg("Starting HTTP server")
		serverErr <- server.Start()
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Info().Msg("Received shutdown signal")
	case err := <-serverErr:
		log.Error().Err(err).Msg("Server error")
	}

	// Start graceful shutdown
	graceful.Shutdown(func() error {
		log.Info().Msg("Starting graceful shutdown")
		
		// Stop accepting new requests
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Error during server shutdown")
		}
		
		// Cancel worker context
		cancel()
		
		// Wait for workers to finish
		workerManager.Wait()
		
		// Process remaining files
		log.Info().Msg("Processing remaining files")
		if err := workerManager.ProcessRemaining(); err != nil {
			log.Error().Err(err).Msg("Error processing remaining files")
		}
		
		log.Info().Msg("Graceful shutdown completed")
		return nil
	})
}