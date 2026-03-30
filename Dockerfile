# Multi-stage build for the bore relay server.
#
# Build: docker build -t bore-relay .
# Run:   docker run -p 8080:8080 bore-relay

# --- Web build stage ---
FROM oven/bun:1.3.10-alpine AS web-builder

WORKDIR /src/web
COPY web/package.json ./
COPY web/astro.config.mjs ./
COPY web/tsconfig.json ./
COPY web/biome.json ./
RUN bun install

COPY web/src ./src
COPY web/tests ./tests
RUN bun run build

# --- Build stage ---
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/web/dist ./web/dist

# Build the relay binary statically.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/bore-relay ./cmd/relay

# --- Runtime stage ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -s /sbin/nologin bore

WORKDIR /app
COPY --from=builder /bin/bore-relay /usr/local/bin/bore-relay
COPY --from=web-builder /src/web/dist ./web/dist

USER bore
EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["bore-relay"]
