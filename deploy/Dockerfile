# syntax=docker/dockerfile:1.7

############################
# Build stage
############################
FROM golang:1.22-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG BUILD_TIME=unknown

# Static, CGO-free (modernc.org/sqlite is pure Go).
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
RUN go build -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" -o /out/d2ip ./cmd/d2ip

############################
# Runtime stage
############################
FROM alpine:3.19

# nftables for the routing agent's preferred backend; iproute2 for the fallback;
# ca-certificates for HTTPS dlc.dat fetch; tini for proper PID 1 signal handling.
RUN apk add --no-cache nftables iproute2 ca-certificates tini libcap

RUN addgroup -S d2ip && adduser -S -G d2ip d2ip

WORKDIR /app
COPY --from=build /out/d2ip /usr/local/bin/d2ip
COPY deploy/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# State + output volumes.
RUN mkdir -p /var/lib/d2ip /var/lib/d2ip/out && chown -R d2ip:d2ip /var/lib/d2ip
VOLUME ["/var/lib/d2ip"]

# We DO NOT switch to USER d2ip by default: the routing agent needs CAP_NET_ADMIN.
# Operators who disable routing should run with `--user d2ip`.

EXPOSE 9099

ENV D2IP_LISTEN=":9099" \
    D2IP_CACHE_DB_PATH="/var/lib/d2ip/cache.db" \
    D2IP_SOURCE_CACHE_PATH="/var/lib/d2ip/dlc.dat" \
    D2IP_EXPORT_DIR="/var/lib/d2ip/out" \
    D2IP_ROUTING_STATE_PATH="/var/lib/d2ip/state.json"

ENTRYPOINT ["/sbin/tini","--","/usr/local/bin/entrypoint.sh"]
CMD ["d2ip","serve"]
