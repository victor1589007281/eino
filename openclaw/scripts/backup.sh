#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════
# OpenClaw 完整备份脚本
# 备份 ~/.openclaw 全部内容（配置、workspace、skills、sessions、memory）
# 支持：时间戳归档、本地目录、可选 rsync 远程、自动清理 30 天前备份
#
# 使用方法:
#   ./backup.sh                    # 备份到默认目录 ~/openclaw-backups/
#   ./backup.sh /path/to/backup    # 指定备份目录
#   RSYNC_TARGET=user@host:/path ./backup.sh  # 备份后 rsync 到远程
#
# Crontab 定时备份（建议每日凌晨 3 点）:
#   1. 编辑 crontab: crontab -e
#   2. 添加一行（替换 /path/to 为实际路径）:
#      0 3 * * * /path/to/openclaw/agent-config/scripts/backup.sh >> ~/openclaw-backups/backup.log 2>&1
#   3. 或带远程同步:
#      0 3 * * * RSYNC_TARGET=user@host:/backups/openclaw /path/to/scripts/backup.sh >> ~/openclaw-backups/backup.log 2>&1
# ═══════════════════════════════════════════════════════════════════════════
set -e

OPENCLAW_HOME="${OPENCLAW_HOME:-$HOME/.openclaw}"
BACKUP_BASE="${1:-$HOME/openclaw-backups}"
RSYNC_TARGET="${RSYNC_TARGET:-}"
KEEP_DAYS="${KEEP_DAYS:-30}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
ARCHIVE_NAME="openclaw_backup_${TIMESTAMP}.tar.gz"
ARCHIVE_PATH="${BACKUP_BASE}/${ARCHIVE_NAME}"
LOG_FILE="${BACKUP_BASE}/backup.log"

mkdir -p "$BACKUP_BASE"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

log "========== OpenClaw 备份开始 =========="
log "源目录: $OPENCLAW_HOME"
log "备份目录: $BACKUP_BASE"
log "归档文件: $ARCHIVE_NAME"

if [ ! -d "$OPENCLAW_HOME" ]; then
  log "错误: OpenClaw 目录不存在: $OPENCLAW_HOME"
  exit 1
fi

# 备份整个 ~/.openclaw 目录（排除 node_modules、.git、缓存以减小体积）
log "正在创建归档..."
tar czf "$ARCHIVE_PATH" \
  -C "$(dirname "$OPENCLAW_HOME")" \
  --exclude='node_modules' \
  --exclude='.git' \
  --exclude='*.log' \
  --exclude='__pycache__' \
  --exclude='.cache' \
  "$(basename "$OPENCLAW_HOME")" 2>/dev/null || {
  log "tar 失败，尝试精简备份..."
  tar czf "$ARCHIVE_PATH" -C "$HOME" .openclaw 2>/dev/null || {
    log "错误: 备份失败"
    exit 1
  }
}

SIZE=$(du -h "$ARCHIVE_PATH" | cut -f1)
log "备份完成: $ARCHIVE_PATH ($SIZE)"

# 可选：rsync 到远程
if [ -n "$RSYNC_TARGET" ]; then
  log "正在同步到远程: $RSYNC_TARGET"
  rsync -avz "$ARCHIVE_PATH" "$RSYNC_TARGET/" || log "警告: rsync 失败"
fi

# 清理 30 天前的备份
log "清理 ${KEEP_DAYS} 天前的旧备份..."
find "$BACKUP_BASE" -name "openclaw_backup_*.tar.gz" -mtime +${KEEP_DAYS} -delete 2>/dev/null || true
REMAINING=$(find "$BACKUP_BASE" -name "openclaw_backup_*.tar.gz" 2>/dev/null | wc -l)
log "当前保留备份数: $REMAINING"

log "========== 备份完成 =========="
echo "$ARCHIVE_PATH"
