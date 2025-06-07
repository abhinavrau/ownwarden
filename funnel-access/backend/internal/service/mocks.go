package service

import (
	"sync"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/models"
)

// MockTailscaleCommander is a mock implementation of TailscaleCommander for testing.
type MockTailscaleCommander struct {
	mu                sync.Mutex
	EnableFunnelFunc  func() error
	DisableFunnelFunc func() error
	GetActualFunnelStatusFunc func() (bool, error)

	EnableFunnelCalls  int
	DisableFunnelCalls int
	GetActualFunnelStatusCalls int
}

func (m *MockTailscaleCommander) EnableFunnel() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EnableFunnelCalls++
	if m.EnableFunnelFunc != nil {
		return m.EnableFunnelFunc()
	}
	return nil // Default success
}

func (m *MockTailscaleCommander) DisableFunnel() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DisableFunnelCalls++
	if m.DisableFunnelFunc != nil {
		return m.DisableFunnelFunc()
	}
	return nil // Default success
}

func (m *MockTailscaleCommander) GetActualFunnelStatus() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetActualFunnelStatusCalls++
	if m.GetActualFunnelStatusFunc != nil {
		return m.GetActualFunnelStatusFunc()
	}
	return false, nil // Default: disabled, no error
}

func (m *MockTailscaleCommander) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EnableFunnelCalls = 0
	m.DisableFunnelCalls = 0
	m.GetActualFunnelStatusCalls = 0
	m.EnableFunnelFunc = nil
	m.DisableFunnelFunc = nil
	m.GetActualFunnelStatusFunc = nil
}


// --- Mock Database Store (Illustrative - for more complex scenarios) ---
// For FunnelService tests, we'll use a real in-memory SQLite for simplicity,
// but a mock store would look something like this if we wanted to avoid DB interaction entirely.

type MockDatabaseStore struct {
	mu sync.Mutex
	GetFunnelStateFunc func() (*models.FunnelState, error)
	UpdateFunnelStateFunc func(status models.FunnelStatus, disableAt *time.Time) (*models.FunnelState, error)

	GetFunnelStateCalls int
	UpdateFunnelStateCalls int
	LastStatusUpdated models.FunnelStatus
	LastDisableAtUpdated *time.Time
}

func (m *MockDatabaseStore) GetFunnelState() (*models.FunnelState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetFunnelStateCalls++
	if m.GetFunnelStateFunc != nil {
		return m.GetFunnelStateFunc()
	}
	// Default: return a disabled state
	return &models.FunnelState{Status: models.FunnelStatusDisabled, LastUpdatedTimestamp: time.Now().UTC()}, nil
}

func (m *MockDatabaseStore) UpdateFunnelState(status models.FunnelStatus, disableAt *time.Time) (*models.FunnelState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdateFunnelStateCalls++
	m.LastStatusUpdated = status
	m.LastDisableAtUpdated = disableAt
	if m.UpdateFunnelStateFunc != nil {
		return m.UpdateFunnelStateFunc(status, disableAt)
	}
	// Default: return the updated state
	return &models.FunnelState{Status: status, DisableAtTimestamp: disableAt, LastUpdatedTimestamp: time.Now().UTC()}, nil
}

func (m *MockDatabaseStore) Close() error { return nil } // Mock close

func (m *MockDatabaseStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetFunnelStateCalls = 0
	m.UpdateFunnelStateCalls = 0
	m.GetFunnelStateFunc = nil
	m.UpdateFunnelStateFunc = nil
	m.LastStatusUpdated = "" // Reset to zero value
	m.LastDisableAtUpdated = nil
}

// Note: The MockDatabaseStore is not directly used by funnel_service_test.go as it uses
// a real in-memory SQLite. This is here for illustrative purposes or if we decide to
// fully mock out the DB layer later. The `database.Store` itself would need to be
// based on an interface for this mock to be substitutable directly.
// For now, `funnel_service_test.go` will initialize a real `database.Store` with an
// in-memory SQLite connection string.
