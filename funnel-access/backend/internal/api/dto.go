package api

import "time"

// EnableRequest is the DTO for the POST /api/funnel/enable endpoint.
type EnableRequest struct {
	DurationMinutes int `json:"duration_minutes"`
}

// ExtendRequest is the DTO for the POST /api/funnel/extend endpoint.
type ExtendRequest struct {
	AdditionalMinutes int `json:"additional_minutes"`
}

// FunnelStatusResponse is the DTO for funnel status responses.
// Used by GET /api/funnel/status and successful POST responses.
type FunnelStatusResponse struct {
	Status             string     `json:"funnel_status"`
	DisableAtTimestamp *time.Time `json:"disable_at_timestamp,omitempty"` // omitempty for null
	Message            string     `json:"message,omitempty"`
}

// ErrorResponse is a generic DTO for API error responses.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// TokenResponse is used for an endpoint that might generate tokens (if we add one).
// For now, tokens are expected to be passed in headers.
// If FR1.2 implies an endpoint to get a token, this would be its response.
// The current PRD says "A secure mechanism ... must exist for an authorized client to obtain a valid token."
// This might be out of scope for this backend-only task if the client is assumed to have a pre-shared key.
// For now, I'll include it as a placeholder.
type TokenResponse struct {
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
}
