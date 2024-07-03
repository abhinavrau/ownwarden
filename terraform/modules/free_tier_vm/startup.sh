#!/bin/bash
# Install docker-compose
VERSION="1.27.4"

echo "* Add an alias for docker-compose to the shell configuration file ..."
echo alias docker-compose="'"'docker run --rm \
-v /var/run/docker.sock:/var/run/docker.sock \
-v "$PWD:$PWD" \
-w="$PWD" \
docker/compose:'"$VERSION"''"'" >> ~/.bashrc

echo "* Pull container image for docker-compose ..."
docker pull docker/compose:$VERSION
echo "* Done"


# Set up automatic reboot on OS update. This only works on GCP ContainerOS

# Local timezone - use the TZ database name from https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
# e.g., Etc/UTC, America/New_York, etc
TZ=${TZ}

# Local time to schedule reboot. This is 2am
TIME=03:00


SCHEDULED=$(eval "date -d 'TZ=\"$TZ\" $TIME' +%H:%M")

sudo update_engine_client --block_until_reboot_is_needed
sudo shutdown -r $SCHEDULED
