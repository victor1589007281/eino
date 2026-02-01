#!/bin/bash

# OCR Agent Demo Script
# 运行OCR Agent演示

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}       OCR Agent Demo                  ${NC}"
echo -e "${GREEN}========================================${NC}"

# Check if config exists
if [ ! -f "config/config.yaml" ]; then
    echo -e "${YELLOW}Config file not found, using default config${NC}"
fi

# Check for required tools
echo -e "${YELLOW}Checking dependencies...${NC}"

if ! command -v tesseract &> /dev/null; then
    echo -e "${RED}Warning: tesseract not found. OCR functionality will be limited.${NC}"
    echo -e "${YELLOW}Install with: brew install tesseract tesseract-lang (macOS)${NC}"
fi

if ! command -v pdftotext &> /dev/null; then
    echo -e "${RED}Warning: poppler-utils not found. PDF functionality will be limited.${NC}"
    echo -e "${YELLOW}Install with: brew install poppler (macOS)${NC}"
fi

# Build if binary doesn't exist
if [ ! -f "ocr-agent" ]; then
    echo -e "${YELLOW}Building OCR Agent...${NC}"
    go build -o ocr-agent ./cmd/main.go
fi

# Check environment variables
if [ -z "$DEEPSEEK_API_KEY" ]; then
    echo -e "${YELLOW}Warning: DEEPSEEK_API_KEY not set. LLM features will be disabled.${NC}"
fi

# Run the agent
echo -e "${GREEN}Starting OCR Agent...${NC}"
echo -e "${YELLOW}Server will be available at http://localhost:8080${NC}"
echo ""
echo -e "${YELLOW}API Endpoints:${NC}"
echo "  - Health Check:  GET  http://localhost:8080/health"
echo "  - Ready Check:   GET  http://localhost:8080/ready"
echo "  - Version:       GET  http://localhost:8080/version"
echo "  - Capabilities:  GET  http://localhost:8080/api/capabilities"
echo "  - OCR:           POST http://localhost:8080/api/ocr"
echo "  - Invoice:       POST http://localhost:8080/api/invoice"
echo "  - PDF:           POST http://localhost:8080/api/pdf"
echo ""
echo -e "${YELLOW}Example usage:${NC}"
echo '  curl -X POST http://localhost:8080/api/ocr -F "file=@image.jpg"'
echo '  curl -X POST http://localhost:8080/api/invoice -F "file=@invoice.jpg"'
echo '  curl -X POST http://localhost:8080/api/pdf -F "file=@doc.pdf"'
echo ""
echo -e "${GREEN}Press Ctrl+C to stop${NC}"
echo ""

# Run with config if exists
if [ -f "config/config.yaml" ]; then
    ./ocr-agent --config config/config.yaml
else
    ./ocr-agent
fi
