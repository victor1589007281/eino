#!/usr/bin/env python3
"""
RapidOCR HTTP 服务
为 Go OCR 工具提供 HTTP API 接口

安装依赖:
    pip install rapidocr_onnxruntime flask pillow

运行:
    python rapidocr_server.py --port 8089
"""

import argparse
import base64
import io
import json
import logging
import os
import sys
import time
from typing import Any, Dict, List, Optional

from flask import Flask, jsonify, request

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# 全局 RapidOCR 实例
ocr_engine = None


def init_ocr_engine():
    """初始化 RapidOCR 引擎"""
    global ocr_engine
    
    try:
        from rapidocr_onnxruntime import RapidOCR
        
        # 配置参数
        config = {
            'use_det': True,      # 使用检测模型
            'use_cls': True,      # 使用方向分类模型
            'use_rec': True,      # 使用识别模型
        }
        
        ocr_engine = RapidOCR(**config)
        logger.info("RapidOCR 引擎初始化成功")
        return True
        
    except ImportError as e:
        logger.error(f"导入 RapidOCR 失败: {e}")
        logger.error("请安装: pip install rapidocr_onnxruntime")
        return False
    except Exception as e:
        logger.error(f"初始化 RapidOCR 失败: {e}")
        return False


@app.route('/health', methods=['GET'])
def health_check():
    """健康检查"""
    if ocr_engine is None:
        return jsonify({
            'status': 'unhealthy',
            'message': 'OCR engine not initialized'
        }), 503
    
    return jsonify({
        'status': 'healthy',
        'engine': 'RapidOCR',
        'version': '1.0.0'
    })


@app.route('/ocr', methods=['POST'])
def ocr():
    """OCR 识别接口"""
    start_time = time.time()
    
    if ocr_engine is None:
        return jsonify({
            'code': -1,
            'message': 'OCR engine not initialized',
            'data': None
        }), 500
    
    try:
        # 获取图片数据
        image_data = None
        
        # 方式1: multipart/form-data 上传文件
        if 'image' in request.files:
            file = request.files['image']
            image_data = file.read()
        
        # 方式2: JSON body 中的 base64
        elif request.is_json:
            data = request.get_json()
            if 'image_base64' in data:
                b64_str = data['image_base64']
                # 移除 data:image/...;base64, 前缀
                if ',' in b64_str:
                    b64_str = b64_str.split(',')[1]
                image_data = base64.b64decode(b64_str)
            elif 'image_path' in data:
                image_path = data['image_path']
                if os.path.exists(image_path):
                    with open(image_path, 'rb') as f:
                        image_data = f.read()
        
        if image_data is None:
            return jsonify({
                'code': -2,
                'message': 'No image provided. Use "image" file or "image_base64" in JSON',
                'data': None
            }), 400
        
        # 获取可选参数
        detect_angle = request.form.get('detect_angle', 'false').lower() == 'true'
        
        # 执行 OCR
        from PIL import Image
        img = Image.open(io.BytesIO(image_data))
        
        # RapidOCR 接受 numpy array 或 PIL Image
        result, elapsed = ocr_engine(img, use_det=True, use_cls=detect_angle, use_rec=True)
        
        elapsed_time = time.time() - start_time
        
        # 格式化结果
        results = []
        if result is not None:
            for item in result:
                # item 格式: [box, text, confidence]
                # box 格式: [[x1,y1], [x2,y2], [x3,y3], [x4,y4]]
                box, text, confidence = item
                results.append({
                    'text': text,
                    'confidence': float(confidence),
                    'box': [[float(pt[0]), float(pt[1])] for pt in box]
                })
        
        return jsonify({
            'code': 0,
            'message': 'success',
            'data': {
                'results': results,
                'elapsed_time': elapsed_time,
                'image_width': img.width,
                'image_height': img.height
            }
        })
        
    except Exception as e:
        logger.error(f"OCR 处理失败: {e}", exc_info=True)
        return jsonify({
            'code': -3,
            'message': str(e),
            'data': None
        }), 500


@app.route('/ocr/table', methods=['POST'])
def ocr_table():
    """表格识别接口 (需要额外安装 rapidocr_layout)"""
    # TODO: 集成 RapidLayout 或 PP-Structure 进行表格识别
    return jsonify({
        'code': -1,
        'message': 'Table OCR not implemented yet. Please use PaddleOCR PP-Structure.',
        'data': None
    }), 501


@app.route('/info', methods=['GET'])
def info():
    """获取服务信息"""
    return jsonify({
        'service': 'RapidOCR Server',
        'version': '1.0.0',
        'engine': 'RapidOCR (ONNX Runtime)',
        'endpoints': {
            '/health': 'GET - 健康检查',
            '/ocr': 'POST - OCR 识别',
            '/ocr/table': 'POST - 表格识别',
            '/info': 'GET - 服务信息'
        },
        'usage': {
            'file_upload': 'POST /ocr with multipart/form-data, field name: image',
            'base64': 'POST /ocr with JSON body: {"image_base64": "..."}',
            'file_path': 'POST /ocr with JSON body: {"image_path": "/path/to/image.jpg"}'
        }
    })


def main():
    parser = argparse.ArgumentParser(description='RapidOCR HTTP Server')
    parser.add_argument('--host', default='0.0.0.0', help='监听地址')
    parser.add_argument('--port', type=int, default=8089, help='监听端口')
    parser.add_argument('--debug', action='store_true', help='调试模式')
    args = parser.parse_args()
    
    # 初始化 OCR 引擎
    if not init_ocr_engine():
        logger.error("无法初始化 OCR 引擎，退出")
        sys.exit(1)
    
    logger.info(f"启动 RapidOCR 服务: http://{args.host}:{args.port}")
    app.run(host=args.host, port=args.port, debug=args.debug)


if __name__ == '__main__':
    main()
