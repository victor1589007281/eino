# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

# First, copy the entire eino project for the replace directive
COPY . /build/eino

WORKDIR /build/eino/vdocsbaoxian

# Download dependencies
RUN go mod download

# Build the demo binary (use native architecture)
RUN CGO_ENABLED=1 go build -ldflags "-s -w" -o /insurance-demo ./cmd/demo/

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite

# Set timezone
ENV TZ=Asia/Shanghai

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create directories
RUN mkdir -p /app /data && \
    chown -R appuser:appgroup /app /data

WORKDIR /app

# Copy binary
COPY --from=builder /insurance-demo /app/

# Switch to non-root user
USER appuser

# Entry point
ENTRYPOINT ["/app/insurance-demo"]
