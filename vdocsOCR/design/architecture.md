# OCR Agent 架构设计

## 1. 系统概述

OCR Agent 是一个基于 eino 框架开发的智能OCR识别系统，能够识别和提取图片、PDF、发票等文档中的信息，并输出结构化数据。

## 2. 架构图

```mermaid
graph TB
    subgraph "**客户端层**"
        CLI[**CLI 客户端**]
        API[**REST API**]
        WEB[**Web 界面**]
    end

    subgraph "**服务层**"
        GW[**API Gateway**]
        SVC[**OCR Service**]
        AUTH[**认证服务**]
    end

    subgraph "**Agent 层**"
        MASTER[**Master Agent**]
        OCR_AGENT[**OCR Agent**]
        INVOICE_AGENT[**Invoice Agent**]
        PDF_AGENT[**PDF Agent**]
    end

    subgraph "**工具层**"
        OCR_ENGINE[**OCR 引擎**]
        PDF_PROC[**PDF 处理器**]
        INVOICE_REC[**发票识别器**]
        LLM[**LLM 模型**]
    end

    subgraph "**存储层**"
        CACHE[**缓存**]
        DB[**数据库**]
        FILE[**文件存储**]
    end

    CLI --> GW
    API --> GW
    WEB --> GW
    GW --> AUTH
    GW --> SVC
    SVC --> MASTER
    MASTER --> OCR_AGENT
    MASTER --> INVOICE_AGENT
    MASTER --> PDF_AGENT
    OCR_AGENT --> OCR_ENGINE
    OCR_AGENT --> LLM
    INVOICE_AGENT --> INVOICE_REC
    INVOICE_AGENT --> LLM
    PDF_AGENT --> PDF_PROC
    PDF_AGENT --> LLM
    OCR_ENGINE --> CACHE
    INVOICE_REC --> DB
    PDF_PROC --> FILE

    style CLI fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style API fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style WEB fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style GW fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SVC fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style AUTH fill:#fffacd,stroke:#333,stroke-width:2px,color:#000
    style MASTER fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style OCR_AGENT fill:#e8e1ff,stroke:#333,stroke-width:2px,color:#000
    style INVOICE_AGENT fill:#d7ffd7,stroke:#333,stroke-width:2px,color:#000
    style PDF_AGENT fill:#ffe1f5,stroke:#333,stroke-width:2px,color:#000
    style OCR_ENGINE fill:#f5ffe1,stroke:#333,stroke-width:2px,color:#000
    style PDF_PROC fill:#e1e1ff,stroke:#333,stroke-width:2px,color:#000
    style INVOICE_REC fill:#fff0e1,stroke:#333,stroke-width:2px,color:#000
    style LLM fill:#e1fff5,stroke:#333,stroke-width:2px,color:#000
    style CACHE fill:#ffffe1,stroke:#333,stroke-width:2px,color:#000
    style DB fill:#e1ffff,stroke:#333,stroke-width:2px,color:#000
    style FILE fill:#ffe1ff,stroke:#333,stroke-width:2px,color:#000
```

## 3. 核心组件

### 3.1 Agent 层

| 组件 | 职责 | 技术实现 |
|------|------|---------|
| **Master Agent** | 任务调度、子Agent协调 | eino adk.Agent |
| **OCR Agent** | 图片OCR识别 | Tesseract / 云OCR API |
| **Invoice Agent** | 发票识别和结构化 | 正则 + LLM |
| **PDF Agent** | PDF文档处理 | poppler-utils |

### 3.2 工具层

| 工具 | 功能 | 实现方式 |
|------|------|---------|
| **OCR引擎** | 文字识别 | Tesseract / 百度OCR / 腾讯OCR |
| **PDF处理器** | PDF解析、图片提取 | pdftotext, pdftoppm, pdfimages |
| **发票识别器** | 发票分类、字段提取 | 正则表达式 + LLM |
| **LLM模型** | 智能分析、摘要生成 | DeepSeek / Qwen / OpenAI |

## 4. 处理流程

### 4.1 图片OCR流程

```mermaid
sequenceDiagram
    participant Client as **客户端**
    participant API as **API服务**
    participant Agent as **OCR Agent**
    participant OCR as **OCR引擎**
    participant LLM as **LLM**

    Client->>API: **上传图片**
    API->>Agent: **创建OCR任务**
    Agent->>OCR: **执行OCR识别**
    OCR-->>Agent: **返回原始文本**
    
    alt 需要智能处理
        Agent->>LLM: **发送文本进行整理**
        LLM-->>Agent: **返回结构化结果**
    end
    
    Agent-->>API: **返回处理结果**
    API-->>Client: **返回JSON响应**
```

### 4.2 发票识别流程

```mermaid
sequenceDiagram
    participant Client as **客户端**
    participant Agent as **Invoice Agent**
    participant OCR as **OCR引擎**
    participant Recognizer as **发票识别器**
    participant LLM as **LLM**

    Client->>Agent: **上传发票图片**
    Agent->>OCR: **执行OCR**
    OCR-->>Agent: **返回文本**
    
    Agent->>Recognizer: **分类发票类型**
    Recognizer-->>Agent: **返回类型**
    
    Agent->>Recognizer: **提取字段**
    Recognizer-->>Agent: **返回字段值**
    
    opt 需要验证
        Agent->>LLM: **验证数据完整性**
        LLM-->>Agent: **返回验证结果**
    end
    
    Agent-->>Client: **返回结构化发票数据**
```

## 5. 数据模型

### 5.1 OCR结果

```go
type OCRResult struct {
    Text       string      `json:"text"`
    Confidence float64     `json:"confidence"`
    Blocks     []TextBlock `json:"blocks"`
    Language   string      `json:"language"`
    Duration   time.Duration `json:"duration"`
    Metadata   map[string]interface{} `json:"metadata"`
}
```

### 5.2 发票数据

```go
type Invoice struct {
    Type       InvoiceType        `json:"type"`
    Confidence float64            `json:"confidence"`
    Fields     map[string]string  `json:"fields"`
    Items      []InvoiceItem      `json:"items"`
    RawText    string             `json:"raw_text"`
}
```

### 5.3 处理结果

```go
type ProcessResult struct {
    TaskType       TaskType        `json:"task_type"`
    DocumentType   DocumentType    `json:"document_type"`
    Text           string          `json:"text"`
    Summary        string          `json:"summary"`
    StructuredData *StructuredData `json:"structured_data"`
    Duration       time.Duration   `json:"duration"`
    ProcessedAt    time.Time       `json:"processed_at"`
}
```

## 6. 支持的发票类型

| 类型 | 说明 | 提取字段 |
|------|------|---------|
| **增值税普通发票** | vat_invoice | 发票代码、号码、日期、金额、税额等 |
| **增值税专用发票** | vat_special_invoice | 含购销方完整信息 |
| **火车票** | train_ticket | 出发/到达站、日期、座位、金额 |
| **出租车发票** | taxi_receipt | 日期、金额、里程 |
| **机票** | air_ticket | 航班号、出发/到达、日期 |
| **酒店发票** | hotel_invoice | 酒店名、日期、金额 |
| **过路费发票** | toll_invoice | 入/出口、金额 |

## 7. 扩展性设计

### 7.1 OCR引擎插件化

```go
type OCREngine interface {
    Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error)
    RecognizeFile(ctx context.Context, filePath string, opts *RecognizeOptions) (*OCRResult, error)
    Close() error
}
```

### 7.2 LLM提供商适配

```go
type LLMProvider interface {
    Generate(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
    Stream(ctx context.Context, messages []*schema.Message) (*schema.StreamReader, error)
}
```

### 7.3 输出格式扩展

```go
type Formatter interface {
    Format(result *ProcessResult, format OutputFormat) (string, error)
    FormatAndSave(result *ProcessResult, format OutputFormat, filename string) error
}
```

## 8. 部署架构

### 8.1 Kubernetes部署

```
┌─────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                        │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐        │
│  │  Ingress     │   │  Service     │   │  ConfigMap   │        │
│  │  Controller  │   │  (ClusterIP) │   │  (config)    │        │
│  └──────────────┘   └──────────────┘   └──────────────┘        │
│          │                  │                                   │
│          ▼                  ▼                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Deployment                              │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐     │  │
│  │  │ Pod 1   │  │ Pod 2   │  │ Pod 3   │  │ ...     │     │  │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘     │  │
│  └──────────────────────────────────────────────────────────┘  │
│          │                                                      │
│          ▼                                                      │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐        │
│  │  Secret      │   │  PVC         │   │  HPA         │        │
│  │  (API Keys)  │   │  (Storage)   │   │  (AutoScale) │        │
│  └──────────────┘   └──────────────┘   └──────────────┘        │
└─────────────────────────────────────────────────────────────────┘
```

## 9. 性能优化

### 9.1 并发控制

- 使用信号量限制并发OCR任务数
- 配置最大并发数：`ocr.max_concurrency`

### 9.2 缓存策略

- L1缓存：内存LRU缓存，快速响应相同请求
- L2缓存：可选Redis缓存，支持分布式部署

### 9.3 资源限制

- 图片最大尺寸：4096x4096
- 文件最大大小：100MB
- PDF最大页数：100页

## 10. 安全考虑

### 10.1 API安全

- 请求大小限制
- 速率限制
- 文件类型校验

### 10.2 数据安全

- API密钥通过Secret管理
- 临时文件及时清理
- 非root用户运行

### 10.3 网络安全

- NetworkPolicy限制网络访问
- TLS加密传输
- CORS配置
