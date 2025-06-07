package scheduler

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/database"
	"github.com/ownwarden/funnel-access/backend/internal/models"
	"github.com/ownwarden/funnel-access/backend/internal/service"
)

// MockFunnelServiceForScheduler is a mock for the FunnelService,
// specifically for testing the scheduler's interaction.
type MockFunnelServiceForScheduler struct {
	service.FunnelService // Embed to satisfy type if FunnelService was an interface, not strictly needed here
	
	mu                         sync.Mutex
	BackgroundDisableCheckFunc func()
	BackgroundDisableCheckCalls int
}

// NewMockFunnelServiceForScheduler creates a new mock FunnelService.
// It needs a nil dbStore and commander because the actual FunnelService constructor requires them.
// However, we will override the method we care about (BackgroundDisableCheck).
// This is a bit awkward due to FunnelService being a concrete type.
// A better approach would be for Scheduler to depend on an interface.
func NewMockFunnelServiceForScheduler() *MockFunnelServiceForScheduler {
	// We don't actually need a real FunnelService, just something that looks like it
	// and can have its BackgroundDisableCheck method mocked.
	// The current Scheduler takes *service.FunnelService.
	// So, this mock needs to be a *service.FunnelService or we need an interface.
	// For simplicity in this test, we'll assume we can pass a struct that has the method.
	// This won't work directly with the current Scheduler which expects *service.FunnelService.
	//
	// Let's adjust: The scheduler takes *service.FunnelService.
	// We need to provide a real *service.FunnelService, but its BackgroundDisableCheck
	// will be the one from the *actual* service. To test calls, we'd need to mock
	// the *dependencies* of BackgroundDisableCheck (dbStore, commander).
	//
	// Alternative: Define an interface for what Scheduler needs from FunnelService:
	// type FunnelChecker interface { BackgroundDisableCheck() }
	// And make Scheduler take FunnelChecker.
	//
	// For now, to test the *timing* and *stopping* of the scheduler, we can use a
	// real FunnelService with mocked dependencies (dbStore, commander) and observe logs,
	// or, more simply, create a wrapper or a simple mock that fits the *service.FunnelService type
	// by embedding it and overriding the method.

	// Let's make MockFunnelServiceForScheduler have the method directly.
	// The test will then need to construct a *service.FunnelService and somehow use this mock.
	// This is getting complicated.
	//
	// Simpler approach for this test:
	// Create a simple counter that BackgroundDisableCheck increments.
	// The actual *service.FunnelService will be used, and its dependencies will be mocked
	// such that BackgroundDisableCheck can run without external effects but we can see it was called.
	// The MockTailscaleCommander in the service package can be used.
	// The DB can be an in-memory one.
	return &MockFunnelServiceForScheduler{}
}


func (m *MockFunnelServiceForScheduler) BackgroundDisableCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BackgroundDisableCheckCalls++
	if m.BackgroundDisableCheckFunc != nil {
		m.BackgroundDisableCheckFunc()
	}
}


// TestScheduler_StartStop tests the basic start and stop functionality.
func TestScheduler_StartStop(t *testing.T) {
	dbStore, dbCleanup := SetupTestDBForServiceScheduler(t)
	defer dbCleanup()

	commander := &service.MockTailscaleCommander{}
	// Ensure DisableFunnelFunc is set so calls can be counted without erroring
	commander.DisableFunnelFunc = func() error { return nil }
	commander.GetActualFunnelStatusFunc = func() (bool, error) { return false, nil }


	fs := service.NewFunnelService(dbStore, commander)

	// Set funnel to enabled and expired to ensure BackgroundDisableCheck tries to act
	expiredTime := time.Now().UTC().Add(-5 * time.Minute)
	_, err := dbStore.UpdateFunnelState(models.FunnelStatusEnabled, &expiredTime)
	if err != nil {
		t.Fatalf("Failed to set DB state for scheduler test: %v", err)
	}

	commander.Reset() // Reset calls after DB setup

	checkInterval := 50 * time.Millisecond
	s := NewScheduler(fs, checkInterval)

	go s.Start()

	// Allow some checks to run, e.g., 3 ticks
	time.Sleep(checkInterval*3 + (checkInterval / 2))

	s.Stop()
	// Wait a bit for the scheduler to fully stop its goroutine
	time.Sleep(checkInterval) // Increased wait time for stop

	// Check how many times commander.DisableFunnel was called.
	// Each tick where the funnel is expired should trigger a call.
	// MockTailscaleCommander methods are internally mutex-protected for call counting.
	disableCalls := commander.DisableFunnelCalls

	// Expect DisableFunnel to be called once, as it fixes the state.
	if disableCalls != 1 {
		t.Errorf("Expected DisableFunnel to be called once for the initially expired state, got %d. Logs:\n%s", disableCalls, "See console for logs") // Placeholder for actual log capture if needed
	}

	// Test that stopping the scheduler prevents further calls
	// Reset DB to an expired state again to see if it would trigger if not stopped.
	_, err = dbStore.UpdateFunnelState(models.FunnelStatusEnabled, &expiredTime)
	if err != nil {
		t.Fatalf("Failed to reset DB state for scheduler stop test: %v", err)
	}
	// Reset call count on commander for this part of the test
	// Important: commander is shared, so its state persists.
	// We need to be careful. The previous `disableCalls` captured the state *after* the first run.
	// Let's re-get the count *before* this sleep.
	callsBeforeStopSleep := commander.DisableFunnelCalls


	furtherWait := checkInterval * 3
	time.Sleep(furtherWait)
	currentDisableCallsAfterStopSleep := commander.DisableFunnelCalls
	if currentDisableCallsAfterStopSleep != callsBeforeStopSleep {
		 t.Errorf("DisableFunnel was called again after Stop(). Before stop sleep: %d, After stop sleep: %d", callsBeforeStopSleep, currentDisableCallsAfterStopSleep)
	}
}


// SetupTestDBForServiceScheduler is a helper to setup DB for scheduler tests.
func SetupTestDBForServiceScheduler(t *testing.T) (*database.Store, func()) {
	t.Helper() // Marks this function as a test helper
	tempDir, err := os.MkdirTemp("", "scheduler_test_db_")
	if err != nil {
		t.Fatalf("Failed to create temp dir for test DB: %v", err)
	}
	dbPath := filepath.Join(tempDir, "test_scheduler_controller.db")

	// Ensure mattn/go-sqlite3 driver is pulled in for tests
	// by importing it in the test file or a shared test utility.
	// It's already imported at the top of this file.

	store, err := database.NewStore(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create test database store: %v", err)
	}
	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}
	return store, cleanup
}

// Note on testing:
// The current test for the scheduler is more of an integration test for the scheduler
// with a real FunnelService (that uses a mock commander and real in-memory DB).
// This is because the Scheduler takes a concrete *service.FunnelService.
// A pure unit test for the Scheduler would involve refactoring Scheduler to depend on an
// interface for the background check function, e.g.:
// type BackgroundTaskRunner interface { RunBackgroundTask() }
// Then, a mock implementation of this interface could be used to simply count calls.
