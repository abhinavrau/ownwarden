#!/bin/bash
set -e

# Source environment variables from .env file if it exists
ENV_FILE="$(dirname "$0")/.env"
if [ -f "$ENV_FILE" ]; then
  echo "Loading environment variables from $ENV_FILE"
  set -a # automatically export all variables
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
else
  echo "Warning: $ENV_FILE not found. Proceeding with existing environment variables."
fi

# Script to build the caddy-tailscale Docker image and push it to Google Artifact Registry.
#
# Required environment variables:
#   GOOGLE_PROJECT_ID: Your Google Cloud Project ID.
#   GOOGLE_REGION: The Google Cloud region where the Artifact Registry is located (e.g., us-central1).
#
# Optional environment variables:
#   IMAGE_NAME: Name for the Docker image (default: caddy-tailscale).
#   IMAGE_TAG: Tag for the Docker image (default: latest).
#   DOCKERFILE_PATH: Path to the Dockerfile (default: images/caddy_tailscale/Dockerfile).
#   BUILD_CONTEXT: Docker build context path (default: images/caddy_tailscale/).
#   ARTIFACT_REGISTRY_REPO_ID: The Artifact Registry repository ID (default: ownwarden).

# --- Configuration ---
: "${GOOGLE_PROJECT_ID:?GOOGLE_PROJECT_ID environment variable is not set. Please set it to your Google Cloud Project ID.}"
: "${GOOGLE_REGION:?GOOGLE_REGION environment variable is not set. Please set it to the region of your Artifact Registry (e.g., us-central1).}"

LOCAL_IMAGE_NAME="${IMAGE_NAME:-caddy_tailscale}"
LOCAL_IMAGE_TAG="${IMAGE_TAG:-latest}"
DOCKERFILE_FULL_PATH="${DOCKERFILE_PATH:-images/caddy_tailscale/Dockerfile}"
DOCKER_BUILD_CONTEXT="${BUILD_CONTEXT:-images/caddy_tailscale/}"
ARTIFACT_REGISTRY_REPO_ID="${ARTIFACT_REGISTRY_REPO_ID:-ownwarden}"

# Construct the full Artifact Registry image path
# The format is LOCATION-docker.pkg.dev/PROJECT_ID/REPOSITORY_ID/IMAGE_NAME:TAG
ARTIFACT_REGISTRY_IMAGE_PATH="${GOOGLE_REGION}-docker.pkg.dev/${GOOGLE_PROJECT_ID}/${ARTIFACT_REGISTRY_REPO_ID}/${LOCAL_IMAGE_NAME}:${LOCAL_IMAGE_TAG}"

# --- Script Logic ---
echo "Starting Docker image build and push process..."
echo "--------------------------------------------------"
echo "Configuration:"
echo "  Google Project ID: ${GOOGLE_PROJECT_ID}"
echo "  Google Region: ${GOOGLE_REGION}"
echo "  Local Image Name: ${LOCAL_IMAGE_NAME}"
echo "  Local Image Tag: ${LOCAL_IMAGE_TAG}"
echo "  Dockerfile Path: ${DOCKERFILE_FULL_PATH}"
echo "  Build Context: ${DOCKER_BUILD_CONTEXT}"
echo "  Artifact Registry Repo ID: ${ARTIFACT_REGISTRY_REPO_ID}"
echo "  Target Artifact Registry Image: ${ARTIFACT_REGISTRY_IMAGE_PATH}"
echo "--------------------------------------------------"

echo ""
echo "Step 1: Building Docker image..."
docker build -t "${LOCAL_IMAGE_NAME}:${LOCAL_IMAGE_TAG}" -f "${DOCKERFILE_FULL_PATH}" "${DOCKER_BUILD_CONTEXT}"

if [ $? -ne 0 ]; then
  echo "Error: Docker build failed."
  exit 1
fi
echo "Docker image built successfully: ${LOCAL_IMAGE_NAME}:${LOCAL_IMAGE_TAG}"
echo "--------------------------------------------------"

echo ""
echo "Step 2: Tagging image for Artifact Registry..."
docker tag "${LOCAL_IMAGE_NAME}:${LOCAL_IMAGE_TAG}" "${ARTIFACT_REGISTRY_IMAGE_PATH}"

if [ $? -ne 0 ]; then
  echo "Error: Docker tag failed."
  exit 1
fi
echo "Image tagged successfully: ${ARTIFACT_REGISTRY_IMAGE_PATH}"
echo "--------------------------------------------------"

echo ""
echo "Step 3: Pushing image to Artifact Registry..."
echo "Note: Ensure you are authenticated with gcloud and Docker is configured for Artifact Registry."
echo "You might need to run: gcloud auth configure-docker ${region}-docker.pkg.dev"

docker push "${ARTIFACT_REGISTRY_IMAGE_PATH}"

if [ $? -ne 0 ]; then
  echo "Error: Docker push failed. Ensure you are authenticated (e.g., 'gcloud auth configure-docker ${region}-docker.pkg.dev')."
  exit 1
fi
echo "Image pushed successfully to Artifact Registry!"
echo "Full image path: ${ARTIFACT_REGISTRY_IMAGE_PATH}"
echo "--------------------------------------------------"

echo ""
echo "Script completed successfully."
