# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# 复制整个eino项目（因为go.mod有replace指令）
COPY . .

WORKDIR /build/vdocs/kernel_expert

# Download dependencies
RUN go mod download

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /kernel-expert .

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata ripgrep

# Set timezone
ENV TZ=Asia/Shanghai

WORKDIR /app

# Copy binary from builder
COPY --from=builder /kernel-expert /app/kernel-expert

# Copy skills
COPY --from=builder /build/vdocs/kernel_expert/skills/ /app/skills/

# Create directories
RUN mkdir -p /app/data /app/index /app/cache /app/logs

# Environment variables
ENV KERNEL_SOURCE_PATH=/linux
ENV KERNEL_INDEX_PATH=/app/index

# Expose port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/app/kernel-expert"]

# Default command (interactive mode)
CMD ["--help"]
