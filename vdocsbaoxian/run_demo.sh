#!/bin/bash
# Insurance Expert Agent Demo Runner
# 保险专家Agent演示启动脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  保险专家Agent 演示启动脚本${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查API密钥
if [ -z "$DEEPSEEK_API_KEY" ]; then
    echo -e "${YELLOW}请输入您的DeepSeek API Key:${NC}"
    read -r DEEPSEEK_API_KEY
    export DEEPSEEK_API_KEY
fi

if [ -z "$DEEPSEEK_API_KEY" ]; then
    echo -e "${RED}❌ 错误: DEEPSEEK_API_KEY 未设置${NC}"
    exit 1
fi

echo -e "${GREEN}✅ API Key 已配置${NC}"

# 获取脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 检查是否需要编译
if [ ! -f "insurance-demo" ] || [ "cmd/demo/main.go" -nt "insurance-demo" ]; then
    echo -e "${YELLOW}⏳ 正在编译...${NC}"
    go build -o insurance-demo ./cmd/demo/
    echo -e "${GREEN}✅ 编译完成${NC}"
fi

# 运行演示
echo ""
echo -e "${BLUE}🚀 启动保险专家Agent演示...${NC}"
echo ""

./insurance-demo
