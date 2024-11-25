#!/usr/bin/env ash

# Synology DSM has nginx service that uses port 80 and 443 by default. This script will change the ports to 81 and 444 respectively so it does not conflict with Ownwarden's Caddy server running on local LAN.
# This script will also restart nginx and docker-compose services to apply the changes.
# This script is intended to be run on a schedule daily to ensure the ports are always changed back to the desired values.

if grep -e 80 -e 443 /usr/syno/share/nginx/server.mustache /usr/syno/share/nginx/DSM.mustache /usr/syno/share/nginx/WWWService.mustache; then
echo "Values will be changed"
sudo sed -i -e 's/80/81/' -e 's/443/444/' /usr/syno/share/nginx/server.mustache /usr/syno/share/nginx/DSM.mustache /usr/syno/share/nginx/WWWService.mustache && sudo systemctl restart nginx && sudo docker-compose restart -d
else
    echo "Do nothing"
fi