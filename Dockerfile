# Multi-stage build for the bore relay server.
#
# Build: docker build -t bore-relay .
# Run:   docker run -p 8080:8080 bore-relay

# --- Build stage ---
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the relay binary statically.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/bore-relay ./cmd/relay

# --- Runtime stage ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -s /sbin/nologin bore

COPY --from=builder /bin/bore-relay /usr/local/bin/bore-relay

USER bore
EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["bore-relay"]
