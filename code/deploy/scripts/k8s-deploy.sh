#!/bin/bash
set -euo pipefail

# ============================================================================
# K8s 部署脚本
# 用法: ./deploy/scripts/k8s-deploy.sh [REGISTRY]
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
K8S_DIR="$(cd "$SCRIPT_DIR/../k8s" && pwd)"
REGISTRY="${1:-}"
TAG="${TAG:-latest}"

echo "============================================"
echo "  K8s 部署"
echo "  Manifests: $K8S_DIR"
echo "============================================"

# --- 如果指定了 registry，先替换镜像地址 ---
if [ -n "$REGISTRY" ]; then
  echo ">>> 更新镜像地址为 ${REGISTRY}/..."
  sed -i.bak "s|image: collab-go-service:latest|image: ${REGISTRY}/collab-go-service:${TAG}|g" "$K8S_DIR/go-service.yaml"
  sed -i.bak "s|imagePullPolicy: IfNotPresent|imagePullPolicy: Always|g" "$K8S_DIR/go-service.yaml"
fi

# --- Step 1: Namespace ---
echo ""
echo ">>> [1/4] 创建 namespace..."
kubectl apply -f "$K8S_DIR/namespace.yaml"

# --- Step 2: PostgreSQL ---
echo ""
echo ">>> [2/4] 部署 PostgreSQL..."
kubectl apply -f "$K8S_DIR/postgres.yaml"
echo "    等待 PostgreSQL 就绪..."
kubectl wait --for=condition=ready pod -l app=postgres -n collab --timeout=120s

# --- Step 3: Go Service ---
echo ""
echo ">>> [3/4] 部署 Go 协作服务..."
kubectl apply -f "$K8S_DIR/go-service.yaml"
echo "    等待 Go 服务就绪..."
kubectl wait --for=condition=ready pod -l app=go-service -n collab --timeout=120s

# --- Step 4: 验证 ---
echo ""
echo ">>> [4/4] 验证部署..."
echo ""
echo "--- Pods ---"
kubectl get pods -n collab
echo ""
echo "--- Services ---"
kubectl get svc -n collab
echo ""

# Health check via port-forward
echo ">>> 端口转发验证 health..."
kubectl port-forward -n collab svc/go-service 8090:8090 &
PF_PID=$!
sleep 3

HEALTH=$(curl -sf http://localhost:8090/health 2>/dev/null || echo '{"status":"fail"}')
echo "    Health: $HEALTH"

kill $PF_PID 2>/dev/null || true

# --- 恢复原文件 ---
if [ -n "$REGISTRY" ]; then
  mv "$K8S_DIR/go-service.yaml.bak" "$K8S_DIR/go-service.yaml"
fi

echo ""
echo "============================================"
echo "✅ K8s 部署完成"
echo ""
echo "  访问方式:"
echo "    NodePort: http://<NODE_IP>:30090"
echo "    Port-forward: kubectl port-forward -n collab svc/go-service 8090:8090"
echo ""
echo "  OpenClaw 插件中配置:"
echo "    goServiceUrl: http://<NODE_IP>:30090"
echo "============================================"
