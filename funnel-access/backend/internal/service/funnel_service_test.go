package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/database"
	"github.com/ownwarden/funnel-access/backend/internal/models"
	// Ensure SQLite driver is included for tests using real DB
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB initializes an in-memory SQLite database for testing.
// It returns the store and a cleanup function.
func setupTestDB(t *testing.T) (*database.Store, func()) {
	// Create a temporary directory for the test database file
	tempDir, err := os.MkdirTemp("", "funnel_test_db_")
	if err != nil {
		t.Fatalf("Failed to create temp dir for test DB: %v", err)
	}
	dbPath := filepath.Join(tempDir, "test_funnel_controller.db")

	// Using a file-based SQLite for tests can sometimes be more representative
	// than pure in-memory ":memory:", especially if file locking or specific pragmas are involved.
	// However, ":memory:" is often faster. For this service, file-based is fine.
	// To use pure in-memory: store, err := database.NewStore(":memory:")
	store, err := database.NewStore(dbPath)
	if err != nil {
		os.RemoveAll(tempDir) // Attempt to clean up temp dir if store creation fails
		t.Fatalf("Failed to create test database store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir) // Clean up the temp directory and DB file
	}
	return store, cleanup
}


func TestFunnelService_EnableFunnel(t *testing.T) {
	mockCmdr := &MockTailscaleCommander{}

	t.Run("successfully enable funnel when disabled", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return false, nil } // Simulate funnel is actually off

		duration := 30
		state, err := service.EnableFunnel(duration)

		if err != nil {
			t.Errorf("EnableFunnel() error = %v, wantErr nil", err)
		}
		if state == nil {
			t.Fatal("EnableFunnel() state is nil, want non-nil")
		}
		if state.Status != models.FunnelStatusEnabled {
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusEnabled)
		}
		if mockCmdr.EnableFunnelCalls != 1 {
			t.Errorf("mockCmdr.EnableFunnelCalls = %d, want 1", mockCmdr.EnableFunnelCalls)
		}
		if mockCmdr.GetActualFunnelStatusCalls != 1 {
			t.Errorf("mockCmdr.GetActualFunnelStatusCalls = %d, want 1", mockCmdr.GetActualFunnelStatusCalls)
		}
		// Check timestamp roughly
		expectedDisableAt := time.Now().Add(time.Duration(duration) * time.Minute)
		if state.DisableAtTimestamp == nil || state.DisableAtTimestamp.Before(expectedDisableAt.Add(-1*time.Minute)) || state.DisableAtTimestamp.After(expectedDisableAt.Add(1*time.Minute)) {
			t.Errorf("state.DisableAtTimestamp = %v, want around %v", state.DisableAtTimestamp, expectedDisableAt)
		}
	})

	t.Run("funnel already enabled in DB and actual, extend duration", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		// Initial state: enabled for 10 mins
		initialDisableAt := time.Now().UTC().Add(10 * time.Minute)
		_, err := db.UpdateFunnelState(models.FunnelStatusEnabled, &initialDisableAt)
		if err != nil {
			t.Fatalf("Failed to set initial DB state: %v", err)
		}
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return true, nil } // Simulate funnel is actually on

		duration := 30 // Request to enable for 30 mins (longer)
		state, err := service.EnableFunnel(duration)

		if err != nil {
			t.Errorf("EnableFunnel() error = %v, wantErr nil", err)
		}
		if state.Status != models.FunnelStatusEnabled {
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusEnabled)
		}
		// EnableFunnel should not call commander.EnableFunnel if already active and only extending
		if mockCmdr.EnableFunnelCalls != 0 {
			t.Errorf("mockCmdr.EnableFunnelCalls = %d, want 0", mockCmdr.EnableFunnelCalls)
		}
		if mockCmdr.GetActualFunnelStatusCalls != 1 {
			t.Errorf("mockCmdr.GetActualFunnelStatusCalls = %d, want 1", mockCmdr.GetActualFunnelStatusCalls)
		}
		expectedDisableAt := time.Now().UTC().Add(time.Duration(duration) * time.Minute)
		if state.DisableAtTimestamp == nil || state.DisableAtTimestamp.Before(expectedDisableAt.Add(-1*time.Minute)) || state.DisableAtTimestamp.After(expectedDisableAt.Add(1*time.Minute)) {
			t.Errorf("state.DisableAtTimestamp = %v, want around %v", state.DisableAtTimestamp, expectedDisableAt)
		}
	})

	t.Run("funnel already enabled in DB, new duration not longer", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		initialDisableAt := time.Now().UTC().Add(60 * time.Minute)
		_, err := db.UpdateFunnelState(models.FunnelStatusEnabled, &initialDisableAt)
		if err != nil {
			t.Fatalf("Failed to set initial DB state: %v", err)
		}
		// Actual status check might or might not be called depending on logic path, let's assume it is for now.
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return true, nil }


		duration := 30 // Request to enable for 30 mins (shorter)
		state, err := service.EnableFunnel(duration)

		if !errors.Is(err, ErrFunnelAlreadyEnabled) {
			t.Errorf("EnableFunnel() error = %v, want %v", err, ErrFunnelAlreadyEnabled)
		}
		if state.Status != models.FunnelStatusEnabled { // Should return current state
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusEnabled)
		}
		if state.DisableAtTimestamp == nil || !state.DisableAtTimestamp.Equal(initialDisableAt) {
			t.Errorf("state.DisableAtTimestamp = %v, want %v", state.DisableAtTimestamp, initialDisableAt)
		}
		if mockCmdr.EnableFunnelCalls != 0 {
			t.Errorf("mockCmdr.EnableFunnelCalls = %d, want 0", mockCmdr.EnableFunnelCalls)
		}
	})
	
	t.Run("DB says disabled, but actual funnel is ON (inconsistent state)", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return true, nil } // Actual is ON

		// DB is default: DISABLED
		duration := 30
		state, err := service.EnableFunnel(duration)

		if err != nil {
			t.Errorf("EnableFunnel() error = %v, wantErr nil", err)
		}
		if state.Status != models.FunnelStatusEnabled {
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusEnabled)
		}
		// EnableFunnel should not call commander.EnableFunnel if actual is already on
		if mockCmdr.EnableFunnelCalls != 0 {
			t.Errorf("mockCmdr.EnableFunnelCalls = %d, want 0", mockCmdr.EnableFunnelCalls)
		}
		if mockCmdr.GetActualFunnelStatusCalls != 1 {
			t.Errorf("mockCmdr.GetActualFunnelStatusCalls = %d, want 1", mockCmdr.GetActualFunnelStatusCalls)
		}
		dbState, _ := db.GetFunnelState()
		if dbState.Status != models.FunnelStatusEnabled {
			t.Errorf("DB state.Status = %s, want %s (should be updated to sync)", dbState.Status, models.FunnelStatusEnabled)
		}
	})


	t.Run("enable command fails", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return false, nil }
		mockCmdr.EnableFunnelFunc = func() error { return errors.New("command failed") }

		_, err := service.EnableFunnel(30)
		if !errors.Is(err, ErrFunnelEnableFailed) {
			t.Errorf("EnableFunnel() error = %v, want %v", err, ErrFunnelEnableFailed)
		}
		if mockCmdr.EnableFunnelCalls != 1 {
			t.Errorf("mockCmdr.EnableFunnelCalls = %d, want 1", mockCmdr.EnableFunnelCalls)
		}
		dbState, _ := db.GetFunnelState()
		if dbState.Status != models.FunnelStatusDisabled { // Should remain disabled
			t.Errorf("DB state.Status = %s, want %s", dbState.Status, models.FunnelStatusDisabled)
		}
	})
}

func TestFunnelService_DisableFunnel(t *testing.T) {
	mockCmdr := &MockTailscaleCommander{}

	t.Run("successfully disable funnel when enabled", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		// Initial state: enabled
		initialDisableAt := time.Now().UTC().Add(30 * time.Minute)
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &initialDisableAt)
		mockCmdr.DisableFunnelFunc = func() error { return nil } // Success

		state, err := service.DisableFunnel()

		if err != nil {
			t.Errorf("DisableFunnel() error = %v, wantErr nil", err)
		}
		if state.Status != models.FunnelStatusDisabled {
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusDisabled)
		}
		if state.DisableAtTimestamp != nil {
			t.Errorf("state.DisableAtTimestamp = %v, want nil", state.DisableAtTimestamp)
		}
		if mockCmdr.DisableFunnelCalls != 1 {
			t.Errorf("mockCmdr.DisableFunnelCalls = %d, want 1", mockCmdr.DisableFunnelCalls)
		}
	})

	t.Run("disable command fails but funnel already disabled", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		// DB state: enabled (to trigger disable logic)
		initialDisableAt := time.Now().UTC().Add(30 * time.Minute)
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &initialDisableAt)

		mockCmdr.DisableFunnelFunc = func() error { return errors.New("command failed") }
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return false, nil } // Actually disabled

		state, err := service.DisableFunnel()
		if err != nil {
			t.Errorf("DisableFunnel() error = %v, wantErr nil (because actual status is disabled)", err)
		}
		if state.Status != models.FunnelStatusDisabled {
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusDisabled)
		}
		if mockCmdr.DisableFunnelCalls != 1 {
			t.Errorf("mockCmdr.DisableFunnelCalls = %d, want 1", mockCmdr.DisableFunnelCalls)
		}
		if mockCmdr.GetActualFunnelStatusCalls != 1 { // Called after primary command fails
			t.Errorf("mockCmdr.GetActualFunnelStatusCalls = %d, want 1", mockCmdr.GetActualFunnelStatusCalls)
		}
	})
	
	t.Run("disable command fails and funnel still active", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		initialDisableAt := time.Now().UTC().Add(30 * time.Minute)
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &initialDisableAt)

		mockCmdr.DisableFunnelFunc = func() error { return errors.New("command failed") }
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return true, nil } // Actually still enabled

		_, err := service.DisableFunnel()
		if !errors.Is(err, ErrFunnelDisableFailed) {
			t.Errorf("DisableFunnel() error = %v, want %v", err, ErrFunnelDisableFailed)
		}
		dbState, _ := db.GetFunnelState()
		if dbState.Status != models.FunnelStatusEnabled { // Should remain enabled in DB
			t.Errorf("DB state.Status = %s, want %s", dbState.Status, models.FunnelStatusEnabled)
		}
	})
}

func TestFunnelService_ExtendFunnelDuration(t *testing.T) {
	mockCmdr := &MockTailscaleCommander{} // Not directly used by extend, but part of service

	t.Run("successfully extend duration", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)

		baseTime := time.Now().UTC().Add(10 * time.Minute) // Expires in 10 mins
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &baseTime)

		additionalMinutes := 15
		state, err := service.ExtendFunnelDuration(additionalMinutes)

		if err != nil {
			t.Errorf("ExtendFunnelDuration() error = %v, wantErr nil", err)
		}
		if state.Status != models.FunnelStatusEnabled {
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusEnabled)
		}
		expectedDisableAt := baseTime.Add(time.Duration(additionalMinutes) * time.Minute)
		if state.DisableAtTimestamp == nil || !state.DisableAtTimestamp.Equal(expectedDisableAt) {
			t.Errorf("state.DisableAtTimestamp = %v, want %v", state.DisableAtTimestamp, expectedDisableAt)
		}
	})

	t.Run("extend when funnel not enabled", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		// DB is default: DISABLED

		_, err := service.ExtendFunnelDuration(15)
		if !errors.Is(err, ErrFunnelNotEnabled) {
			t.Errorf("ExtendFunnelDuration() error = %v, want %v", err, ErrFunnelNotEnabled)
		}
	})

	t.Run("extend when funnel enabled but expired", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)

		expiredTime := time.Now().UTC().Add(-10 * time.Minute) // Expired 10 mins ago
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &expiredTime)

		_, err := service.ExtendFunnelDuration(15)
		if !errors.Is(err, ErrFunnelNotEnabled) {
			t.Errorf("ExtendFunnelDuration() error = %v, want %v", err, ErrFunnelNotEnabled)
		}
	})
}

func TestFunnelService_GetFunnelStatus(t *testing.T) {
	mockCmdr := &MockTailscaleCommander{}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	service := NewFunnelService(db, mockCmdr)

	t.Run("get status when disabled", func(t *testing.T) {
		// DB is default: DISABLED
		state, err := service.GetFunnelStatus()
		if err != nil {
			t.Fatalf("GetFunnelStatus() error = %v", err)
		}
		if state.Status != models.FunnelStatusDisabled {
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusDisabled)
		}
		if state.DisableAtTimestamp != nil {
			t.Errorf("state.DisableAtTimestamp = %v, want nil", state.DisableAtTimestamp)
		}
	})

	t.Run("get status when enabled", func(t *testing.T) {
		expectedDisableAt := time.Now().UTC().Add(30 * time.Minute)
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &expectedDisableAt)
		defer func() { _, _ = db.UpdateFunnelState(models.FunnelStatusDisabled, nil) }() // Cleanup

		state, err := service.GetFunnelStatus()
		if err != nil {
			t.Fatalf("GetFunnelStatus() error = %v", err)
		}
		if state.Status != models.FunnelStatusEnabled {
			t.Errorf("state.Status = %s, want %s", state.Status, models.FunnelStatusEnabled)
		}
		if state.DisableAtTimestamp == nil || !state.DisableAtTimestamp.Equal(expectedDisableAt) {
			// Compare with tolerance due to potential slight differences in time.Now() if not careful
			if state.DisableAtTimestamp == nil || state.DisableAtTimestamp.Sub(expectedDisableAt).Abs() > time.Second {
				t.Errorf("state.DisableAtTimestamp = %v, want %v (within 1 sec)", state.DisableAtTimestamp, expectedDisableAt)
			}
		}
	})
}

func TestFunnelService_BackgroundDisableCheck(t *testing.T) {
	mockCmdr := &MockTailscaleCommander{}
	
	t.Run("funnel enabled and expired, disable successfully", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		expiredTime := time.Now().UTC().Add(-5 * time.Minute)
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &expiredTime)
		mockCmdr.DisableFunnelFunc = func() error { return nil }

		service.BackgroundDisableCheck()

		if mockCmdr.DisableFunnelCalls != 1 {
			t.Errorf("mockCmdr.DisableFunnelCalls = %d, want 1", mockCmdr.DisableFunnelCalls)
		}
		dbState, _ := db.GetFunnelState()
		if dbState.Status != models.FunnelStatusDisabled {
			t.Errorf("DB state.Status = %s, want %s", dbState.Status, models.FunnelStatusDisabled)
		}
	})

	t.Run("funnel enabled and expired, disable command fails but already disabled", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		expiredTime := time.Now().UTC().Add(-5 * time.Minute)
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &expiredTime)
		
		mockCmdr.DisableFunnelFunc = func() error { return errors.New("cmd failed") }
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return false, nil } // Actually disabled

		service.BackgroundDisableCheck()

		if mockCmdr.DisableFunnelCalls != 1 {
			t.Errorf("mockCmdr.DisableFunnelCalls = %d, want 1", mockCmdr.DisableFunnelCalls)
		}
		if mockCmdr.GetActualFunnelStatusCalls != 1 {
			t.Errorf("mockCmdr.GetActualFunnelStatusCalls = %d, want 1", mockCmdr.GetActualFunnelStatusCalls)
		}
		dbState, _ := db.GetFunnelState()
		if dbState.Status != models.FunnelStatusDisabled {
			t.Errorf("DB state.Status = %s, want %s", dbState.Status, models.FunnelStatusDisabled)
		}
	})
	
	t.Run("funnel enabled and expired, disable command fails and still active", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		expiredTime := time.Now().UTC().Add(-5 * time.Minute)
		initialState, _ := db.UpdateFunnelState(models.FunnelStatusEnabled, &expiredTime)
		
		mockCmdr.DisableFunnelFunc = func() error { return errors.New("cmd failed") }
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return true, nil } // Actually still active

		service.BackgroundDisableCheck()

		if mockCmdr.DisableFunnelCalls != 1 {
			t.Errorf("mockCmdr.DisableFunnelCalls = %d, want 1", mockCmdr.DisableFunnelCalls)
		}
		if mockCmdr.GetActualFunnelStatusCalls != 1 {
			t.Errorf("mockCmdr.GetActualFunnelStatusCalls = %d, want 1", mockCmdr.GetActualFunnelStatusCalls)
		}
		dbState, _ := db.GetFunnelState()
		if dbState.Status != models.FunnelStatusEnabled { // Should remain enabled in DB
			t.Errorf("DB state.Status = %s, want %s", dbState.Status, models.FunnelStatusEnabled)
		}
		if dbState.DisableAtTimestamp == nil || !dbState.DisableAtTimestamp.Equal(*initialState.DisableAtTimestamp) {
             t.Errorf("DB state.DisableAtTimestamp = %v, want %v", dbState.DisableAtTimestamp, initialState.DisableAtTimestamp)
        }
	})

	t.Run("funnel enabled but not expired", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()

		futureTime := time.Now().UTC().Add(30 * time.Minute)
		_, _ = db.UpdateFunnelState(models.FunnelStatusEnabled, &futureTime)

		service.BackgroundDisableCheck()

		if mockCmdr.DisableFunnelCalls != 0 {
			t.Errorf("mockCmdr.DisableFunnelCalls = %d, want 0", mockCmdr.DisableFunnelCalls)
		}
		dbState, _ := db.GetFunnelState()
		if dbState.Status != models.FunnelStatusEnabled {
			t.Errorf("DB state.Status = %s, want %s", dbState.Status, models.FunnelStatusEnabled)
		}
	})

	t.Run("funnel disabled", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		service := NewFunnelService(db, mockCmdr)
		mockCmdr.Reset()
		// DB is default: DISABLED

		service.BackgroundDisableCheck()

		if mockCmdr.DisableFunnelCalls != 0 {
			t.Errorf("mockCmdr.DisableFunnelCalls = %d, want 0", mockCmdr.DisableFunnelCalls)
		}
	})
}
