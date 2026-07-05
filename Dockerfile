# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS go-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
    -X github.com/allbot/allbot/core/version.Commit=${COMMIT} \
    -X github.com/allbot/allbot/core/version.BuildTime=${BUILD_TIME} \
    -X github.com/allbot/allbot/core/version.BuildChannel=docker" \
    -o /out/allbot .

FROM debian:bookworm-slim AS runtime

ENV ALLBOT_WEB_PORT=3000 \
    ALLBOT_WEB_MODE=embedded \
    ALLBOT_UPDATE_MODE=docker \
    TZ=Asia/Shanghai

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    nodejs \
    npm \
    python3 \
    python3-pip \
    python3-venv \
    tzdata \
    && ln -sf /usr/bin/python3 /usr/local/bin/python \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /data

COPY --from=go-builder /out/allbot /opt/allbot/allbot
COPY sdk/ /opt/allbot/sdk/
COPY openapis/ /opt/allbot/openapis/
COPY runtime/package.json runtime/package-lock.json runtime/python_deps.json /opt/allbot/runtime/
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /usr/local/bin/docker-entrypoint.sh /opt/allbot/allbot \
    && sha256sum /opt/allbot/allbot | cut -d ' ' -f 1 > /opt/allbot/allbot.sha256 \
    && mkdir -p /data /opt/allbot/runtime

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS "http://127.0.0.1:${ALLBOT_WEB_PORT}/login" >/dev/null || curl -fsS "http://127.0.0.1:${ALLBOT_WEB_PORT}/" >/dev/null || exit 1

ENTRYPOINT ["docker-entrypoint.sh"]
