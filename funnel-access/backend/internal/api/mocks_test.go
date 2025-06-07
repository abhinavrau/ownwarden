package api

import (
	"sync"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/models"
)

// MockFunnelService is a mock implementation of the parts of FunnelService used by handlers.
type MockFunnelService struct {
	mu sync.Mutex

	EnableFunnelFunc         func(durationMinutes int) (*models.FunnelState, error)
	DisableFunnelFunc        func() (*models.FunnelState, error)
	ExtendFunnelDurationFunc func(additionalMinutes int) (*models.FunnelState, error)
	GetFunnelStatusFunc      func() (*models.FunnelState, error)

	EnableFunnelCalls         int
	DisableFunnelCalls        int
	ExtendFunnelDurationCalls int
	GetFunnelStatusCalls      int
}

func (m *MockFunnelService) EnableFunnel(durationMinutes int) (*models.FunnelState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EnableFunnelCalls++
	if m.EnableFunnelFunc != nil {
		return m.EnableFunnelFunc(durationMinutes)
	}
	// Default success, returns an enabled state
	return &models.FunnelState{
		Status:             models.FunnelStatusEnabled,
		DisableAtTimestamp: func() *time.Time { t := time.Now().Add(time.Duration(durationMinutes) * time.Minute); return &t }(),
		LastUpdatedTimestamp: time.Now().UTC(),
	}, nil
}

func (m *MockFunnelService) DisableFunnel() (*models.FunnelState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DisableFunnelCalls++
	if m.DisableFunnelFunc != nil {
		return m.DisableFunnelFunc()
	}
	return &models.FunnelState{Status: models.FunnelStatusDisabled, LastUpdatedTimestamp: time.Now().UTC()}, nil
}

func (m *MockFunnelService) ExtendFunnelDuration(additionalMinutes int) (*models.FunnelState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ExtendFunnelDurationCalls++
	if m.ExtendFunnelDurationFunc != nil {
		return m.ExtendFunnelDurationFunc(additionalMinutes)
	}
	return &models.FunnelState{
		Status:             models.FunnelStatusEnabled,
		DisableAtTimestamp: func() *time.Time { t := time.Now().Add(time.Duration(additionalMinutes) * time.Minute); return &t }(),
		LastUpdatedTimestamp: time.Now().UTC(),
	}, nil
}

func (m *MockFunnelService) GetFunnelStatus() (*models.FunnelState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetFunnelStatusCalls++
	if m.GetFunnelStatusFunc != nil {
		return m.GetFunnelStatusFunc()
	}
	return &models.FunnelState{Status: models.FunnelStatusDisabled, LastUpdatedTimestamp: time.Now().UTC()}, nil
}

func (m *MockFunnelService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EnableFunnelCalls = 0
	m.DisableFunnelCalls = 0
	m.ExtendFunnelDurationCalls = 0
	m.GetFunnelStatusCalls = 0
	m.EnableFunnelFunc = nil
	m.DisableFunnelFunc = nil
	m.ExtendFunnelDurationFunc = nil
	m.GetFunnelStatusFunc = nil
}

// This mock is designed to be used with APIHandler, which expects a *service.FunnelService.
// To use this mock, the APIHandler's funnelService field would need to accept an interface
// that both *service.FunnelService and *MockFunnelService implement.
//
// For example:
// type FunnelServiceProvider interface {
//     EnableFunnel(durationMinutes int) (*models.FunnelState, error)
//     DisableFunnel() (*models.FunnelState, error)
//     ExtendFunnelDuration(additionalMinutes int) (*models.FunnelState, error)
//     GetFunnelStatus() (*models.FunnelState, error)
// }
// Then APIHandler would take FunnelServiceProvider.
//
// Since APIHandler currently takes a concrete *service.FunnelService, testing handlers
// in complete isolation from the actual service logic (by using this kind of mock)
// requires refactoring APIHandler or using a more complex test setup.
//
// An alternative for handler tests is to use the *real* service.FunnelService,
// initialized with an in-memory DB and a MockTailscaleCommander. This tests the
// handler's interaction with the actual service. This is the approach I will take for handlers_test.go
// to avoid refactoring service.FunnelService or APIHandler to use interfaces at this stage.
// Therefore, this MockFunnelService might not be directly used if we test handlers integrated with the real service.
// I'll keep it for now as it's a common pattern, but the actual handler tests will likely construct a real FunnelService.
