package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/csrf"
	"github.com/ownwarden/funnel-access/backend/internal/models"
	"github.com/ownwarden/funnel-access/backend/internal/service"
	// token package import removed
)

// APIHandler holds dependencies for HTTP handlers.
type APIHandler struct {
	funnelService *service.FunnelService
	// tokenManager field removed
}

// NewAPIHandler creates a new APIHandler.
func NewAPIHandler(fs *service.FunnelService) *APIHandler {
	return &APIHandler{
		funnelService: fs,
		// tokenManager initialization removed
	}
}

// respondWithError sends a JSON error response.
func respondWithError(w http.ResponseWriter, code int, message string, details ...string) {
	errResp := ErrorResponse{Error: message}
	if len(details) > 0 {
		errResp.Details = details[0]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		log.Printf("Error encoding error response: %v", err)
	}
}

// respondWithJSON sends a JSON success response.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if payload != nil {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			log.Printf("Error encoding success response: %v", err)
			// Attempt to send a basic error if encoding the main payload fails
			http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
		}
	}
}

// tokenAuthMiddleware removed

// EnableFunnelHandler handles POST /api/funnel/enable
func (h *APIHandler) EnableFunnelHandler(w http.ResponseWriter, r *http.Request) {
	var req EnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}
	defer r.Body.Close()

	if req.DurationMinutes <= 0 {
		respondWithError(w, http.StatusBadRequest, "duration_minutes must be a positive integer")
		return
	}

	state, err := h.funnelService.EnableFunnel(req.DurationMinutes)
	if err != nil {
		log.Printf("EnableFunnel service error: %v", err)
		if errors.Is(err, service.ErrFunnelAlreadyEnabled) {
			// Return current state if already enabled and not extended
			respondWithJSON(w, http.StatusOK, FunnelStatusResponse{
				Status:             string(state.Status),
				DisableAtTimestamp: state.DisableAtTimestamp,
				Message:            "Funnel is already enabled.",
			})
			return
		}
		if errors.Is(err, service.ErrFunnelEnableFailed) {
			respondWithError(w, http.StatusInternalServerError, "Failed to enable funnel", err.Error())
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Error enabling funnel", err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, FunnelStatusResponse{
		Status:             string(state.Status),
		DisableAtTimestamp: state.DisableAtTimestamp,
		Message:            "Funnel enabled successfully.",
	})
}

// DisableFunnelHandler handles POST /api/funnel/disable
func (h *APIHandler) DisableFunnelHandler(w http.ResponseWriter, r *http.Request) {
	state, err := h.funnelService.DisableFunnel()
	if err != nil {
		log.Printf("DisableFunnel service error: %v", err)
		if errors.Is(err, service.ErrFunnelDisableFailed) {
			respondWithError(w, http.StatusInternalServerError, "Failed to disable funnel", err.Error())
			return
		}
		// Consider if it's already disabled - service might handle this gracefully.
		// If service returns an error for "already disabled", map it to a specific HTTP response.
		// Current service logic updates DB even if already disabled (idempotency).
		respondWithError(w, http.StatusInternalServerError, "Error disabling funnel", err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, FunnelStatusResponse{
		Status:  string(state.Status),
		Message: "Funnel disabled successfully.",
	})
}

// ExtendFunnelHandler handles POST /api/funnel/extend
func (h *APIHandler) ExtendFunnelHandler(w http.ResponseWriter, r *http.Request) {
	var req ExtendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}
	defer r.Body.Close()

	if req.AdditionalMinutes <= 0 {
		respondWithError(w, http.StatusBadRequest, "additional_minutes must be a positive integer")
		return
	}

	state, err := h.funnelService.ExtendFunnelDuration(req.AdditionalMinutes)
	if err != nil {
		log.Printf("ExtendFunnelDuration service error: %v", err)
		if errors.Is(err, service.ErrFunnelNotEnabled) {
			respondWithError(w, http.StatusConflict, "Funnel is not enabled or has expired, cannot extend.")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Error extending funnel duration", err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, FunnelStatusResponse{
		Status:             string(state.Status),
		DisableAtTimestamp: state.DisableAtTimestamp,
		Message:            "Funnel duration extended successfully.",
	})
}

// GetFunnelStatusHandler handles GET /api/funnel/status
func (h *APIHandler) GetFunnelStatusHandler(w http.ResponseWriter, r *http.Request) {
	state, err := h.funnelService.GetFunnelStatus()
	if err != nil {
		log.Printf("GetFunnelStatus service error: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Error getting funnel status", err.Error())
		return
	}
	
	// The service layer's GetFunnelStatus already logs if it's enabled but past due.
	// The UI should interpret an ENABLED status with a past timestamp as effectively disabled.
	statusStr := string(state.Status)
	if state.Status == models.FunnelStatusEnabled && state.DisableAtTimestamp != nil && state.DisableAtTimestamp.Before(time.Now().UTC()){
		// Optionally, the API could choose to report this as DISABLED for clarity to simple clients,
		// though the PRD implies returning DB state.
		// statusStr = string(models.FunnelStatusDisabled) // Example if we wanted to override
		log.Printf("GetFunnelStatusHandler: Funnel is ENABLED in DB but DisableAtTimestamp (%v) is past. Reporting DB state.", *state.DisableAtTimestamp)
	}

	w.Header().Set("X-CSRF-Token", csrf.Token(r))
	respondWithJSON(w, http.StatusOK, FunnelStatusResponse{
		Status:             statusStr,
		DisableAtTimestamp: state.DisableAtTimestamp,
	})
}

// GenerateTokenHandler removed
