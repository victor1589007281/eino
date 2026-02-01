# Build stage
FROM golang:1.22-bookworm AS builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y \
    git \
    && rm -rf /var/lib/apt/lists/*

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o ocr-agent ./cmd/main.go

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tesseract-ocr \
    tesseract-ocr-chi-sim \
    tesseract-ocr-chi-tra \
    tesseract-ocr-eng \
    poppler-utils \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -r ocragent && useradd -r -g ocragent ocragent

# Create directories
RUN mkdir -p /app /data /data/output /etc/ocr-agent \
    && chown -R ocragent:ocragent /app /data /etc/ocr-agent

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/ocr-agent .

# Copy default config
COPY config/config.yaml /etc/ocr-agent/config.yaml

# Set user
USER ocragent

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set entrypoint
ENTRYPOINT ["/app/ocr-agent"]
CMD ["--config", "/etc/ocr-agent/config.yaml"]
