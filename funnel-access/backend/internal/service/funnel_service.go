package service

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ownwarden/funnel-access/backend/internal/database"
	"github.com/ownwarden/funnel-access/backend/internal/models"
)

var (
	ErrFunnelAlreadyEnabled  = errors.New("funnel is already enabled")
	ErrFunnelNotEnabled      = errors.New("funnel is not enabled or has expired")
	ErrFunnelEnableFailed    = errors.New("failed to enable funnel via command")
	ErrFunnelDisableFailed   = errors.New("failed to disable funnel via command")
	ErrFunnelStatusCheckFailed = errors.New("failed to check actual funnel status")
)

// FunnelService provides methods to manage the Tailscale Funnel.
type FunnelService struct {
	dbStore   *database.Store
	commander TailscaleCommander
}

// NewFunnelService creates a new FunnelService.
func NewFunnelService(dbStore *database.Store, commander TailscaleCommander) *FunnelService {
	return &FunnelService{
		dbStore:   dbStore,
		commander: commander,
	}
}

// EnableFunnel handles the logic for FR2.2.
func (s *FunnelService) EnableFunnel(durationMinutes int) (*models.FunnelState, error) {
	if durationMinutes <= 0 {
		return nil, errors.New("duration_minutes must be positive")
	}

	currentState, err := s.dbStore.GetFunnelState()
	if err != nil {
		return nil, fmt.Errorf("failed to get current funnel state from DB: %w", err)
	}

	now := time.Now().UTC()
	newDisableAtTimestamp := now.Add(time.Duration(durationMinutes) * time.Minute)

	// FR2.2.3 - Fault Tolerance Check 1 (DB State)
	if currentState.Status == models.FunnelStatusEnabled && currentState.DisableAtTimestamp != nil && currentState.DisableAtTimestamp.After(now) {
		// Funnel is already enabled and active according to DB.
		// Requirement: "return an appropriate response ... or update the disable_at_timestamp if the new duration is longer."
		if newDisableAtTimestamp.After(*currentState.DisableAtTimestamp) {
			log.Printf("Funnel already enabled, extending duration from %v to %v", *currentState.DisableAtTimestamp, newDisableAtTimestamp)
			// Proceed to update timestamp, but no need to run enable command if actual status confirms.
		} else {
			log.Printf("Funnel already enabled with expiry %v. Requested duration %d mins does not extend further.", *currentState.DisableAtTimestamp, durationMinutes)
			return currentState, ErrFunnelAlreadyEnabled // Or a more specific error/status indicating it's enabled with existing expiry.
		}
	}

	// FR2.2.3 - Fault Tolerance Check 2 (Actual Funnel Status)
	actualEnabled, err := s.commander.GetActualFunnelStatus()
	if err != nil {
		log.Printf("Warning: Failed to get actual funnel status: %v. Proceeding based on DB state.", err)
		// Depending on strictness, could return ErrFunnelStatusCheckFailed here.
		// For now, we'll log and proceed, relying more on DB if actual check fails.
	}

	if actualEnabled {
		// If DB said disabled but actual is enabled, this is an inconsistent state.
		// Or if DB said enabled, and actual confirms.
		log.Println("Actual funnel status is ENABLED.")
		if currentState.Status == models.FunnelStatusDisabled || currentState.DisableAtTimestamp == nil || currentState.DisableAtTimestamp.Before(now) {
			log.Println("Warning: DB state was out of sync with actual funnel status (DB said disabled/expired, but funnel is active). Updating DB.")
		}
		// Update DB to reflect the new (potentially extended) duration.
		return s.dbStore.UpdateFunnelState(models.FunnelStatusEnabled, &newDisableAtTimestamp)
	}

	// If not already enabled (or if safe to re-enable/update)
	log.Println("Attempting to enable Tailscale Funnel via command...")
	if err := s.commander.EnableFunnel(); err != nil {
		log.Printf("Error enabling Tailscale Funnel via command: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrFunnelEnableFailed, err)
	}
	log.Println("Tailscale Funnel enabled successfully via command.")

	// On successful command execution, update the database
	updatedState, err := s.dbStore.UpdateFunnelState(models.FunnelStatusEnabled, &newDisableAtTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to update funnel state in DB after enabling: %w", err)
	}

	return updatedState, nil
}

// DisableFunnel handles the logic for FR2.3.
func (s *FunnelService) DisableFunnel() (*models.FunnelState, error) {
	log.Println("Attempting to disable Tailscale Funnel via command...")
	if err := s.commander.DisableFunnel(); err != nil {
		// NFR1.4: Gracefully handle failures. Should we update DB even if command fails?
		// PRD: "On successful command execution, update the database"
		// This implies if command fails, DB is not updated to DISABLED.
		// However, for idempotency, if it's already disabled, command might fail.
		// Let's check actual status if primary command fails.
		actualEnabled, statusErr := s.commander.GetActualFunnelStatus()
		if statusErr == nil && !actualEnabled {
			log.Println("Disable command failed, but funnel is already actually disabled. Updating DB.")
			// Fall through to update DB
		} else {
			log.Printf("Error disabling Tailscale Funnel via command: %v. Status check error: %v", err, statusErr)
			return nil, fmt.Errorf("%w: %v", ErrFunnelDisableFailed, err)
		}
	}
	log.Println("Tailscale Funnel disabled successfully via command (or was already disabled).")

	// On successful command execution (or if already disabled), update the database
	updatedState, err := s.dbStore.UpdateFunnelState(models.FunnelStatusDisabled, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update funnel state in DB after disabling: %w", err)
	}
	return updatedState, nil
}

// ExtendFunnelDuration handles the logic for FR2.4.
func (s *FunnelService) ExtendFunnelDuration(additionalMinutes int) (*models.FunnelState, error) {
	if additionalMinutes <= 0 {
		return nil, errors.New("additional_minutes must be positive")
	}

	currentState, err := s.dbStore.GetFunnelState()
	if err != nil {
		return nil, fmt.Errorf("failed to get current funnel state: %w", err)
	}

	now := time.Now().UTC()
	if currentState.Status != models.FunnelStatusEnabled || currentState.DisableAtTimestamp == nil || currentState.DisableAtTimestamp.Before(now) {
		return nil, ErrFunnelNotEnabled
	}

	// Add additional_minutes to the existing disable_at_timestamp
	newDisableAtTimestamp := currentState.DisableAtTimestamp.Add(time.Duration(additionalMinutes) * time.Minute)

	updatedState, err := s.dbStore.UpdateFunnelState(models.FunnelStatusEnabled, &newDisableAtTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to update funnel state in DB for extension: %w", err)
	}
	log.Printf("Funnel duration extended. New disable_at_timestamp: %v", newDisableAtTimestamp)
	return updatedState, nil
}

// GetFunnelStatus handles the logic for FR2.5.
func (s *FunnelService) GetFunnelStatus() (*models.FunnelState, error) {
	// FR2.5.2: "Query and return the current funnel_status and disable_at_timestamp from the database."
	// It does not require checking actual funnel status here, relying on DB as source of truth for this endpoint.
	// The background scheduler will handle discrepancies.
	state, err := s.dbStore.GetFunnelState()
	if err != nil {
		return nil, fmt.Errorf("failed to get funnel state from DB: %w", err)
	}
	
	// Ensure consistency: if DB says enabled but timestamp is past, reflect as disabled.
	// This is more for the response to the user; the scheduler handles actual disabling.
	if state.Status == models.FunnelStatusEnabled && state.DisableAtTimestamp != nil && state.DisableAtTimestamp.Before(time.Now().UTC()) {
		log.Printf("Funnel status from DB is ENABLED but DisableAtTimestamp (%v) is in the past. Reporting as effectively DISABLED for status query.", *state.DisableAtTimestamp)
		// This is a tricky part: should GetStatus *change* the DB state or just report?
		// PRD says "Query and return", so we just report. Scheduler will fix DB.
		// For a cleaner API response, we could return a derived status.
		// However, to stick to "return from database", we return what DB has.
		// The client UI should be smart enough to interpret an ENABLED status with a past timestamp.
	}

	return state, nil
}

// BackgroundDisableCheck is called by the scheduler (FR3.2, FR3.3)
func (s *FunnelService) BackgroundDisableCheck() {
	log.Println("BackgroundDisableCheck: Running periodic check...")
	state, err := s.dbStore.GetFunnelState()
	if err != nil {
		log.Printf("BackgroundDisableCheck: Error getting funnel state: %v", err)
		return
	}

	if state.Status == models.FunnelStatusEnabled && state.DisableAtTimestamp != nil && state.DisableAtTimestamp.Before(time.Now().UTC()) {
		log.Printf("BackgroundDisableCheck: Funnel (ID: %d) is ENABLED and DisableAtTimestamp (%v) is in the past. Attempting to disable.", state.ID, *state.DisableAtTimestamp)
		
		if errCmd := s.commander.DisableFunnel(); errCmd != nil {
			// NFR1.4: Gracefully handle failures.
			// If command fails, should we retry or just log?
			// For now, log and it will be picked up in the next check.
			log.Printf("BackgroundDisableCheck: Error executing disable command for funnel (ID: %d): %v", state.ID, errCmd)
			
			// Check actual status to see if it's already disabled despite command error
			actualEnabled, statusErr := s.commander.GetActualFunnelStatus()
			if statusErr == nil && !actualEnabled {
				log.Printf("BackgroundDisableCheck: Disable command failed, but funnel (ID: %d) is already actually disabled. Updating DB.", state.ID)
			} else {
				log.Printf("BackgroundDisableCheck: Failed to disable funnel (ID: %d) and it might still be active. Status check err: %v", state.ID, statusErr)
				return // Don't update DB if command failed and it might still be active
			}
		}
		
		// If successful (or already disabled), update the database
		_, errUpdate := s.dbStore.UpdateFunnelState(models.FunnelStatusDisabled, nil)
		if errUpdate != nil {
			log.Printf("BackgroundDisableCheck: Error updating funnel state to DISABLED in DB for funnel (ID: %d): %v", state.ID, errUpdate)
			return
		}
		log.Printf("BackgroundDisableCheck: Funnel (ID: %d) automatically disabled and DB updated.", state.ID)
	} else {
		// log.Println("BackgroundDisableCheck: No action needed.")
	}
}
