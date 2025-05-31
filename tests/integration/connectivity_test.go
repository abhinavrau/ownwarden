package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime" // Added for determining file path
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"tailscale.com/tsnet"
)

const (
	// Environment variables for configuration
	envTailscaleAuthKey            = "TAILSCALE_AUTH_KEY_INTEGRATION_TEST" // Specific key for this test client
	envTailscaleHostname           = "TAILSCALE_HOSTNAME"                  // Target Vaultwarden service hostname on Tailnet
	envTailscaleDomain             = "TAILSCALE_DOMAIN"                    // Tailscale domain
	envTestClientHostname          = "TAILSCALE_TEST_CLIENT_HOSTNAME"      // Optional: Hostname for this test client node
	envVaultwardenUserEmail        = "VAULTWARDEN_USER_EMAIL_INTEGRATION_TEST"
	envVaultwardenPassword         = "VAULTWARDEN_PASSWORD_INTEGRATION_TEST"


	defaultTestClientHostname = "ownwarden-integration-go-client"
	// dockerComposeTestFile     = "docker-compose-test.yml" // Old: specific test compose file
	dockerComposeMainFile     = "docker-compose.yml"      // New: main project compose file
	envFileName               = ".env"
)

// TestMain manages the setup and teardown of Docker Compose services.
func TestMain(m *testing.M) {
	// Determine the actual project root directory.
	// Assumes this test file is in a subdirectory like 'tests/integration'
	// and the project root is two levels up.
	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("Error: Unable to determine current test file path using runtime.Caller")
		os.Exit(1)
	}
	currentTestDir := filepath.Dir(currentFilePath)                 // e.g., /workspaces/ownwarden/tests/integration
	projectRoot := filepath.Clean(filepath.Join(currentTestDir, "..", "..")) // e.g., /workspaces/ownwarden

	// Load .env files:
	// 1. Project root .env (e.g., /workspaces/ownwarden/.env)
	// 2. Test directory specific .env (e.g., /workspaces/ownwarden/tests/integration/.env)
	// godotenv.Load loads them in order, and variables in later files do not override earlier ones
	// if already set by the system or a previous file.
	// For our desired precedence (test .env > project .env > system env),
	// we should list them so that test-specific can "conceptually" override project-specific for godotenv's first-come-first-serve.
	// However, godotenv standard behavior is: first file loaded wins for a given key if not already in ENV.
	// To achieve test .env overriding project .env, load test .env first, then project .env.
	// But the prompt's original logic was project root then test dir, implying test dir vars are more specific additions.
	// Let's stick to the original intent: load project root, then test-specific. Shell ENV vars always win.
	envPathProjectRoot := filepath.Join(projectRoot, envFileName)
	envPathTestDir := filepath.Join(currentTestDir, envFileName) // Corrected path for test's .env

	// Attempt to load .env files. godotenv.Load doesn't error if files don't exist.
	// It loads in the order given. If a variable is in multiple files, the first one encountered is used.
	// Shell environment variables take precedence over any .env files.
	if loadErr := godotenv.Load(envPathProjectRoot, envPathTestDir); loadErr != nil {
		// This typically means an actual error reading an existing file, not "file not found".
		fmt.Printf("Warning: error during godotenv.Load (some .env files might be missing or unreadable): %v\n", loadErr)
	}

	// Use the main docker-compose.yml from the project root.
	composeFilePath := filepath.Join(projectRoot, dockerComposeMainFile) // e.g., /workspaces/ownwarden/docker-compose.yml
	composeFileDir := projectRoot                                        // Docker compose commands should run from project root

	if _, statErr := os.Stat(composeFilePath); os.IsNotExist(statErr) {
		fmt.Printf("Error: Docker Compose file %s not found in determined project root %s. Please ensure the file exists and path logic is correct.\n", dockerComposeMainFile, projectRoot)
		os.Exit(1)
	}

	fmt.Printf("Using Docker Compose file: %s (Project Root: %s, Test Dir: %s)\n", composeFilePath, projectRoot, currentTestDir)

	// Define a cleanup function that will be deferred.
	cleanup := func() {
		fmt.Println("Cleanup function invoked.") // Added log
		fmt.Println("Tearing down Docker Compose services (main project)...")
		cmdDown := exec.Command("docker", "compose", "-f", composeFilePath, "down", "--volumes", "--remove-orphans")
		cmdDown.Dir = composeFileDir // Run from project root
		cmdDown.Stdout = os.Stdout
		cmdDown.Stderr = os.Stderr
		fmt.Println("Executing 'docker compose down'...") // Added log
		if err := cmdDown.Run(); err != nil {
			fmt.Printf("Error during 'docker compose down': %v\n", err) // Clarified error source
		} else {
			fmt.Println("Docker Compose services torn down successfully.") // More explicit success message
		}
		fmt.Println("Cleanup function finished.") // Added log
	}
	

	// Setup: Start Docker Compose services
	fmt.Println("Setting up Docker Compose services (main project) for integration test...")
	cmdUp := exec.Command("docker", "compose", "-f", composeFilePath, "up", "-d", "--wait") 
	cmdUp.Dir = composeFileDir // Run from project root
	cmdUp.Stdout = os.Stdout
	cmdUp.Stderr = os.Stderr
	if err := cmdUp.Run(); err != nil {
		fmt.Printf("Error starting Docker Compose services: %v\n", err)
		cmdLogs := exec.Command("docker", "compose", "-f", composeFilePath, "logs")
		cmdLogs.Dir = composeFileDir // Run from project root
		cmdLogs.Stdout = os.Stdout
		cmdLogs.Stderr = os.Stderr
		cmdLogs.Run()
		os.Exit(1)
	}
	fmt.Println("Docker Compose services started.")

	// Run the tests
	exitCode := m.Run()
	fmt.Printf("TestMain: m.Run() completed with exitCode %d. Performing cleanup before os.Exit().\n", exitCode) // Updated diagnostic log

	cleanup() // Explicitly call cleanup here

	os.Exit(exitCode)
}

func TestVaultwardenConnectivityViaTailscale(t *testing.T) {
	authKey := os.Getenv(envTailscaleAuthKey)
	targetHostname := os.Getenv(envTailscaleHostname)
	tsDomain := os.Getenv(envTailscaleDomain)
	clientHostname := os.Getenv(envTestClientHostname)

	if clientHostname == "" {
		clientHostname = defaultTestClientHostname
	}

	assert.NotEmpty(t, authKey, fmt.Sprintf("%s environment variable must be set", envTailscaleAuthKey))
	assert.NotEmpty(t, targetHostname, fmt.Sprintf("%s environment variable must be set", envTailscaleHostname))
	assert.NotEmpty(t, tsDomain, fmt.Sprintf("%s environment variable must be set", envTailscaleDomain))

	// Construct the full Vaultwarden URL. Assuming HTTPS and standard port.
	// The targetHostname from env should be just the machine name, e.g., "vaultwarden-server"
	// The tsDomain should be the tailnet name, e.g., "your-tailnet.ts.net"
	// So, vaultwardenURL becomes "https://vaultwarden-server.your-tailnet.ts.net"
	// However, tsnet.Dial typically works with "hostname:port", and the hostname part
	// can be the simple Tailscale machine name if DNS is configured correctly within Tailscale.
	// The http.Client.Get will need the full URL including the scheme and domain.

	vaultwardenServiceAddress := targetHostname // This is the address tsnet will dial, e.g., "vaultwarden-server:443"
	if !strings.Contains(vaultwardenServiceAddress, ":") {
		vaultwardenServiceAddress = fmt.Sprintf("%s:443", targetHostname) // Default to HTTPS port
	}

	// The URL for the HTTP client needs to be fully qualified.
	// TAILSCALE_HOSTNAME is the machine name (e.g., "my-server")
	// TAILSCALE_DOMAIN is the full tailnet name (e.g., "org-name.ts.net")
	// So, the FQDN is "my-server.org-name.ts.net"
	vaultwardenFQDN := fmt.Sprintf("%s.%s", strings.TrimSuffix(targetHostname, "."), strings.TrimPrefix(tsDomain, "."))
	vaultwardenURL := fmt.Sprintf("https://%s", vaultwardenFQDN)

	t.Logf("Attempting to connect to Vaultwarden at %s (service address for dial: %s) via Tailscale as %s", vaultwardenURL, vaultwardenServiceAddress, clientHostname)

	srv := &tsnet.Server{
		Hostname:  clientHostname,
		AuthKey:   authKey,
		Ephemeral: true,
		// Logf:      t.Logf, // Optional: route tsnet logs to test logs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute) // Overall timeout for Tailscale setup + HTTP check
	defer cancel()

	status, err := srv.Up(ctx)
	if err != nil {
		t.Fatalf("Failed to bring up tsnet.Server: %v. Status: %+v", err, status)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			t.Logf("Error closing tsnet.Server: %v", err)
		}
		t.Log("tsnet.Server closed.")
	}()

	assert.NotEmpty(t, status.TailscaleIPs, "tsnet.Server should have at least one Tailscale IP")
	t.Logf("tsnet.Server UP. Tailscale IPs: %v. Self: %v", status.TailscaleIPs, status.Self)

	// Wait a bit for Tailscale network to settle, especially DNS propagation if relying on FQDNs for srv.Dial
	// Although srv.Dial with just hostname:port should use Tailscale's direct routing.
	time.Sleep(5 * time.Second)

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// addr will be what http.Client tries to dial, e.g., "vaultwarden-server.your-tailnet.ts.net:443"
				// We want srv.Dial to use the simple Tailscale hostname and port.
				t.Logf("DialContext: network=%s, original addr=%s, attempting to dial serviceAddress=%s", network, addr, vaultwardenServiceAddress)
				return srv.Dial(ctx, network, vaultwardenServiceAddress)
			},
		},
		Timeout: 30 * time.Second, // Timeout for the HTTP request itself
	}

	req, err := http.NewRequestWithContext(ctx, "GET", vaultwardenURL, nil)
	assert.NoError(t, err, "Failed to create HTTP request")

	t.Logf("Performing HTTP GET to %s", vaultwardenURL)
	resp, err := httpClient.Do(req)
	if err != nil {
		// Check if context timed out
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("HTTP GET to %s failed due to context deadline exceeded: %v", vaultwardenURL, err)
		}
		t.Fatalf("HTTP GET to %s failed: %v", vaultwardenURL, err)
	}
	defer resp.Body.Close()

	t.Logf("Received response: Status=%s, Code=%d, ContentLength=%d", resp.Status, resp.StatusCode, resp.ContentLength)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected HTTP 200 OK")
	// Add more assertions as needed, e.g., checking body content, headers.
	// For a simple connectivity test, 200 OK is often sufficient.
	// Example:
	// bodyBytes, err := io.ReadAll(resp.Body)
	// assert.NoError(t, err)
	// assert.Contains(t, string(bodyBytes), "Vaultwarden", "Response body should contain Vaultwarden")

	t.Log("Vaultwarden connectivity test successful.")
}

