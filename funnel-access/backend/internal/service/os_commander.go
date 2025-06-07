package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	defaultServeConfigPath  = "/config/serve-config.json" // Path inside the container
	defaultFunnelKeyPattern = "${TS_CERT_DOMAIN}:443"    // The literal key from the example serve-config.json
)

// ServeConfig represents the structure of the ts_config/serve-config.json file.
type ServeConfig struct {
	TCP         map[string]map[string]bool `json:"TCP,omitempty"`
	Web         map[string]WebConfig       `json:"Web,omitempty"`
	AllowFunnel map[string]bool            `json:"AllowFunnel"` // Should not be omitempty if we want to write "AllowFunnel": {}
}

// WebConfig represents the "Web" section for a specific domain.
type WebConfig struct {
	Handlers map[string]HandlerConfig `json:"Handlers,omitempty"`
}

// HandlerConfig represents a specific handler, e.g., for "/".
type HandlerConfig struct {
	Proxy string `json:"Proxy,omitempty"`
}

// FileTailscaleCommander implements TailscaleCommander by modifying the serve-config.json file.
type FileTailscaleCommander struct {
	ServeConfigPath string
	FunnelKey       string // The key in the AllowFunnel map, e.g., "${TS_CERT_DOMAIN}:443"
}

// NewFileTailscaleCommander creates a new FileTailscaleCommander.
// It attempts to read configuration from environment variables first (SERVE_CONFIG_PATH, FUNNEL_KEY),
// then falls back to provided arguments or defaults.
func NewFileTailscaleCommander(serveConfigPathArg, funnelKeyArg string) *FileTailscaleCommander {
	serveConfigPath := os.Getenv("SERVE_CONFIG_PATH")
	if serveConfigPath == "" {
		serveConfigPath = serveConfigPathArg // Use arg if env is not set
	}
	if serveConfigPath == "" {
		serveConfigPath = defaultServeConfigPath // Fallback to default if neither env var nor arg is provided
		log.Printf("FileTailscaleCommander: SERVE_CONFIG_PATH not set via env or arg, defaulting to '%s'", defaultServeConfigPath)
	}

	funnelKey := os.Getenv("FUNNEL_KEY")
	if funnelKey == "" {
		funnelKey = funnelKeyArg // Use arg if env is not set
	}
	if funnelKey == "" {
		// Fallback to the literal pattern if neither env var nor arg is provided
		funnelKey = defaultFunnelKeyPattern
		log.Printf("FileTailscaleCommander: FUNNEL_KEY not set via env or arg, defaulting to literal pattern '%s'", defaultFunnelKeyPattern)
	}

	log.Printf("FileTailscaleCommander initialized with ServeConfigPath: '%s', FunnelKey: '%s'", serveConfigPath, funnelKey)

	return &FileTailscaleCommander{
		ServeConfigPath: serveConfigPath,
		FunnelKey:       funnelKey,
	}
}

func (c *FileTailscaleCommander) readServeConfig() (*ServeConfig, error) {
	data, err := os.ReadFile(c.ServeConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Serve config file '%s' does not exist. Creating a default structure.", c.ServeConfigPath)
			return &ServeConfig{
				AllowFunnel: make(map[string]bool),
				TCP:         make(map[string]map[string]bool),
				Web:         make(map[string]WebConfig),
			}, nil
		}
		return nil, fmt.Errorf("failed to read serve config file '%s': %w", c.ServeConfigPath, err)
	}

	var config ServeConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal serve config data from '%s': %w. Content snippet: %s", c.ServeConfigPath, err, string(data[:min(len(data), 200)]))
	}

	// Ensure maps are initialized if they were nil (e.g. empty JSON object "{}" was read or file had partial data)
	if config.AllowFunnel == nil {
		config.AllowFunnel = make(map[string]bool)
	}
	if config.TCP == nil {
		config.TCP = make(map[string]map[string]bool)
	}
	if config.Web == nil {
		config.Web = make(map[string]WebConfig)
	}

	return &config, nil
}

func (c *FileTailscaleCommander) writeServeConfig(config *ServeConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal serve config: %w", err)
	}

	dir := filepath.Dir(c.ServeConfigPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", dir, err)
		}
	}

	err = os.WriteFile(c.ServeConfigPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write serve config file '%s': %w", c.ServeConfigPath, err)
	}
	log.Printf("Successfully wrote serve config to '%s'", c.ServeConfigPath)
	return nil
}

// EnableFunnel sets the AllowFunnel property to true for the configured key.
func (c *FileTailscaleCommander) EnableFunnel() error {
	log.Printf("Enabling funnel by modifying file: %s for key: %s", c.ServeConfigPath, c.FunnelKey)
	config, err := c.readServeConfig()
	if err != nil {
		return fmt.Errorf("failed to read serve config for enabling funnel: %w", err)
	}

	config.AllowFunnel[c.FunnelKey] = true

	err = c.writeServeConfig(config)
	if err != nil {
		return fmt.Errorf("failed to write serve config for enabling funnel: %w", err)
	}
	log.Printf("Funnel enabled for key '%s' in '%s'", c.FunnelKey, c.ServeConfigPath)
	return nil
}

// DisableFunnel sets the AllowFunnel property to false for the configured key.
func (c *FileTailscaleCommander) DisableFunnel() error {
	log.Printf("Disabling funnel by modifying file: %s for key: %s", c.ServeConfigPath, c.FunnelKey)
	config, err := c.readServeConfig()
	if err != nil {
		return fmt.Errorf("failed to read serve config for disabling funnel: %w", err)
	}

	config.AllowFunnel[c.FunnelKey] = false // If key doesn't exist, it's created and set to false.

	err = c.writeServeConfig(config)
	if err != nil {
		return fmt.Errorf("failed to write serve config for disabling funnel: %w", err)
	}
	log.Printf("Funnel disabled for key '%s' in '%s'", c.FunnelKey, c.ServeConfigPath)
	return nil
}

// GetActualFunnelStatus reads the AllowFunnel property for the configured key.
func (c *FileTailscaleCommander) GetActualFunnelStatus() (bool, error) {
	log.Printf("Getting funnel status from file: %s for key: %s", c.ServeConfigPath, c.FunnelKey)
	config, err := c.readServeConfig()
	if err != nil {
		log.Printf("Error reading serve config for status check: %v. Assuming funnel is OFF or status unknown.", err)
		return false, fmt.Errorf("failed to read serve config for status check: %w", err)
	}

	status, exists := config.AllowFunnel[c.FunnelKey]
	if !exists {
		log.Printf("Funnel key '%s' not found in AllowFunnel map, status is false (disabled)", c.FunnelKey)
		return false, nil
	}

	log.Printf("Funnel status for key '%s' is %t", c.FunnelKey, status)
	return status, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
