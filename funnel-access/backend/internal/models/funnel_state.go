package models

import "time"

// FunnelStatus represents the status of the Tailscale Funnel.
type FunnelStatus string

const (
	// FunnelStatusEnabled indicates the funnel is active.
	FunnelStatusEnabled FunnelStatus = "ENABLED"
	// FunnelStatusDisabled indicates the funnel is not active.
	FunnelStatusDisabled FunnelStatus = "DISABLED"
)

// FunnelState represents the persisted state of the Tailscale Funnel.
type FunnelState struct {
	ID                  int64        `json:"-"` // Primary key for DB
	Status              FunnelStatus `json:"funnel_status"`
	DisableAtTimestamp  *time.Time   `json:"disable_at_timestamp"` // Pointer to allow NULL
	LastUpdatedTimestamp time.Time    `json:"last_updated_timestamp"`
}
