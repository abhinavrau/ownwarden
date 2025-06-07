package service

// TailscaleCommander defines the interface for executing Tailscale commands.
// This allows for mocking in tests.
type TailscaleCommander interface {
	// EnableFunnel executes the command to enable Tailscale Funnel.
	// It should return an error if the command fails.
	EnableFunnel() error

	// DisableFunnel executes the command to disable Tailscale Funnel.
	// It should return an error if the command fails.
	DisableFunnel() error

	// GetFunnelStatus executes a command to check the actual status of Tailscale Funnel.
	// Returns true if enabled, false if disabled, and an error if status cannot be determined.
	GetActualFunnelStatus() (isEnabled bool, err error)
}
