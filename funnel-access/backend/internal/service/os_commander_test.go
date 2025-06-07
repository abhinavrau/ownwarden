package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFileTailscaleCommander tests the NewFileTailscaleCommander constructor.
func TestNewFileTailscaleCommander(t *testing.T) {
	defaultPath := "/config/serve-config.json" // As defined in os_commander.go
	defaultKey := "${TS_CERT_DOMAIN}:443"    // As defined in os_commander.go

	// Helper to manage environment variables for tests
	setEnv := func(key, value string) (originalValue string, wasSet bool) {
		originalValue, wasSet = os.LookupEnv(key)
		err := os.Setenv(key, value)
		require.NoError(t, err)
		return
	}
	restoreEnv := func(key, originalValue string, wasSet bool) {
		if wasSet {
			err := os.Setenv(key, originalValue)
			require.NoError(t, err)
		} else {
			err := os.Unsetenv(key)
			require.NoError(t, err)
		}
	}

	tests := []struct {
		name              string
		envServePathVal   *string // Use pointer to distinguish between empty string and not set
		envFunnelKeyVal   *string // Use pointer to distinguish between empty string and not set
		argServePath      string
		argFunnelKey      string
		expectedServePath string
		expectedFunnelKey string
	}{
		{
			name:              "all defaults",
			expectedServePath: defaultPath,
			expectedFunnelKey: defaultKey,
		},
		{
			name:              "args provided, no env vars",
			argServePath:      "/arg/path",
			argFunnelKey:      "arg_key",
			expectedServePath: "/arg/path",
			expectedFunnelKey: "arg_key",
		},
		{
			name:              "env vars provided, no args",
			envServePathVal:   strPtr("/env/path"),
			envFunnelKeyVal:   strPtr("env_key"),
			expectedServePath: "/env/path",
			expectedFunnelKey: "env_key",
		},
		{
			name:              "env vars override args",
			envServePathVal:   strPtr("/env/override/path"),
			envFunnelKeyVal:   strPtr("env_override_key"),
			argServePath:      "/arg/path/ignored",
			argFunnelKey:      "arg_key_ignored",
			expectedServePath: "/env/override/path",
			expectedFunnelKey: "env_override_key",
		},
		{
			name:              "env var for path, arg for key",
			envServePathVal:   strPtr("/env/path/only"),
			argFunnelKey:      "arg_key_for_env_path",
			expectedServePath: "/env/path/only",
			expectedFunnelKey: "arg_key_for_env_path",
		},
		{
			name:              "arg for path, env var for key",
			argServePath:      "/arg/path_for_env_key",
			envFunnelKeyVal:   strPtr("env_key_for_arg_path"),
			expectedServePath: "/arg/path_for_env_key",
			expectedFunnelKey: "env_key_for_arg_path",
		},
		{
			name:              "empty string env vars, args used",
			envServePathVal:   strPtr(""), // Empty string set in env
			envFunnelKeyVal:   strPtr(""), // Empty string set in env
			argServePath:      "/arg/path/if/env/empty",
			argFunnelKey:      "arg_key_if/env/empty",
			expectedServePath: "/arg/path/if/env/empty",
			expectedFunnelKey: "arg_key_if/env/empty",
		},
		{
			name:              "empty string env vars, no args, defaults used",
			envServePathVal:   strPtr(""), // Empty string set in env
			envFunnelKeyVal:   strPtr(""), // Empty string set in env
			expectedServePath: defaultPath,
			expectedFunnelKey: defaultKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var origServePathVal string
			var servePathWasSet bool
			if tt.envServePathVal != nil {
				origServePathVal, servePathWasSet = setEnv("SERVE_CONFIG_PATH", *tt.envServePathVal)
				defer restoreEnv("SERVE_CONFIG_PATH", origServePathVal, servePathWasSet)
			} else {
				origServePathVal, servePathWasSet = os.LookupEnv("SERVE_CONFIG_PATH")
				if servePathWasSet {
					err := os.Unsetenv("SERVE_CONFIG_PATH")
					require.NoError(t, err)
					defer func() {
						err := os.Setenv("SERVE_CONFIG_PATH", origServePathVal)
						require.NoError(t, err)
					}()
				}
			}

			var origFunnelKeyVal string
			var funnelKeyWasSet bool
			if tt.envFunnelKeyVal != nil {
				origFunnelKeyVal, funnelKeyWasSet = setEnv("FUNNEL_KEY", *tt.envFunnelKeyVal)
				defer restoreEnv("FUNNEL_KEY", origFunnelKeyVal, funnelKeyWasSet)
			} else {
				origFunnelKeyVal, funnelKeyWasSet = os.LookupEnv("FUNNEL_KEY")
				if funnelKeyWasSet {
					err := os.Unsetenv("FUNNEL_KEY")
					require.NoError(t, err)
					defer func() {
						err := os.Setenv("FUNNEL_KEY", origFunnelKeyVal)
						require.NoError(t, err)
					}()
				}
			}

			commander := NewFileTailscaleCommander(tt.argServePath, tt.argFunnelKey)
			assert.Equal(t, tt.expectedServePath, commander.ServeConfigPath)
			assert.Equal(t, tt.expectedFunnelKey, commander.FunnelKey)
		})
	}
}

// Helper to convert string to pointer for test cases
func strPtr(s string) *string {
	return &s
}

// Helper to create a temporary config file
func createTempConfigFile(t *testing.T, initialContent *ServeConfig) (configFilePath string, cleanupFunc func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "serveconfigtest")
	require.NoError(t, err, "Failed to create temp dir")

	configFilePath = filepath.Join(tempDir, "serve-config.json")

	if initialContent != nil {
		data, err := json.MarshalIndent(initialContent, "", "  ")
		require.NoError(t, err, "Failed to marshal initial config")
		err = os.WriteFile(configFilePath, data, 0644)
		require.NoError(t, err, "Failed to write initial config file")
	}

	cleanupFunc = func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			// Log error during cleanup but don't fail the test
			t.Logf("Warning: failed to remove temp dir %s: %v", tempDir, err)
		}
	}
	return configFilePath, cleanupFunc
}

func TestFileTailscaleCommander_readServeConfig(t *testing.T) {
	defaultFunnelKey := "test.example.com:443"

	t.Run("file does not exist, creates default", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "readtest_nonexistent")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		nonExistentPath := filepath.Join(tempDir, "nonexistent-serve-config.json")
		_, statErr := os.Stat(nonExistentPath)
		require.True(t, os.IsNotExist(statErr), "File should not exist before test")

		commander := NewFileTailscaleCommander(nonExistentPath, defaultFunnelKey)

		config, err := commander.readServeConfig()
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.NotNil(t, config.TCP, "TCP should be initialized")
		assert.NotNil(t, config.Web, "Web should be initialized")
		assert.NotNil(t, config.AllowFunnel, "AllowFunnel should be initialized")
		assert.Empty(t, config.TCP)
		assert.Empty(t, config.Web)
		assert.Empty(t, config.AllowFunnel)
	})

	t.Run("file exists and is valid", func(t *testing.T) {
		initialConfig := &ServeConfig{
			AllowFunnel: map[string]bool{defaultFunnelKey: true},
			Web: map[string]WebConfig{
				"example.com": {Handlers: map[string]HandlerConfig{"/": {Proxy: "http://localhost:8080"}}},
			},
			TCP: map[string]map[string]bool{
				"2222": {"localhost:22": true},
			},
		}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, defaultFunnelKey)
		config, err := commander.readServeConfig()
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, initialConfig.AllowFunnel, config.AllowFunnel)
		assert.Equal(t, initialConfig.Web, config.Web)
		assert.Equal(t, initialConfig.TCP, config.TCP)
	})

	t.Run("file exists but is empty JSON object", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "readtest_emptyjson")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		configPath := filepath.Join(tempDir, "empty-serve-config.json")
		err = os.WriteFile(configPath, []byte("{}"), 0644)
		require.NoError(t, err)

		commander := NewFileTailscaleCommander(configPath, defaultFunnelKey)
		config, err := commander.readServeConfig()
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.NotNil(t, config.AllowFunnel, "AllowFunnel should be an empty map, not nil")
		assert.NotNil(t, config.TCP, "TCP should be an empty map, not nil")
		assert.NotNil(t, config.Web, "Web should be an empty map, not nil")
		assert.Empty(t, config.AllowFunnel)
		assert.Empty(t, config.TCP)
		assert.Empty(t, config.Web)
	})
	
	t.Run("file exists but is invalid JSON", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "readtest_invalidjson")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		configPath := filepath.Join(tempDir, "invalid-serve-config.json")
		err = os.WriteFile(configPath, []byte("this is not json"), 0644)
		require.NoError(t, err)

		commander := NewFileTailscaleCommander(configPath, defaultFunnelKey)
		_, err = commander.readServeConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal serve config data")
	})

	t.Run("file exists with some fields nil or missing", func(t *testing.T) {
		tempDir1, _ := os.MkdirTemp("", "readtest_nilfields1")
		defer os.RemoveAll(tempDir1)
		configPath1 := filepath.Join(tempDir1, "config1.json")
		err := os.WriteFile(configPath1, []byte(`{"AllowFunnel": null, "TCP": {}, "Web": null}`), 0644)
		require.NoError(t, err)

		commander1 := NewFileTailscaleCommander(configPath1, defaultFunnelKey)
		config1, err1 := commander1.readServeConfig()
		require.NoError(t, err1)
		assert.NotNil(t, config1.AllowFunnel, "AllowFunnel should be initialized from null")
		assert.Empty(t, config1.AllowFunnel)
		assert.NotNil(t, config1.TCP, "TCP should be initialized from empty object")
		assert.Empty(t, config1.TCP)
		assert.NotNil(t, config1.Web, "Web should be initialized from null")
		assert.Empty(t, config1.Web)

		tempDir2, _ := os.MkdirTemp("", "readtest_nilfields2")
		defer os.RemoveAll(tempDir2)
		configPath2 := filepath.Join(tempDir2, "config2.json")
		err = os.WriteFile(configPath2, []byte(`{"TCP": {"1234": {"localhost:1234": true}}}`), 0644)
		require.NoError(t, err)
		
		commander2 := NewFileTailscaleCommander(configPath2, defaultFunnelKey)
		config2, err2 := commander2.readServeConfig()
		require.NoError(t, err2)
		assert.NotNil(t, config2.AllowFunnel, "AllowFunnel should be initialized when missing")
		assert.Empty(t, config2.AllowFunnel)
		assert.NotNil(t, config2.TCP)
		assert.NotEmpty(t, config2.TCP)
		assert.NotNil(t, config2.Web, "Web should be initialized when missing")
		assert.Empty(t, config2.Web)
	})
}

func TestFileTailscaleCommander_writeServeConfig(t *testing.T) {
	defaultFunnelKey := "test.example.com:443"

	t.Run("successfully writes config and creates directory if needed", func(t *testing.T) {
		tempBaseDir, err := os.MkdirTemp("", "writetestbase")
		require.NoError(t, err)
		defer os.RemoveAll(tempBaseDir)

		configDir := filepath.Join(tempBaseDir, "newdir")
		configPath := filepath.Join(configDir, "serve-config.json")

		_, statErr := os.Stat(configDir)
		require.True(t, os.IsNotExist(statErr), "Intermediate directory should not exist before write")

		commander := NewFileTailscaleCommander(configPath, defaultFunnelKey)
		configToWrite := &ServeConfig{
			AllowFunnel: map[string]bool{defaultFunnelKey: true},
			Web:         map[string]WebConfig{"example.com": {Handlers: map[string]HandlerConfig{"/": {Proxy: "http://localhost:3000"}}}},
			TCP:         map[string]map[string]bool{"5432": {"localhost:5432": true}},
		}

		err = commander.writeServeConfig(configToWrite)
		require.NoError(t, err)

		dirStat, dirStatErr := os.Stat(configDir)
		assert.NoError(t, dirStatErr, "Directory should have been created")
		assert.True(t, dirStat.IsDir(), "Path created should be a directory")

		data, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)

		var readBackConfig ServeConfig
		unmarshalErr := json.Unmarshal(data, &readBackConfig)
		require.NoError(t, unmarshalErr)
		assert.Equal(t, configToWrite.AllowFunnel, readBackConfig.AllowFunnel)
		assert.Equal(t, configToWrite.Web, readBackConfig.Web)
		assert.Equal(t, configToWrite.TCP, readBackConfig.TCP)
	})

	t.Run("successfully overwrites existing config", func(t *testing.T) {
		initialConfig := &ServeConfig{AllowFunnel: map[string]bool{"oldkey:123": false}}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, defaultFunnelKey)
		newConfig := &ServeConfig{
			AllowFunnel: map[string]bool{defaultFunnelKey: true},
		}
		err := commander.writeServeConfig(newConfig)
		require.NoError(t, err)

		data, readErr := os.ReadFile(configPath)
		require.NoError(t, readErr)
		var readBackConfig ServeConfig
		unmarshalErr := json.Unmarshal(data, &readBackConfig)
		require.NoError(t, unmarshalErr)
		assert.Equal(t, newConfig.AllowFunnel, readBackConfig.AllowFunnel)
		assert.Empty(t, readBackConfig.Web)
		assert.Empty(t, readBackConfig.TCP)
	})
}


func TestFileTailscaleCommander_EnableFunnel(t *testing.T) {
	funnelKey := "enable.example.com:443"

	t.Run("enables funnel when file does not exist", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "enable_nonexistent")
		defer os.RemoveAll(tempDir)
		configPath := filepath.Join(tempDir, "serve-config.json")

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		err := commander.EnableFunnel()
		require.NoError(t, err)

		config, readErr := commander.readServeConfig()
		require.NoError(t, readErr)
		require.NotNil(t, config.AllowFunnel)
		assert.True(t, config.AllowFunnel[funnelKey])
	})

	t.Run("enables funnel when file exists and key is false", func(t *testing.T) {
		initialConfig := &ServeConfig{AllowFunnel: map[string]bool{funnelKey: false, "other:1": true}}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		err := commander.EnableFunnel()
		require.NoError(t, err)

		config, readErr := commander.readServeConfig()
		require.NoError(t, readErr)
		assert.True(t, config.AllowFunnel[funnelKey])
		assert.True(t, config.AllowFunnel["other:1"], "Other keys should be preserved")
	})

	t.Run("enables funnel when file exists and key does not exist", func(t *testing.T) {
		initialConfig := &ServeConfig{AllowFunnel: map[string]bool{"otherkey:443": true}}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		err := commander.EnableFunnel()
		require.NoError(t, err)

		config, readErr := commander.readServeConfig()
		require.NoError(t, readErr)
		assert.True(t, config.AllowFunnel[funnelKey])
		assert.True(t, config.AllowFunnel["otherkey:443"], "Other keys should be preserved")
	})

	t.Run("enable funnel when config is empty json", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "enable_emptyjson")
		defer os.RemoveAll(tempDir)
		configPath := filepath.Join(tempDir, "serve-config.json")
		err := os.WriteFile(configPath, []byte("{}"), 0644)
		require.NoError(t, err)
		
		commander := NewFileTailscaleCommander(configPath, funnelKey)
		err = commander.EnableFunnel()
		require.NoError(t, err)

		config, readErr := commander.readServeConfig()
		require.NoError(t, readErr)
		assert.True(t, config.AllowFunnel[funnelKey])
	})
}

func TestFileTailscaleCommander_DisableFunnel(t *testing.T) {
	funnelKey := "disable.example.com:443"

	t.Run("disables funnel when file does not exist (creates key as false)", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "disable_nonexistent")
		defer os.RemoveAll(tempDir)
		configPath := filepath.Join(tempDir, "serve-config.json")

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		err := commander.DisableFunnel()
		require.NoError(t, err)

		config, readErr := commander.readServeConfig()
		require.NoError(t, readErr)
		require.NotNil(t, config.AllowFunnel)
		assert.False(t, config.AllowFunnel[funnelKey])
	})

	t.Run("disables funnel when file exists and key is true", func(t *testing.T) {
		initialConfig := &ServeConfig{AllowFunnel: map[string]bool{funnelKey: true, "other:1": true}}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		err := commander.DisableFunnel()
		require.NoError(t, err)

		config, readErr := commander.readServeConfig()
		require.NoError(t, readErr)
		assert.False(t, config.AllowFunnel[funnelKey])
		assert.True(t, config.AllowFunnel["other:1"], "Other keys should be preserved")
	})

	t.Run("disables funnel when file exists and key does not exist (creates key as false)", func(t *testing.T) {
		initialConfig := &ServeConfig{AllowFunnel: map[string]bool{"otherkey:443": true}}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		err := commander.DisableFunnel()
		require.NoError(t, err)

		config, readErr := commander.readServeConfig()
		require.NoError(t, readErr)
		assert.False(t, config.AllowFunnel[funnelKey])
		assert.True(t, config.AllowFunnel["otherkey:443"], "Other keys should be preserved")
	})
}

func TestFileTailscaleCommander_GetActualFunnelStatus(t *testing.T) {
	funnelKey := "status.example.com:443"

	t.Run("status is false when file does not exist", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "status_nonexistent")
		defer os.RemoveAll(tempDir)
		configPath := filepath.Join(tempDir, "serve-config.json")
		
		commander := NewFileTailscaleCommander(configPath, funnelKey)
		
		status, err := commander.GetActualFunnelStatus()
		require.NoError(t, err) 
		assert.False(t, status)
	})

	t.Run("status is true when key is true in file", func(t *testing.T) {
		initialConfig := &ServeConfig{AllowFunnel: map[string]bool{funnelKey: true}}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		status, err := commander.GetActualFunnelStatus()
		require.NoError(t, err)
		assert.True(t, status)
	})

	t.Run("status is false when key is false in file", func(t *testing.T) {
		initialConfig := &ServeConfig{AllowFunnel: map[string]bool{funnelKey: false}}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		status, err := commander.GetActualFunnelStatus()
		require.NoError(t, err)
		assert.False(t, status)
	})

	t.Run("status is false when key does not exist in file", func(t *testing.T) {
		initialConfig := &ServeConfig{AllowFunnel: map[string]bool{"otherkey:443": true}}
		configPath, cleanup := createTempConfigFile(t, initialConfig)
		defer cleanup()

		commander := NewFileTailscaleCommander(configPath, funnelKey)
		status, err := commander.GetActualFunnelStatus()
		require.NoError(t, err)
		assert.False(t, status)
	})

	t.Run("error when config file is unreadable (e.g. a directory)", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "status_unreadable_dir")
		defer os.RemoveAll(tempDir)
		
		unreadableConfigPath := filepath.Join(tempDir, "serve-config.json")
		err := os.Mkdir(unreadableConfigPath, 0755) // Create a directory at the config path
		require.NoError(t, err)

		commander := NewFileTailscaleCommander(unreadableConfigPath, funnelKey)
		_, err = commander.GetActualFunnelStatus()
		require.Error(t, err, "Expected an error when trying to read a directory as config file")
		assert.Contains(t, err.Error(), "failed to read serve config for status check")
	})
}
