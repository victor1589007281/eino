#!/bin/bash
set -euo pipefail

# ============================================================================
# 安装测试 Agent 到 OpenClaw
# 用法: ./deploy/scripts/install-test-agent.sh [OPENCLAW_DIR]
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
OPENCLAW_DIR="${1:-$HOME/.openclaw}"

AGENT_SRC="$DEPLOY_DIR/test-agent"
AGENT_DEST="$OPENCLAW_DIR/agents/test-coordinator"

echo "============================================"
echo "  安装测试 Agent"
echo "  Source: $AGENT_SRC"
echo "  Dest:   $AGENT_DEST"
echo "============================================"

# --- 复制 Agent 文件 ---
mkdir -p "$AGENT_DEST/agent"
cp -r "$AGENT_SRC/agent/"* "$AGENT_DEST/agent/"

echo "✅ 测试 Agent 已安装到 $AGENT_DEST"
echo ""
echo ">>> 还需要在 openclaw.json 的 agents.list 中添加:"
echo ""
cat <<'JSON'
{
  "id": "TEST_COORDINATOR",
  "name": "测试协调员",
  "agentDir": "<OPENCLAW_DIR>/agents/test-coordinator/agent",
  "subagents": { "allowAgents": ["*"] }
}
JSON
echo ""
echo ">>> 然后重启 OpenClaw 加载新 Agent"
