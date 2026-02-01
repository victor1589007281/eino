# 构建阶段
FROM golang:1.21-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -o /app/bin/image-gen-agent \
    ./cmd/main.go

# 运行阶段
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN adduser -D -g '' appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/bin/image-gen-agent /app/image-gen-agent

# 复制配置文件模板
COPY --from=builder /app/config/config-dev.yaml /app/config/config.yaml

# 创建必要的目录
RUN mkdir -p /data/output /data/cache && \
    chown -R appuser:appuser /app /data

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 8080

# 设置环境变量
ENV IMAGE_GEN_CONFIG=/app/config/config.yaml

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 入口点
ENTRYPOINT ["/app/image-gen-agent"]

# 默认参数
CMD ["-mode", "server"]
