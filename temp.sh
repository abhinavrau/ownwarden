#!/bin/bash  
# This script is run as the user 'composer' and is used to check out the latest code
region="us-east1"
project_id="abhi-personal-373819"
artifact_registry_repo_id="ownwarden"
# Set the timezone to UTC
# Check out the latest code
echo "Step 1: Checking out the latest code..."
echo "--------------------------------------------------"

rm -Rf /home/composer/ownwarden 
git clone https://github.com/abhinavrau/ownwarden.git /home/composer/ownwarden 
cd /home/composer/ownwarden

# Link the .env file we created above to the docker-compose directory
cp /home/composer/.env /home/composer/ownwarden/.env

# Build the docker image and push to artifact registry
echo "Step 2: Building Docker image..."
docker build -t "caddy_tailscale:latest" -f "images/caddy_tailscale/Dockerfile" "images/caddy_tailscale"
if [ $? -ne 0 ]; then
  echo "Error: Docker build failed."
  exit 1
fi
echo "Docker image built successfully: caddy_tailscale:latest"
echo "--------------------------------------------------"

echo "Step 3: Tagging image for Artifact Registry..."
docker tag "caddy_tailscale:latest" "${region}-docker.pkg.dev/${${project_id}}/${artifact_registry_repo_id}/caddy_tailscale:latest"

if [ $? -ne 0 ]; then
  echo "Error: Docker tag failed."
  exit 1
fi
echo "Image tagged successfully: ${region}-docker.pkg.dev/${${project_id}}/${artifact_registry_repo_id}/caddy_tailscale:latest"
echo "--------------------------------------------------"
echo "Step 4: Pushing image to Artifact Registry..."

docker push "${region}-docker.pkg.dev/${${project_id}}/${artifact_registry_repo_id}/caddy_tailscale:latest"
if [ $? -ne 0 ]; then
  echo "Error: Docker push failed. Ensure you are authenticated (e.g., 'gcloud auth configure-docker ${region}-docker.pkg.dev')."
  exit 1
fi
echo "Image pushed successfully to Artifact Registry!"
echo "Full image path: ${region}-docker.pkg.dev/${project_id}/${artifact_registry_repo_id}/caddy_tailscale:latest"
echo "--------------------------------------------------"

echo "Pull docker image for docker compose ..."
docker pull docker:v2.36.1
if [ $? -ne 0 ]; then
  echo "Error: Docker pull failed."
  exit 1
fi
echo "Docker image pulled successfully: docker:v2.36.1"
echo "--------------------------------------------------"
TZ=${TZ}

# Local time to schedule reboot. This is 2am
TIME=03:00
SCHEDULED=$(eval "date -d 'TZ=\"$TZ\" $TIME' +%H:%M")
#update_engine_client --block_until_reboot_is_needed
shutdown -r $SCHEDULED
if [ $? -ne 0 ]; then
  echo "Error: Scheduling Shutdown  failed."
  exit 1
fi