package scheduler

import (
	"log"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/service"
)

// Scheduler manages periodic tasks.
type Scheduler struct {
	funnelService *service.FunnelService
	ticker        *time.Ticker
	quit          chan struct{}
}

// NewScheduler creates a new Scheduler.
// checkInterval specifies how often the background check should run.
func NewScheduler(fs *service.FunnelService, checkInterval time.Duration) *Scheduler {
	if checkInterval <= 0 {
		checkInterval = 1 * time.Minute // Default interval if invalid
		log.Printf("Scheduler: Invalid checkInterval, defaulting to %v", checkInterval)
	}
	return &Scheduler{
		funnelService: fs,
		ticker:        time.NewTicker(checkInterval),
		quit:          make(chan struct{}),
	}
}

// Start begins the scheduler's periodic checks.
// This should be run in a goroutine.
func (s *Scheduler) Start() {
	log.Printf("Scheduler started with check interval: %v", s.ticker.C)
	defer log.Println("Scheduler stopped.")
	for {
		select {
		case <-s.ticker.C:
			log.Println("Scheduler: Triggering background funnel check...")
			s.funnelService.BackgroundDisableCheck()
		case <-s.quit:
			s.ticker.Stop()
			return
		}
	}
}

// Stop terminates the scheduler.
func (s *Scheduler) Stop() {
	log.Println("Scheduler: Stopping...")
	close(s.quit)
}
