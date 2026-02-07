#!/bin/bash
# OCR 模型下载脚本
# 用于下载和配置各种 OCR 引擎所需的模型

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODEL_DIR="${MODEL_DIR:-$SCRIPT_DIR/../models}"

echo "======================================"
echo "OCR 模型下载脚本"
echo "模型目录: $MODEL_DIR"
echo "======================================"

# 创建模型目录
mkdir -p "$MODEL_DIR"
cd "$MODEL_DIR"

# 1. 安装 RapidOCR (推荐用于 CPU)
install_rapidocr() {
    echo ""
    echo "[1/4] 安装 RapidOCR..."
    
    if command -v pip3 &> /dev/null; then
        pip3 install rapidocr_onnxruntime flask pillow -q
        echo "✅ RapidOCR 安装成功"
    else
        echo "❌ 未找到 pip3，请先安装 Python3"
        return 1
    fi
}

# 2. 安装 PaddleOCR (可选)
install_paddleocr() {
    echo ""
    echo "[2/4] 安装 PaddleOCR (可选)..."
    
    if command -v pip3 &> /dev/null; then
        # 安装 PaddlePaddle (CPU 版本)
        pip3 install paddlepaddle paddleocr -q
        echo "✅ PaddleOCR 安装成功"
    else
        echo "❌ 未找到 pip3，跳过 PaddleOCR"
        return 1
    fi
}

# 3. 安装 Tesseract
install_tesseract() {
    echo ""
    echo "[3/4] 检查 Tesseract..."
    
    if command -v tesseract &> /dev/null; then
        TESS_VERSION=$(tesseract --version 2>&1 | head -1)
        echo "✅ Tesseract 已安装: $TESS_VERSION"
    else
        echo "Tesseract 未安装，正在安装..."
        
        # 根据系统安装
        if [[ "$OSTYPE" == "darwin"* ]]; then
            # macOS
            if command -v brew &> /dev/null; then
                brew install tesseract tesseract-lang
                echo "✅ Tesseract 安装成功 (macOS)"
            else
                echo "❌ 请先安装 Homebrew"
                return 1
            fi
        elif [[ -f /etc/debian_version ]]; then
            # Debian/Ubuntu
            sudo apt-get update
            sudo apt-get install -y tesseract-ocr tesseract-ocr-chi-sim tesseract-ocr-chi-tra
            echo "✅ Tesseract 安装成功 (Debian/Ubuntu)"
        elif [[ -f /etc/redhat-release ]]; then
            # CentOS/RHEL
            sudo yum install -y tesseract tesseract-langpack-chi_sim
            echo "✅ Tesseract 安装成功 (CentOS/RHEL)"
        else
            echo "❌ 不支持的操作系统，请手动安装 Tesseract"
            return 1
        fi
    fi
    
    # 检查中文语言包
    LANGS=$(tesseract --list-langs 2>&1)
    if echo "$LANGS" | grep -q "chi_sim"; then
        echo "✅ 中文简体语言包已安装"
    else
        echo "⚠️  中文简体语言包未安装"
    fi
}

# 4. 下载 RapidOCR 模型 (如果需要离线使用)
download_rapidocr_models() {
    echo ""
    echo "[4/4] 下载 RapidOCR 模型 (离线模式)..."
    
    RAPIDOCR_MODEL_DIR="$MODEL_DIR/rapidocr"
    mkdir -p "$RAPIDOCR_MODEL_DIR"
    
    # RapidOCR 模型会在首次运行时自动下载
    # 这里提供手动下载选项
    
    BASE_URL="https://github.com/RapidAI/RapidOCR/releases/download/v1.3.0"
    
    # 检测模型
    if [[ ! -f "$RAPIDOCR_MODEL_DIR/ch_PP-OCRv4_det_infer.onnx" ]]; then
        echo "下载检测模型..."
        curl -L -o "$RAPIDOCR_MODEL_DIR/ch_PP-OCRv4_det_infer.onnx" \
            "$BASE_URL/ch_PP-OCRv4_det_infer.onnx" || echo "⚠️  下载失败"
    fi
    
    # 识别模型
    if [[ ! -f "$RAPIDOCR_MODEL_DIR/ch_PP-OCRv4_rec_infer.onnx" ]]; then
        echo "下载识别模型..."
        curl -L -o "$RAPIDOCR_MODEL_DIR/ch_PP-OCRv4_rec_infer.onnx" \
            "$BASE_URL/ch_PP-OCRv4_rec_infer.onnx" || echo "⚠️  下载失败"
    fi
    
    # 方向分类模型
    if [[ ! -f "$RAPIDOCR_MODEL_DIR/ch_ppocr_mobile_v2.0_cls_infer.onnx" ]]; then
        echo "下载方向分类模型..."
        curl -L -o "$RAPIDOCR_MODEL_DIR/ch_ppocr_mobile_v2.0_cls_infer.onnx" \
            "$BASE_URL/ch_ppocr_mobile_v2.0_cls_infer.onnx" || echo "⚠️  下载失败"
    fi
    
    echo "✅ RapidOCR 模型下载完成"
}

# 显示帮助
show_help() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  all          安装所有组件 (默认)"
    echo "  rapidocr     仅安装 RapidOCR"
    echo "  paddleocr    仅安装 PaddleOCR"
    echo "  tesseract    仅安装 Tesseract"
    echo "  models       仅下载模型"
    echo "  help         显示帮助"
    echo ""
    echo "环境变量:"
    echo "  MODEL_DIR    模型存储目录 (默认: ./models)"
}

# 主函数
main() {
    case "${1:-all}" in
        all)
            install_rapidocr
            install_tesseract
            # install_paddleocr  # 可选，按需启用
            ;;
        rapidocr)
            install_rapidocr
            ;;
        paddleocr)
            install_paddleocr
            ;;
        tesseract)
            install_tesseract
            ;;
        models)
            download_rapidocr_models
            ;;
        help|--help|-h)
            show_help
            exit 0
            ;;
        *)
            echo "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
    
    echo ""
    echo "======================================"
    echo "安装完成!"
    echo ""
    echo "启动 RapidOCR 服务:"
    echo "  python3 $SCRIPT_DIR/../services/rapidocr_server.py --port 8089"
    echo ""
    echo "测试 Tesseract:"
    echo "  tesseract test.png output -l chi_sim+eng"
    echo "======================================"
}

main "$@"
