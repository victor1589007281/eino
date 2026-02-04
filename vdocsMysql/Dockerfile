# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies (including SQLite dev for CGO)
RUN apk add --no-cache git make gcc musl-dev sqlite-dev

# Set Go Proxy
ENV GOPROXY=https://goproxy.cn,direct

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Set working directory
WORKDIR /app

# Copy the entire eino workspace (needed for local module)
COPY . /workspace

# Change to the vdocsMysql directory
WORKDIR /workspace/vdocsMysql

# Build the binary with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/bin/mysql-expert-agent \
    ./cmd/main.go

# Build the integration test binary
RUN CGO_ENABLED=1 GOOS=linux go build \
    -o /app/bin/test_integration \
    ./cmd/test_integration/main.go

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    ripgrep \
    ctags \
    sqlite-libs \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -u 1000 -G appuser -s /bin/sh -D appuser

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/mysql-expert-agent /app/
COPY --from=builder /app/bin/test_integration /app/

# Create necessary directories
RUN mkdir -p /data/index /data/cache /data/output /data/source && \
    chown -R appuser:appuser /app /data

# Copy default config
COPY --from=builder /workspace/vdocsMysql/config/config-dev.yaml /app/config/config.yaml

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 9090

# Environment variables
ENV DEEPSEEK_API_KEY=""
ENV DATA_DIR="/data"
ENV SOURCE_PATH="/data/source"

# Default command
ENTRYPOINT ["/app/mysql-expert-agent"]
CMD ["--config=/app/config/config.yaml"]
