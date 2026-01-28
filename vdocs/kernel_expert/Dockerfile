# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /kernel-expert .

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata ripgrep

# Set timezone
ENV TZ=Asia/Shanghai

WORKDIR /app

# Copy binary from builder
COPY --from=builder /kernel-expert /app/kernel-expert

# Copy config and skills
COPY config/ /app/config/
COPY skills/ /app/skills/

# Create directories
RUN mkdir -p /app/data /app/index /app/cache /app/logs

# Expose port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/app/kernel-expert"]

# Default command
CMD ["serve", "--config", "/app/config/config.yaml"]
