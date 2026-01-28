# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-s -w" -o bin/insurance-expert ./cmd/main.go

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite

# Set timezone
ENV TZ=Asia/Shanghai

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create directories
RUN mkdir -p /app/bin /data/index /data/cache /data/output /etc/insurance-expert && \
    chown -R appuser:appgroup /app /data /etc/insurance-expert

WORKDIR /app

# Copy binary
COPY --from=builder /app/bin/insurance-expert /app/bin/

# Copy default config
COPY config.yaml /etc/insurance-expert/config.yaml

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Entry point
ENTRYPOINT ["/app/bin/insurance-expert"]
CMD ["serve", "--config=/etc/insurance-expert/config.yaml"]
