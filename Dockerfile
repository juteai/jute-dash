# syntax=docker/dockerfile:1.7

FROM node:24-bookworm-slim AS web
WORKDIR /src
COPY apps/web/package*.json ./apps/web/
RUN cd apps/web && npm ci
COPY . .
RUN cd apps/web && npm run build

FROM golang:1.25-bookworm AS hub
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/apps/web/build ./apps/hub/internal/displayassets/dist
ARG VERSION=dev
RUN CGO_ENABLED=1 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/juted \
    ./apps/hub/cmd/juted

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --home-dir /data --create-home --shell /usr/sbin/nologin jute \
    && mkdir -p /config /data \
    && chown -R jute:jute /config /data

COPY --from=hub /out/juted /usr/local/bin/juted

ENV JUTE_HOME=/data \
    JUTE_CONFIG=/config/config.yaml \
    JUTE_LISTEN=0.0.0.0:8787

EXPOSE 8787
VOLUME ["/data"]
USER jute

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8787/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/juted"]
