package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/database"
	"github.com/ownwarden/funnel-access/backend/internal/models"
	"github.com/ownwarden/funnel-access/backend/internal/service"
	// token package import removed

	// SQLite driver for tests
	_ "github.com/mattn/go-sqlite3"
)

// testAPISecretKey constant removed

// setupTestAPIHandler initializes an APIHandler with a real FunnelService
// (using an in-memory DB and MockTailscaleCommander).
// It also returns the dbStore for direct manipulation in tests if needed.
func setupTestAPIHandler(t *testing.T) (*APIHandler, *service.MockTailscaleCommander, *database.Store, func()) {
	// Setup in-memory DB for FunnelService
	tempDir, err := os.MkdirTemp("", "handler_test_db_")
	if err != nil {
		t.Fatalf("Failed to create temp dir for test DB: %v", err)
	}
	dbPath := filepath.Join(tempDir, "test_handler_funnel_controller.db")
	dbStore, err := database.NewStore(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create test database store: %v", err)
	}

	mockCmdr := &service.MockTailscaleCommander{}
	funnelService := service.NewFunnelService(dbStore, mockCmdr)

	// tokenManager creation removed

	apiHandler := NewAPIHandler(funnelService) // tokenManager removed from call

	cleanup := func() {
		dbStore.Close()
		os.RemoveAll(tempDir)
	}

	return apiHandler, mockCmdr, dbStore, cleanup
}

func TestEnableFunnelHandler(t *testing.T) {
	handler, mockCmdr, _, cleanup := setupTestAPIHandler(t) // dbStore not directly needed here
	defer cleanup()

	router := NewRouter(handler) // Use the actual router to test middleware wiring

	t.Run("success", func(t *testing.T) {
		mockCmdr.Reset()
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return false, nil } // Funnel is off
		mockCmdr.EnableFunnelFunc = func() error { return nil }                          // Enable command succeeds

		payload := `{"duration_minutes": 30}`
		req := httptest.NewRequest(http.MethodPost, "/api/funnel/enable", strings.NewReader(payload))
		// authToken generation and setting removed
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
		}

		var resp FunnelStatusResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Could not decode response: %v", err)
		}
		if resp.Status != string(models.FunnelStatusEnabled) {
			t.Errorf("response status = %s, want %s", resp.Status, models.FunnelStatusEnabled)
		}
		if resp.DisableAtTimestamp == nil {
			t.Error("response DisableAtTimestamp is nil, want non-nil")
		}
		if mockCmdr.EnableFunnelCalls != 1 {
			t.Errorf("EnableFunnel on commander was called %d times, want 1", mockCmdr.EnableFunnelCalls)
		}
	})

	// "missing token" and "invalid token" test cases removed
	
	t.Run("bad request - invalid duration", func(t *testing.T) {
		mockCmdr.Reset()
		payload := `{"duration_minutes": -5}` // Invalid duration
		req := httptest.NewRequest(http.MethodPost, "/api/funnel/enable", strings.NewReader(payload))
		// authToken generation and setting removed
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusBadRequest, rr.Body.String())
		}
	})

	t.Run("funnel already enabled - not extended", func(t *testing.T) {
		mockCmdr.Reset()
		// Setup: Funnel is already enabled for 60 minutes
		initialTime := time.Now().Add(60 * time.Minute)
		_,_ = handler.funnelService.EnableFunnel(60) // Enable it first
		mockCmdr.Reset() // Reset calls after setup
		mockCmdr.GetActualFunnelStatusFunc = func() (bool, error) { return true, nil } // Funnel is on

		payload := `{"duration_minutes": 30}` // Request for shorter duration
		req := httptest.NewRequest(http.MethodPost, "/api/funnel/enable", strings.NewReader(payload))
		// authToken generation and setting removed
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK { // Service returns OK with "already enabled" message
			t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
		}
		var resp FunnelStatusResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("Could not decode response: %v", err)
		}
		if !strings.Contains(resp.Message, "Funnel is already enabled") {
			t.Errorf("response message = '%s', want to contain 'Funnel is already enabled'", resp.Message)
		}
		if resp.DisableAtTimestamp == nil || resp.DisableAtTimestamp.Before(initialTime.Add(-1 * time.Minute)) {
             t.Errorf("Expected disable time to be around %v, got %v", initialTime, resp.DisableAtTimestamp)
        }
		if mockCmdr.EnableFunnelCalls != 0 { // Should not call enable again
			t.Errorf("EnableFunnel on commander was called %d times, want 0", mockCmdr.EnableFunnelCalls)
		}
	})
}


func TestDisableFunnelHandler(t *testing.T) {
    handler, mockCmdr, _, cleanup := setupTestAPIHandler(t) // dbStore not directly needed here
    defer cleanup()
    router := NewRouter(handler)

    t.Run("success", func(t *testing.T) {
        mockCmdr.Reset()
        // Pre-condition: funnel is enabled in DB (service will handle actual command)
        _, _ = handler.funnelService.EnableFunnel(30) // Enable it first
        mockCmdr.Reset() // Reset calls after setup
        mockCmdr.DisableFunnelFunc = func() error { return nil } // Disable command succeeds

        req := httptest.NewRequest(http.MethodPost, "/api/funnel/disable", nil)
        // authToken generation and setting removed

        rr := httptest.NewRecorder()
        router.ServeHTTP(rr, req)

        if status := rr.Code; status != http.StatusOK {
            t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
        }
        var resp FunnelStatusResponse
        if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
            t.Fatalf("Could not decode response: %v", err)
        }
        if resp.Status != string(models.FunnelStatusDisabled) {
            t.Errorf("response status = %s, want %s", resp.Status, models.FunnelStatusDisabled)
        }
        if mockCmdr.DisableFunnelCalls != 1 {
            t.Errorf("DisableFunnel on commander was called %d times, want 1", mockCmdr.DisableFunnelCalls)
        }
    })
}

func TestExtendFunnelHandler(t *testing.T) {
    handler, mockCmdr, _, cleanup := setupTestAPIHandler(t) // dbStore not directly needed here
    defer cleanup()
    router := NewRouter(handler)

    t.Run("success", func(t *testing.T) {
        mockCmdr.Reset()
        // Pre-condition: funnel is enabled
        _, _ = handler.funnelService.EnableFunnel(10) // Enable for 10 mins
        mockCmdr.Reset()

        payload := `{"additional_minutes": 20}`
        req := httptest.NewRequest(http.MethodPost, "/api/funnel/extend", strings.NewReader(payload))
        // authToken generation and setting removed
        req.Header.Set("Content-Type", "application/json")

        rr := httptest.NewRecorder()
        router.ServeHTTP(rr, req)

        if status := rr.Code; status != http.StatusOK {
            t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
        }
        var resp FunnelStatusResponse
        if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
            t.Fatalf("Could not decode response: %v", err)
        }
        if resp.Status != string(models.FunnelStatusEnabled) {
            t.Errorf("response status = %s, want %s", resp.Status, models.FunnelStatusEnabled)
        }
        if resp.DisableAtTimestamp == nil {
            t.Error("response DisableAtTimestamp is nil")
        } else {
            // Original 10 mins + 20 additional mins = 30 mins from now (roughly)
            expectedMinExpiry := time.Now().Add(29 * time.Minute)
            if resp.DisableAtTimestamp.Before(expectedMinExpiry) {
                t.Errorf("Expected DisableAtTimestamp to be around 30 mins from now, got %v", resp.DisableAtTimestamp)
            }
        }
    })

    t.Run("funnel not enabled", func(t *testing.T) {
        mockCmdr.Reset()
        // Ensure funnel is disabled
        _, _ = handler.funnelService.DisableFunnel()
        mockCmdr.Reset()


        payload := `{"additional_minutes": 20}`
        req := httptest.NewRequest(http.MethodPost, "/api/funnel/extend", strings.NewReader(payload))
        // authToken generation and setting removed
        req.Header.Set("Content-Type", "application/json")

        rr := httptest.NewRecorder()
        router.ServeHTTP(rr, req)

        if status := rr.Code; status != http.StatusConflict {
            t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusConflict, rr.Body.String())
        }
    })
}

func TestGetFunnelStatusHandler(t *testing.T) {
    handler, _, dbStore, cleanup := setupTestAPIHandler(t) // mockCmdr not strictly needed, dbStore is
    defer cleanup()
    router := NewRouter(handler)

    t.Run("success - disabled", func(t *testing.T) {
        // Ensure funnel is disabled
        _, _ = handler.funnelService.DisableFunnel()

        req := httptest.NewRequest(http.MethodGet, "/api/funnel/status", nil)
        // authToken setting removed

        rr := httptest.NewRecorder()
        router.ServeHTTP(rr, req)

        if status := rr.Code; status != http.StatusOK {
            t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
        }
        var resp FunnelStatusResponse
        if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
            t.Fatalf("Could not decode response: %v", err)
        }
        if resp.Status != string(models.FunnelStatusDisabled) {
            t.Errorf("response status = %s, want %s", resp.Status, models.FunnelStatusDisabled)
        }
    })

    t.Run("success - enabled", func(t *testing.T) {
        // Ensure funnel is enabled
        targetTime := time.Now().Add(30 * time.Minute)
        _, err := dbStore.UpdateFunnelState(models.FunnelStatusEnabled, &targetTime)
		if err != nil {
			t.Fatalf("Failed to set DB state for test: %v", err)
		}

        req := httptest.NewRequest(http.MethodGet, "/api/funnel/status", nil)
        // authToken setting removed

        rr := httptest.NewRecorder()
        router.ServeHTTP(rr, req)

        if status := rr.Code; status != http.StatusOK {
            t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
        }
        var resp FunnelStatusResponse
        if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
            t.Fatalf("Could not decode response: %v", err)
        }
        if resp.Status != string(models.FunnelStatusEnabled) {
            t.Errorf("response status = %s, want %s", resp.Status, models.FunnelStatusEnabled)
        }
        if resp.DisableAtTimestamp == nil || resp.DisableAtTimestamp.Before(targetTime.Add(-time.Second)) || resp.DisableAtTimestamp.After(targetTime.Add(time.Second)) {
             t.Errorf("response DisableAtTimestamp = %v, want around %v", resp.DisableAtTimestamp, targetTime)
        }
    })
}
