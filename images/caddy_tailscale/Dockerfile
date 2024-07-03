
FROM caddy:2.6-builder AS builder

RUN xcaddy build

FROM tailscale/tailscale:stable

RUN set -eux; \
        mkdir -p \
                /config/tailscale \
                /config/caddy \
                /data/caddy \
                /etc/caddy \
                /usr/share/caddy

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY start.sh /usr/bin/start.sh
RUN chmod +x /usr/bin/start.sh

ENV XDG_CONFIG_HOME /config
ENV XDG_DATA_HOME /data

WORKDIR /srv

CMD "start.sh"

