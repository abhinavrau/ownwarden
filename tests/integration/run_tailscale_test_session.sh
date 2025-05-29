#!/bin/bash

# Script to verify connectivity to a Vaultwarden URL via Tailscale

# --- Configuration ---
# Load environment variables from .env file if it exists
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)
ENV_FILE="$SCRIPT_DIR/.env"

if [ -f "$ENV_FILE" ]; then
    echo "Loading environment variables from $ENV_FILE"
    # Use set -a to export all variables read from .env
    # Use set +a to stop exporting after sourcing
    set -a
    # shellcheck source=/dev/null
    source "$ENV_FILE"
    source ../../.env
    set +a
else
    echo "Warning: .env file not found at $ENV_FILE. Please ensure TAILSCALE_AUTH_KEY, TAILSCALE_HOSTNAME, and TAILSCALE_DOMAIN are set in your environment or create the .env file."
fi

# TAILSCALE_AUTH_KEY, TAILSCALE_HOSTNAME, and TAILSCALE_DOMAIN should now be available from .env or existing environment
# Construct VAULTWARDEN_URL
if [ -n "$TAILSCALE_HOSTNAME" ] && [ -n "$TAILSCALE_DOMAIN" ]; then
    VAULTWARDEN_URL="https://$(echo "$TAILSCALE_HOSTNAME" | sed 's/\.$//').$(echo "$TAILSCALE_DOMAIN" | sed 's/^\.//')"
    # Ensure no double dots if either variable accidentally contains leading/trailing dots
    VAULTWARDEN_URL=$(echo "$VAULTWARDEN_URL" | sed 's/\.\./\./g')
else
    VAULTWARDEN_URL="" # Will trigger error if not properly set
fi


# Optional: If using an OAuth client secret, you need to specify tags.
# This can also be set in the .env file as TS_EXTRA_ARGS
# Make sure the tag (e.g., tag:container-test) is defined in your Tailscale ACLs.
# Example for .env: TS_EXTRA_ARGS="--advertise-tags=tag:container-test --accept-dns=false"
# TS_EXTRA_ARGS is used as is from environment or .env, default to empty if not set.
TS_EXTRA_ARGS="${TS_EXTRA_ARGS:-}"


# --- Script Parameters ---
TEST_CONTAINER_NAME="ownwarden-integration-test" # As defined in docker-compose-test.yml
# TAILSCALE_IMAGE is defined in docker-compose-test.yml
# STATE_VOLUME_NAME is managed by docker-compose-test.yml

# --- Helper Functions ---
cleanup() {
    echo "" # Newline for better formatting
    echo "Cleaning up..."

    echo "Stopping and removing Tailscale test container (from docker-compose-test.yml)..."
    if [ -f "$SCRIPT_DIR/docker-compose-test.yml" ]; then
        docker compose -f "$SCRIPT_DIR/docker-compose-test.yml" down --remove-orphans --volumes > /dev/null 2>&1
        echo "Tailscale test Docker Compose services shut down."
    else
        echo "Warning: docker-compose-test.yml not found in $SCRIPT_DIR, skipping test compose down."
    fi

    
    echo "Cleanup complete."
}

# Trap EXIT signal to ensure cleanup runs
trap cleanup EXIT

# --- Main Script ---


echo "" # Newline for better formatting
echo "Attempting to construct Vaultwarden URL from TAILSCALE_HOSTNAME and TAILSCALE_DOMAIN..."
echo "TAILSCALE_HOSTNAME: $TAILSCALE_HOSTNAME"
echo "TAILSCALE_DOMAIN: $TAILSCALE_DOMAIN"
echo "Constructed VAULTWARDEN_URL for test: $VAULTWARDEN_URL"
echo "" # Newline for better formatting
echo "Starting Tailscale connectivity test using $TEST_CONTAINER_NAME..."

# Validate inputs (now loaded from .env or environment)
if [ -z "$TAILSCALE_AUTH_KEY" ]; then # This is used by docker-compose-test.yml
    echo "Error: TAILSCALE_AUTH_KEY is not set. Please ensure it's in $ENV_FILE or your environment."
    exit 1
fi

if [ -z "$TAILSCALE_HOSTNAME" ]; then
    echo "Error: TAILSCALE_HOSTNAME is not set. Please ensure it's in $ENV_FILE or your environment."
    exit 1
fi

if [ -z "$TAILSCALE_DOMAIN" ]; then
    echo "Error: TAILSCALE_DOMAIN is not set. Please ensure it's in $ENV_FILE or your environment."
    exit 1
fi

if [ -z "$VAULTWARDEN_URL" ]; then
    echo "Error: VAULTWARDEN_URL could not be constructed. Ensure TAILSCALE_HOSTNAME and TAILSCALE_DOMAIN are correctly set."
    exit 1
fi

# 1. Start Tailscale test container using docker-compose-test.yml
echo "" # Newline for better formatting
echo "Starting Tailscale test container using docker-compose-test.yml..."
if [ ! -f "$SCRIPT_DIR/docker-compose-test.yml" ]; then
    echo "Error: docker-compose-test.yml not found in $SCRIPT_DIR."
    exit 1
fi
docker compose -f "$SCRIPT_DIR/docker-compose-test.yml" up -d 
if [ $? -ne 0 ]; then
    echo "Error: Failed to start Tailscale test container using docker-compose-test.yml."
    docker compose -f "$SCRIPT_DIR/docker-compose-test.yml" logs
    exit 1
fi

# 2. Wait for Tailscale to connect
MAX_WAIT_SECONDS=90
WAIT_INTERVAL_SECONDS=5
ELAPSED_SECONDS=0
EXPECTED_TS_HOSTNAME="${TAILSCALE_HOSTNAME}-integration-test" # From docker-compose-test.yml

# Give tailscale some initial time to start up before polling
sleep 15 # Increased sleep for compose startup

# 2a. Wait for the main TAILSCALE_HOSTNAME to be active
echo ""
echo "Checking for main Tailscale host ($TAILSCALE_HOSTNAME) to be active via $TEST_CONTAINER_NAME..."
MAX_WAIT_MAIN_HOST_SECONDS=60
ELAPSED_SECONDS_MAIN_HOST=0
MAIN_HOST_FOUND=false

while [ $ELAPSED_SECONDS_MAIN_HOST -lt $MAX_WAIT_MAIN_HOST_SECONDS ]; do
    echo "Checking main host $TAILSCALE_HOSTNAME status (elapsed: ${ELAPSED_SECONDS_MAIN_HOST}s)..."
    if docker exec "$TEST_CONTAINER_NAME" tailscale status --active 2>/dev/null | grep -q "$TAILSCALE_HOSTNAME"; then
        echo "Main Tailscale host $TAILSCALE_HOSTNAME found and active."
        MAIN_HOST_FOUND=true
        break
    else
        echo "Main Tailscale host $TAILSCALE_HOSTNAME not yet active or not found. Retrying..."
    fi
    sleep $WAIT_INTERVAL_SECONDS
    ELAPSED_SECONDS_MAIN_HOST=$((ELAPSED_SECONDS_MAIN_HOST + WAIT_INTERVAL_SECONDS))
done

if [ "$MAIN_HOST_FOUND" = false ]; then
    echo "Error: Main Tailscale host $TAILSCALE_HOSTNAME did not become active within $MAX_WAIT_MAIN_HOST_SECONDS seconds."
    echo "--- Current Tailscale status from $TEST_CONTAINER_NAME ---"
    docker exec "$TEST_CONTAINER_NAME" tailscale status 2>/dev/null || echo "Failed to get Tailscale status from $TEST_CONTAINER_NAME."
    echo "---------------------------------------------"
    exit 1
fi

# 2b. Wait for the test container's Tailscale to connect and be active
echo ""
echo "Checking for test container's Tailscale hostname ($EXPECTED_TS_HOSTNAME) to be active..."
while [ $ELAPSED_SECONDS -lt $MAX_WAIT_SECONDS ]; do
    echo "Checking Tailscale status for $EXPECTED_TS_HOSTNAME in $TEST_CONTAINER_NAME (elapsed: ${ELAPSED_SECONDS}s)..."
    
    # Check if the container is running
    if ! docker inspect "$TEST_CONTAINER_NAME" &> /dev/null; then
        echo "Error: Tailscale test container $TEST_CONTAINER_NAME is not running. It might have crashed."
        docker compose -f "$SCRIPT_DIR/docker-compose-test.yml" logs
        exit 1
    fi

    TS_IP=$(docker exec "$TEST_CONTAINER_NAME" tailscale ip -4 2>/dev/null)
    if [ -n "$TS_IP" ]; then
        echo "Tailscale connected in $TEST_CONTAINER_NAME. Container IP: $TS_IP"
        # Check if the expected hostname is present and active in tailscale status
        if docker exec "$TEST_CONTAINER_NAME" tailscale status --active | grep -q "$EXPECTED_TS_HOSTNAME"; then
             echo "Tailscale status confirmed active for $EXPECTED_TS_HOSTNAME."
             break
        else
            echo "Tailscale IP found, but status for $EXPECTED_TS_HOSTNAME not yet fully active or hostname mismatch. Retrying..."
            docker exec "$TEST_CONTAINER_NAME" tailscale status # Print status for debugging
        fi
    else
        echo "Tailscale not yet reporting an IP in $TEST_CONTAINER_NAME."
    fi
    
    sleep $WAIT_INTERVAL_SECONDS
    ELAPSED_SECONDS=$((ELAPSED_SECONDS + WAIT_INTERVAL_SECONDS))
    
    if [ $ELAPSED_SECONDS -ge $MAX_WAIT_SECONDS ]; then
        echo "Error: Tailscale in $TEST_CONTAINER_NAME did not connect or become fully active within $MAX_WAIT_SECONDS seconds."
        echo "--- Current Tailscale logs for $TEST_CONTAINER_NAME (from compose) ---"
        docker compose -f "$SCRIPT_DIR/docker-compose-test.yml" logs tailscale # Assuming service name is 'tailscale'
        echo "--- Current Tailscale status for $TEST_CONTAINER_NAME ---"
        docker exec "$TEST_CONTAINER_NAME" tailscale status 2>/dev/null || echo "Failed to get Tailscale status from $TEST_CONTAINER_NAME."
        echo "---------------------------------------------"
        exit 1
    fi
done

# 3. Install curl in the Tailscale container
echo "" # Newline for better formatting
echo "Installing curl in $TEST_CONTAINER_NAME..."
docker exec "$TEST_CONTAINER_NAME" sh -c "apk update && apk add curl"
if [ $? -ne 0 ]; then
    echo "Error: Failed to install curl in $TEST_CONTAINER_NAME."
    docker exec "$TEST_CONTAINER_NAME" sh -c "apk update" # Attempt update again to see logs
    exit 1
fi
echo "curl installed successfully in $TEST_CONTAINER_NAME."

# 4. Curl the Vaultwarden URL
echo "" # Newline for better formatting
echo "Attempting to curl $TAILSCALE_HOSTNAME from within the Tailscale test container ($TEST_CONTAINER_NAME)..."
# Using -I to get headers. Add -s for silent, -L for redirects.
# Increased connect-timeout to 15 seconds.
docker exec "$TEST_CONTAINER_NAME" curl --verbose --show-error -L --connect-timeout 30 "http://$TAILSCALE_HOSTNAME"

CURL_EXIT_CODE=$?

if [ $CURL_EXIT_CODE -eq 0 ]; then
    echo "Curl command executed successfully to http://$TAILSCALE_HOSTNAME."
else
    echo "Error: Curl command failed with exit code $CURL_EXIT_CODE."
    echo "Check the verbose output above for details."
    exit 1
fi

echo "" # Newline for better formatting
echo "Attempting to curl TLS endpoint $VAULTWARDEN_URL from within the Tailscale test container ($TEST_CONTAINER_NAME)... This takes a few seconds..."
docker exec "$TEST_CONTAINER_NAME" curl --verbose --show-error -L --connect-timeout 30 "$VAULTWARDEN_URL"
CURL_EXIT_CODE=$?

if [ $CURL_EXIT_CODE -eq 0 ]; then
    echo "" # Newline
    echo "Successfully connected to http://$TAILSCALE_HOSTNAME and retrieved content/headers from $TEST_CONTAINER_NAME."
    echo "To see just headers, you could use: curl -I -s -L --connect-timeout 30 \"$VAULTWARDEN_URL\""
    echo "To see full content silently, you could use: curl -s -L --connect-timeout 30 \"$VAULTWARDEN_URL\""
else
    echo "" # Newline
    echo "Error: Failed to connect to $VAULTWARDEN_URL from $TEST_CONTAINER_NAME (curl exit code: $CURL_EXIT_CODE)."
    echo "Check the verbose curl output above for details."
    # Additional diagnostics
    echo "--- DNS resolution check from $TEST_CONTAINER_NAME for $(echo $VAULTWARDEN_URL | awk -F/ '{print $3}' | cut -d: -f1) ---"
    docker exec "$TEST_CONTAINER_NAME" nslookup "$(echo "$VAULTWARDEN_URL" | awk -F/ '{print $3}' | cut -d: -f1)"
    echo "---------------------------------------------"
    exit 1
fi

echo "" # Newline
echo "Connectivity test successful using $TEST_CONTAINER_NAME."
# Cleanup will be handled by the trap EXIT

exit 0
