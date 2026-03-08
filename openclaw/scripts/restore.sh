#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════
# OpenClaw 恢复脚本
# 从备份 tar.gz 恢复 ~/.openclaw
#
# 使用方法:
#   ./restore.sh /path/to/openclaw_backup_20260308_030000.tar.gz
#
# 流程: 停止 openclaw → 确认覆盖 → 解压恢复 → 重启 openclaw → 验证技能
# ═══════════════════════════════════════════════════════════════════════════
set -e

BACKUP_FILE="${1:?用法: $0 <backup.tar.gz 路径>}"
OPENCLAW_HOME="${OPENCLAW_HOME:-$HOME/.openclaw}"

echo "========== OpenClaw 恢复 =========="
echo "备份文件: $BACKUP_FILE"
echo "目标目录: $OPENCLAW_HOME"
echo ""

if [ ! -f "$BACKUP_FILE" ]; then
  echo "错误: 备份文件不存在: $BACKUP_FILE"
  exit 1
fi

# 安全确认
echo "警告: 此操作将覆盖 $OPENCLAW_HOME 的内容"
read -p "确认继续? (y/N): " confirm
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
  echo "已取消"
  exit 0
fi

# 停止 openclaw（若在运行）
echo ""
echo "正在检查 OpenClaw 进程..."
if pgrep -f "openclaw" >/dev/null 2>&1; then
  echo "检测到 OpenClaw 正在运行，正在停止..."
  pkill -f "openclaw" 2>/dev/null || true
  sleep 2
fi

# 备份现有目录（可选，防止误操作）
if [ -d "$OPENCLAW_HOME" ]; then
  PRE_RESTORE_BACKUP="${OPENCLAW_HOME}.pre-restore-$(date +%Y%m%d_%H%M%S)"
  echo "将现有目录重命名为: $PRE_RESTORE_BACKUP"
  mv "$OPENCLAW_HOME" "$PRE_RESTORE_BACKUP"
fi

# 解压恢复
echo ""
echo "正在解压恢复..."
mkdir -p "$(dirname "$OPENCLAW_HOME")"
tar xzf "$BACKUP_FILE" -C "$(dirname "$OPENCLAW_HOME")"

# 确保目录名正确（备份可能包含 .openclaw 子目录）
if [ -d "$(dirname "$OPENCLAW_HOME")/.openclaw" ] && [ ! -d "$OPENCLAW_HOME" ]; then
  mv "$(dirname "$OPENCLAW_HOME")/.openclaw" "$OPENCLAW_HOME"
fi

echo "恢复完成: $OPENCLAW_HOME"

# 验证技能
echo ""
echo "验证技能目录..."
if [ -d "$OPENCLAW_HOME/skills" ]; then
  SKILL_COUNT=$(find "$OPENCLAW_HOME/skills" -maxdepth 2 -name "SKILL.md" 2>/dev/null | wc -l)
  echo "已加载技能数: $SKILL_COUNT"
else
  echo "提示: skills 目录不存在，请运行 install-skills.sh 安装"
fi

# 重启提示
echo ""
echo "========== 恢复完成 =========="
echo "请手动启动 OpenClaw: openclaw start"
echo "或: openclaw gateway feishu"
echo ""
