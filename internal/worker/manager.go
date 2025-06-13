package worker

import (
	"context"
	"sync"
	"time"

	"github.com/ideamans/lightfile6-insights-gateway/internal/cache"
	"github.com/ideamans/lightfile6-insights-gateway/internal/config"
	"github.com/ideamans/lightfile6-insights-gateway/internal/s3"
	"github.com/rs/zerolog/log"
)

// Manager manages background workers
type Manager struct {
	cacheManager *cache.Manager
	s3Client     *s3.Client
	aggregator   *s3.Aggregator
	config       *config.Config
	wg           sync.WaitGroup
	usageTicker  *time.Ticker
	errorTicker  *time.Ticker
}

// NewManager creates a new worker manager
func NewManager(cacheManager *cache.Manager, s3Client *s3.Client, cfg *config.Config) *Manager {
	// Set cache manager in S3 client
	s3Client.SetCacheManager(cacheManager)
	
	// Create aggregator
	aggregator := s3.NewAggregator(cacheManager, s3Client)
	
	return &Manager{
		cacheManager: cacheManager,
		s3Client:     s3Client,
		aggregator:   aggregator,
		config:       cfg,
	}
}

// Start starts all background workers
func (m *Manager) Start(ctx context.Context) {
	// Start usage aggregation worker
	m.usageTicker = time.NewTicker(m.config.Aggregation.UsageInterval)
	m.wg.Add(1)
	go m.runAggregationWorker(ctx, "usage", m.usageTicker.C)
	
	// Start error aggregation worker
	m.errorTicker = time.NewTicker(m.config.Aggregation.ErrorInterval)
	m.wg.Add(1)
	go m.runAggregationWorker(ctx, "error", m.errorTicker.C)
	
	log.Info().
		Dur("usageInterval", m.config.Aggregation.UsageInterval).
		Dur("errorInterval", m.config.Aggregation.ErrorInterval).
		Msg("Started background workers")
}

// Wait waits for all workers to finish
func (m *Manager) Wait() {
	// Stop tickers
	if m.usageTicker != nil {
		m.usageTicker.Stop()
	}
	if m.errorTicker != nil {
		m.errorTicker.Stop()
	}
	
	// Wait for workers
	m.wg.Wait()
}

// ProcessRemaining processes any remaining files
func (m *Manager) ProcessRemaining() error {
	return m.aggregator.ProcessRemaining()
}

// runAggregationWorker runs the aggregation worker for a specific data type
func (m *Manager) runAggregationWorker(ctx context.Context, dataType string, ticker <-chan time.Time) {
	defer m.wg.Done()
	
	log.Info().Str("dataType", dataType).Msg("Aggregation worker started")
	
	for {
		select {
		case <-ctx.Done():
			log.Info().Str("dataType", dataType).Msg("Aggregation worker stopping")
			return
		case <-ticker:
			if err := m.aggregator.AggregateAndUpload(dataType); err != nil {
				log.Error().
					Err(err).
					Str("dataType", dataType).
					Msg("Aggregation failed")
			}
		}
	}
}