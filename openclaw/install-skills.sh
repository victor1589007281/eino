#!/bin/bash
# ═══════════════════════════════════════════════════════════════════════════
# OpenClaw 技能一键安装脚本 - 中国大陆版
# 为 6 个 Agent 安装全部所需技能（~20-25 个）
# 使用前请确保：npx / openclaw 已安装，网络可访问 ClawHub
# ═══════════════════════════════════════════════════════════════════════════
set -e

SKILLS_DIR="${SKILLS_DIR:-$HOME/.openclaw/skills}"
CONFIG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==========================================="
echo "  OpenClaw 技能安装 - 中国大陆版"
echo "  目标目录: $SKILLS_DIR"
echo "==========================================="
mkdir -p "$SKILLS_DIR"

# ─────────────────────────────────────────────────────────────────────────
# 安全类
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[1/9] 安装安全类技能..."
npx playbooks add skill openclaw/skills --skill secure-install
npx playbooks add skill openclaw/skills --skill clawsec-suite

# ─────────────────────────────────────────────────────────────────────────
# OCR 类
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[2/9] 安装 OCR 类技能..."
npx playbooks add skill openclaw/skills --skill smart-ocr
npx playbooks add skill openclaw/skills --skill pdf-text-extractor

# ─────────────────────────────────────────────────────────────────────────
# 文生图 / 文生视频（仅国内可达：硅基流动 / 即梦 / 可灵）
# 已移除 Pollinations（付费）、Google Gemini（被墙）、fal.ai/krea（海外不稳定）
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[3/9] 安装图片/视频生成技能（仅国内可达）..."
npx playbooks add skill openclaw/skills --skill siliconflow-image-gen
npx playbooks add skill openclaw/skills --skill siliconflow-video-gen
npx playbooks add skill openclaw/skills --skill video-gen

# ─────────────────────────────────────────────────────────────────────────
# 金融类
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[4/9] 安装金融类技能..."
npx playbooks add skill openclaw/skills --skill yahoofinance 2>/dev/null || npx playbooks add skill openclaw/skills --skill yahoo-finance 2>/dev/null || echo "  [跳过] yahoo-finance 未找到，可手动安装"
npx playbooks add skill openclaw/skills --skill tushare 2>/dev/null || npx playbooks add skill openclaw/skills --skill tushare-finance 2>/dev/null || echo "  [跳过] tushare-finance 未找到，需自建或从社区安装"
# stock-analysis, portfolio-watcher 可能为社区技能，按需安装
npx playbooks add skill openclaw/skills --skill stock-analysis 2>/dev/null || echo "  [跳过] stock-analysis 未找到"
npx playbooks add skill openclaw/skills --skill portfolio-watcher 2>/dev/null || echo "  [跳过] portfolio-watcher 未找到"

# ─────────────────────────────────────────────────────────────────────────
# 内容运营类
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[5/9] 安装内容运营类技能..."
npx playbooks add skill openclaw/skills --skill xiaohongshu 2>/dev/null || echo "  [跳过] xiaohongshu 未找到，可从 ClawHub 搜索或自建"
npx playbooks add skill openclaw/skills --skill wechat-accounts-publisher 2>/dev/null || echo "  [跳过] wechat-accounts-publisher 未找到"
npx playbooks add skill openclaw/skills --skill clawhub-publish 2>/dev/null || echo "  [跳过] clawhub-publish 未找到"

# ─────────────────────────────────────────────────────────────────────────
# 邮件类
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[6/9] 安装邮件类技能..."
npx playbooks add skill openclaw/skills --skill imap-smtp-email 2>/dev/null || npx playbooks add skill openclaw/skills --skill email 2>/dev/null || echo "  [跳过] imap-smtp-email 未找到"

# ─────────────────────────────────────────────────────────────────────────
# 研发类
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[7/9] 安装研发类技能..."
npx playbooks add skill openclaw/skills --skill github

# ─────────────────────────────────────────────────────────────────────────
# 工具类
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[8/9] 安装工具类技能..."
npx playbooks add skill openclaw/skills --skill memory-tools
npx playbooks add skill openclaw/skills --skill prompt-enhancer
npx playbooks add skill openclaw/skills --skill smart-router 2>/dev/null || echo "  [跳过] smart-router 未找到"
npx playbooks add skill openclaw/skills --skill backup-and-restore 2>/dev/null || echo "  [跳过] backup-and-restore 未找到"
npx playbooks add skill openclaw/skills --skill multi-user-workspace

# ─────────────────────────────────────────────────────────────────────────
# 自定义技能（从 agent-config 复制）
# ─────────────────────────────────────────────────────────────────────────
echo ""
echo "[9/9] 安装自定义技能..."
if [ -d "$CONFIG_DIR/skills/api-fallback-guard" ]; then
  cp -r "$CONFIG_DIR/skills/api-fallback-guard" "$SKILLS_DIR/"
  echo "  ✓ api-fallback-guard 已复制"
else
  echo "  [跳过] api-fallback-guard 源目录不存在"
fi

if [ -d "$CONFIG_DIR/skills/children-growth-handbook" ]; then
  cp -r "$CONFIG_DIR/skills/children-growth-handbook" "$SKILLS_DIR/"
  echo "  ✓ children-growth-handbook 已复制"
  if [ -f "$SKILLS_DIR/children-growth-handbook/scripts/requirements.txt" ]; then
    pip install -r "$SKILLS_DIR/children-growth-handbook/scripts/requirements.txt" -q 2>/dev/null || echo "  [提示] pip 安装依赖失败，请手动执行"
  fi
else
  echo "  [跳过] children-growth-handbook 源目录不存在"
fi

echo ""
echo "==========================================="
echo "  技能安装完成"
echo "==========================================="
echo "已安装技能目录: $SKILLS_DIR"
echo "请确保 openclaw.json 中 skills.load.extraDirs 包含: ~/.openclaw/skills"
echo "部分社区技能可能未找到，请到 ClawHub 搜索或自建后放入 $SKILLS_DIR"
echo ""
