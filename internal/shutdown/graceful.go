package shutdown

import (
	"sync"

	"github.com/rs/zerolog/log"
)

// GracefulShutdown manages graceful shutdown
type GracefulShutdown struct {
	mu     sync.Mutex
	done   bool
	doneCh chan struct{}
}

// NewGracefulShutdown creates a new graceful shutdown manager
func NewGracefulShutdown() *GracefulShutdown {
	return &GracefulShutdown{
		doneCh: make(chan struct{}),
	}
}

// Shutdown performs graceful shutdown
func (gs *GracefulShutdown) Shutdown(cleanup func() error) {
	gs.mu.Lock()
	if gs.done {
		gs.mu.Unlock()
		return
	}
	gs.done = true
	gs.mu.Unlock()
	
	// Perform cleanup
	if cleanup != nil {
		if err := cleanup(); err != nil {
			log.Error().Err(err).Msg("Error during cleanup")
		}
	}
	
	// Signal completion
	close(gs.doneCh)
}

// Done returns a channel that's closed when shutdown is complete
func (gs *GracefulShutdown) Done() <-chan struct{} {
	return gs.doneCh
}