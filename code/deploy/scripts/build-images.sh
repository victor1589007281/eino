#!/bin/bash
set -euo pipefail

# ============================================================================
# 构建 Docker 镜像
# 用法: ./deploy/scripts/build-images.sh [REGISTRY]
# 示例: ./deploy/scripts/build-images.sh registry.cn-hangzhou.aliyuncs.com/myns
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
REGISTRY="${1:-}"
TAG="${TAG:-latest}"

echo "============================================"
echo "  构建 Docker 镜像"
echo "  Root: $ROOT_DIR"
echo "  Registry: ${REGISTRY:-local}"
echo "  Tag: $TAG"
echo "============================================"

# --- Go Service ---
echo ""
echo ">>> [1/2] Building collab-go-service..."
docker build \
  -t collab-go-service:${TAG} \
  -f "$ROOT_DIR/go-collab-service/Dockerfile" \
  "$ROOT_DIR/go-collab-service"

if [ -n "$REGISTRY" ]; then
  docker tag collab-go-service:${TAG} "${REGISTRY}/collab-go-service:${TAG}"
  echo "    Tagged: ${REGISTRY}/collab-go-service:${TAG}"
fi

# --- TS Plugin ---
echo ""
echo ">>> [2/2] Building collab-ts-plugin..."
docker build \
  -t collab-ts-plugin:${TAG} \
  -f "$ROOT_DIR/ts-plugins/collab/Dockerfile" \
  "$ROOT_DIR/ts-plugins/collab"

if [ -n "$REGISTRY" ]; then
  docker tag collab-ts-plugin:${TAG} "${REGISTRY}/collab-ts-plugin:${TAG}"
  echo "    Tagged: ${REGISTRY}/collab-ts-plugin:${TAG}"
fi

echo ""
echo "✅ 镜像构建完成:"
docker images | grep collab | head -5

# --- Push (optional) ---
if [ -n "$REGISTRY" ]; then
  echo ""
  read -p ">>> 推送到 ${REGISTRY}? [y/N] " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    docker push "${REGISTRY}/collab-go-service:${TAG}"
    docker push "${REGISTRY}/collab-ts-plugin:${TAG}"
    echo "✅ 推送完成"
  fi
fi
