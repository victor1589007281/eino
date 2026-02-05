# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/mcp-server ./cmd/mcp-server

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/mcp-server /app/mcp-server

# Copy skills documentation (optional, for reference)
COPY --from=builder /app/skills /app/skills

# Create work directory for logs and data
RUN mkdir -p /app/work && chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose MCP server port
EXPOSE 8080

# Set environment variables
ENV MCP_LISTEN_ADDR=:8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the application
ENTRYPOINT ["/app/mcp-server"]
CMD ["-config", "/app/work/config.json"]
