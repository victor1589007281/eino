# OCR Agent

基于 [eino](https://github.com/cloudwego/eino) 框架开发的智能OCR识别Agent，支持图片、PDF、发票等文档的识别和结构化信息提取。

## 功能特性

- 🖼️ **图片OCR**: 支持多种图片格式（JPG、PNG、GIF、BMP、TIFF、WebP）
- 📄 **PDF处理**: 支持文本提取、图片提取、扫描件OCR
- 🧾 **发票识别**: 自动识别多种发票类型并提取关键字段
- 🤖 **智能分析**: 集成LLM进行文本整理和结构化输出
- 📊 **多格式输出**: 支持JSON、Markdown、HTML、Text、CSV输出
- ☸️ **K8S部署**: 提供完整的Kubernetes部署方案

## 支持的发票类型

| 发票类型 | 说明 |
|---------|------|
| 增值税普通发票 | 含发票代码、号码、日期、金额等 |
| 增值税专用发票 | 含完整购销方信息和商品明细 |
| 火车票 | 出发站、到达站、日期、座位、金额 |
| 出租车发票 | 日期、金额、里程 |
| 机票 | 航班号、出发/到达城市、日期 |
| 酒店发票 | 酒店名称、入住日期、金额 |
| 过路费发票 | 入口/出口、金额 |

## 快速开始

### 前置条件

- Go 1.22+
- Tesseract OCR 4.0+（用于本地OCR）
- poppler-utils（用于PDF处理）

### 安装依赖

```bash
# macOS
brew install tesseract tesseract-lang poppler

# Ubuntu/Debian
apt-get install tesseract-ocr tesseract-ocr-chi-sim tesseract-ocr-eng poppler-utils

# CentOS/RHEL
yum install tesseract tesseract-langpack-chi_sim poppler-utils
```

### 构建运行

```bash
# 克隆项目
cd eino/vdocsOCR

# 安装依赖
go mod download

# 构建
make build

# 运行
./ocr-agent --config config/config.yaml
```

### API使用

#### 图片OCR

```bash
curl -X POST http://localhost:8080/api/ocr \
  -F "file=@image.jpg" \
  -F "language=chi_sim+eng" \
  -F "format=json"
```

#### 发票识别

```bash
curl -X POST http://localhost:8080/api/invoice \
  -F "file=@invoice.jpg"
```

#### PDF处理

```bash
curl -X POST http://localhost:8080/api/pdf \
  -F "file=@document.pdf" \
  -F "page_range=1-5"
```

## 配置

### 配置文件示例

```yaml
server:
  host: "0.0.0.0"
  port: 8080

llm:
  default_provider: "deepseek"
  providers:
    deepseek:
      type: "deepseek"
      enabled: true
      model: "deepseek-chat"

ocr:
  engine: "tesseract"
  language: "chi_sim+eng"
  max_concurrency: 4

output:
  format: "json"
  output_dir: "./output"
```

### 环境变量

| 变量名 | 说明 |
|--------|------|
| `DEEPSEEK_API_KEY` | DeepSeek API密钥 |
| `QWEN_API_KEY` | 通义千问API密钥 |
| `OPENAI_API_KEY` | OpenAI API密钥 |
| `BAIDU_OCR_API_KEY` | 百度OCR API密钥 |
| `PORT` | 服务端口 |
| `LOG_LEVEL` | 日志级别 |

## 项目结构

```
vdocsOCR/
├── agent/           # Agent实现
│   ├── agent.go     # OCR Agent主体
│   ├── factory.go   # Agent工厂
│   ├── interface.go # 接口定义
│   └── tools.go     # 工具实现
├── cmd/             # 入口程序
│   └── main.go
├── config/          # 配置管理
│   ├── config.go
│   └── config.yaml
├── design/          # 设计文档
├── k8s/             # K8S部署文件
├── output/          # 输出格式化
├── tools/           # 核心工具
│   ├── invoice/     # 发票识别
│   ├── ocr/         # OCR引擎
│   └── pdf/         # PDF处理
├── Dockerfile
├── Makefile
└── README.md
```

## Kubernetes部署

### 部署步骤

```bash
# 创建命名空间和配置
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml

# 创建Secret（需先修改API密钥）
kubectl apply -f k8s/secret.yaml

# 部署应用
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml

# 或一键部署
make k8s-deploy
```

### 查看状态

```bash
kubectl get all -n ocr-agent
```

## 测试

### 运行单元测试

```bash
make test-unit
```

### 运行集成测试

```bash
# 设置测试图片路径
export TEST_IMAGE_PATH=/path/to/test/image.jpg

make test-integration
```

### 生成测试覆盖率报告

```bash
make test-coverage
```

## API文档

### 健康检查

```
GET /health    - 存活检查
GET /ready     - 就绪检查
GET /version   - 版本信息
```

### OCR接口

```
POST /api/ocr
Content-Type: multipart/form-data

参数:
- file: 图片文件
- language: OCR语言 (默认: chi_sim+eng)
- format: 输出格式 (json/markdown/text)
```

### 发票识别接口

```
POST /api/invoice
Content-Type: multipart/form-data

参数:
- file: 发票图片
```

### PDF处理接口

```
POST /api/pdf
Content-Type: multipart/form-data

参数:
- file: PDF文件
- page_range: 页面范围 (默认: all)
```

## 性能指标

| 指标 | 值 |
|------|-----|
| 图片OCR延迟 | < 3s (中等尺寸图片) |
| PDF处理速度 | ~2s/页 |
| 发票识别准确率 | > 95% (清晰图片) |
| 最大并发 | 可配置 (默认4) |

## 开发指南

### 添加新的发票类型

1. 在 `tools/invoice/invoice.go` 中添加发票类型常量
2. 添加对应的正则表达式模式
3. 实现字段提取函数
4. 更新 `classifyInvoice` 方法

### 添加新的OCR引擎

1. 实现 `OCREngine` 接口
2. 在 `OCRManager` 中注册新引擎
3. 添加配置支持

## License

Apache-2.0

## 贡献

欢迎提交Issue和Pull Request！
