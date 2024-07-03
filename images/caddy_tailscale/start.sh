#!/bin/ash
trap 'kill -TERM $PID' TERM INT
echo "Starting Tailscale daemon"
# -state=mem: will logout and remove ephemeral node from network immediately after ending.
tailscaled --tun=userspace-networking --statedir=${TAILSCALE_STATE_DIR} --state=${TAILSCALE_STATE_ARG} &
PID=$!
echo "---------Starting Tailscale network----------"
until tailscale up --authkey="${TAILSCALE_AUTH_KEY}" --hostname="${TAILSCALE_HOSTNAME}"; do
    sleep 0.1
done
echo "---------Starting Tailscale proxy----------"
tailscale serve https / http://127.0.0.1:${TS_PORT:-8080}

echo "---------Starting Caddy ----------"
caddy run \
        --config /etc/caddy/Caddyfile \
        --adapter caddyfile

tailscale status
wait ${PID}
wait ${PID}

