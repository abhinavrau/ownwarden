# Stage 1: Use the standard Caddy image as a source for the Caddy binary.
# This avoids rebuilding Caddy if you don't need custom plugins.
# The caddy:2.8.4 image already contains a pre-built Caddy binary.
FROM caddy:2.8.4 AS caddy_binary_source

# Stage 2: Final image using Tailscale base
FROM tailscale/tailscale:stable

# Create necessary directories. This layer is likely to be cached.
# Grouping these operations can sometimes help with layer management.
RUN set -eux; \
    mkdir -p \
        /config/tailscale \
        /config/caddy \
        /data/caddy \
        /etc/caddy \
        /usr/share/caddy

# Set environment variables for Caddy and other tools
ENV XDG_CONFIG_HOME /config
ENV XDG_DATA_HOME /data

# Copy the pre-built Caddy binary from the caddy_binary_source stage.
# This is much faster than building it from scratch.
COPY --from=caddy_binary_source /usr/bin/caddy /usr/bin/caddy

# Copy the start script and make it executable.
# This layer will only be rebuilt if start.sh changes.
COPY start.sh /usr/bin/start.sh
RUN chmod +x /usr/bin/start.sh

# Set the working directory for the application
WORKDIR /srv

# Set the default command to run when the container starts.
# Using JSON array form is best practice and calls /usr/bin/start.sh directly.
CMD ["/usr/bin/start.sh"]
