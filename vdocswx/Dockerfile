# 多阶段构建
# Stage 1: 构建阶段
FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make gcc musl-dev

# 设置工作目录
WORKDIR /app

# 复制go.mod和go.sum
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-s -w -X 'github.com/cloudwego/eino/vdocswx/cmd.Version=$(git describe --tags --abbrev=0 2>/dev/null || echo v0.1.0)'" \
    -o /app/wechat-polish \
    ./cmd/main.go

# Stage 2: 运行阶段
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    sqlite

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非root用户
RUN adduser -D -u 1000 appuser

# 创建必要目录
RUN mkdir -p /app/data /app/config /app/workspace /app/skills \
    && chown -R appuser:appuser /app

# 复制二进制文件
COPY --from=builder /app/wechat-polish /app/

# 复制默认配置
COPY config.yaml /app/config/
COPY skills/ /app/skills/

# 设置工作目录
WORKDIR /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 入口点
ENTRYPOINT ["/app/wechat-polish"]
CMD ["serve", "--config=/app/config/config.yaml"]
