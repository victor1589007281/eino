// Package engines 腾讯 OCR 引擎实现
package engines

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

// TencentOCREngine 腾讯 OCR 引擎
type TencentOCREngine struct {
	config       *TencentOCRConfig
	client       *http.Client
	mu           sync.RWMutex
	loaded       bool
	lastUsed     time.Time
	totalCalls   int64
	failedCalls  int64
	totalLatency int64
}

// TencentOCRConfig 腾讯 OCR 配置
type TencentOCRConfig struct {
	SecretID  string
	SecretKey string
	Region    string
	Timeout   time.Duration
}

// DefaultTencentOCRConfig 默认配置
func DefaultTencentOCRConfig() *TencentOCRConfig {
	return &TencentOCRConfig{
		Region:  "ap-guangzhou",
		Timeout: 30 * time.Second,
	}
}

// NewTencentOCREngine 创建腾讯 OCR 引擎
func NewTencentOCREngine(config *TencentOCRConfig) *TencentOCREngine {
	if config == nil {
		config = DefaultTencentOCRConfig()
	}
	return &TencentOCREngine{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

// Name 返回引擎名称
func (e *TencentOCREngine) Name() ocr.EngineType {
	return ocr.EngineTencent
}

// Layer 返回引擎层级
func (e *TencentOCREngine) Layer() ocr.EngineLayer {
	return ocr.LayerSmallModel
}

// SupportedScenes 支持的场景
func (e *TencentOCREngine) SupportedScenes() []ocr.SceneType {
	return []ocr.SceneType{
		ocr.SceneGeneral,
		ocr.ScenePrint,
		ocr.SceneDocument,
		ocr.SceneTable,
		ocr.SceneInvoice,
		ocr.SceneIDCard,
		ocr.SceneHandwriting,
	}
}

// Recognize 执行 OCR 识别
func (e *TencentOCREngine) Recognize(ctx context.Context, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
	e.mu.Lock()
	e.totalCalls++
	e.lastUsed = time.Now()
	e.mu.Unlock()

	startTime := time.Now()
	result := &ocr.OCRResult{
		Engine: e.Name(),
		Metadata: &ocr.OCRMeta{
			ProcessTime: startTime,
		},
	}

	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("load image failed: %v", err)
		return result, err
	}

	result.Metadata.ImageWidth = imgData.Width
	result.Metadata.ImageHeight = imgData.Height

	imageBase64 := base64.StdEncoding.EncodeToString(imgData.RawBytes)

	// 调用通用文字识别 API
	action := "GeneralBasicOCR"
	payload := map[string]interface{}{
		"ImageBase64": imageBase64,
	}

	respBody, err := e.callAPI(ctx, action, payload)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("api call failed: %v", err)
		return result, err
	}

	latency := time.Since(startTime).Milliseconds()
	result.Latency = latency

	e.mu.Lock()
	e.totalLatency += latency
	e.mu.Unlock()

	// 解析响应
	var ocrResp struct {
		Response struct {
			TextDetections []struct {
				DetectedText string  `json:"DetectedText"`
				Confidence   float64 `json:"Confidence"`
				Polygon      []struct {
					X int `json:"X"`
					Y int `json:"Y"`
				} `json:"Polygon"`
			} `json:"TextDetections"`
			Error struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}

	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("decode response failed: %v", err)
		return result, err
	}

	if ocrResp.Response.Error.Code != "" {
		e.recordFailure()
		result.Error = fmt.Sprintf("tencent ocr error: %s - %s",
			ocrResp.Response.Error.Code, ocrResp.Response.Error.Message)
		return result, fmt.Errorf("tencent ocr: %s", result.Error)
	}

	// 构建结果
	result.Success = true
	var fullText string
	var totalConf float64

	for _, item := range ocrResp.Response.TextDetections {
		if fullText != "" {
			fullText += "\n"
		}
		fullText += item.DetectedText
		totalConf += item.Confidence

		block := ocr.TextBlock{
			Text:       item.DetectedText,
			Confidence: item.Confidence / 100.0, // 腾讯返回的是百分比
			BlockType:  "text",
		}

		if len(item.Polygon) >= 4 && req.ReturnPosition {
			minX, minY := item.Polygon[0].X, item.Polygon[0].Y
			maxX, maxY := item.Polygon[0].X, item.Polygon[0].Y
			for _, pt := range item.Polygon {
				if pt.X < minX {
					minX = pt.X
				}
				if pt.X > maxX {
					maxX = pt.X
				}
				if pt.Y < minY {
					minY = pt.Y
				}
				if pt.Y > maxY {
					maxY = pt.Y
				}
			}
			block.Box = &ocr.BoundingBox{
				X:      minX,
				Y:      minY,
				Width:  maxX - minX,
				Height: maxY - minY,
			}
		}

		result.Blocks = append(result.Blocks, block)
	}

	result.FullText = fullText
	if len(ocrResp.Response.TextDetections) > 0 && req.ReturnConfidence {
		result.Confidence = totalConf / float64(len(ocrResp.Response.TextDetections)) / 100.0
	}

	return result, nil
}

// callAPI 调用腾讯云 API
func (e *TencentOCREngine) callAPI(ctx context.Context, action string, payload map[string]interface{}) ([]byte, error) {
	host := "ocr.tencentcloudapi.com"
	service := "ocr"
	version := "2018-11-19"

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// 构建规范请求
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:application/json\nhost:%s\nx-tc-action:%s\n",
		host, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"

	hashedPayload := sha256Hex(payloadBytes)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod, canonicalURI, canonicalQueryString,
		canonicalHeaders, signedHeaders, hashedPayload)

	// 构建签名字符串
	algorithm := "TC3-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := sha256Hex([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm, timestamp, credentialScope, hashedCanonicalRequest)

	// 计算签名
	secretDate := hmacSHA256([]byte("TC3"+e.config.SecretKey), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(service))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))

	// 构建 Authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, e.config.SecretID, credentialScope, signedHeaders, signature)

	// 发送请求
	url := fmt.Sprintf("https://%s", host)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Region", e.config.Region)
	req.Header.Set("Authorization", authorization)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// RecognizeTable 表格识别
func (e *TencentOCREngine) RecognizeTable(ctx context.Context, req *ocr.OCRRequest) (*ocr.TableResult, error) {
	e.mu.Lock()
	e.totalCalls++
	e.lastUsed = time.Now()
	e.mu.Unlock()

	startTime := time.Now()
	result := &ocr.TableResult{
		OCRResult: ocr.OCRResult{
			Engine: e.Name(),
			Metadata: &ocr.OCRMeta{
				ProcessTime: startTime,
			},
		},
	}

	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imgData.RawBytes)

	payload := map[string]interface{}{
		"ImageBase64": imageBase64,
	}

	respBody, err := e.callAPI(ctx, "TableOCR", payload)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	result.Latency = time.Since(startTime).Milliseconds()

	var tableResp struct {
		Response struct {
			TableDetections []struct {
				Cells []struct {
					Text     string `json:"Text"`
					RowStart int    `json:"RowTl"`
					ColStart int    `json:"ColTl"`
				} `json:"Cells"`
			} `json:"TableDetections"`
			Error struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}

	if err := json.Unmarshal(respBody, &tableResp); err != nil {
		e.recordFailure()
		return result, err
	}

	if tableResp.Response.Error.Code != "" {
		e.recordFailure()
		return result, fmt.Errorf("table ocr error: %s", tableResp.Response.Error.Message)
	}

	result.Success = true

	for _, t := range tableResp.Response.TableDetections {
		maxRow, maxCol := 0, 0
		for _, cell := range t.Cells {
			if cell.RowStart > maxRow {
				maxRow = cell.RowStart
			}
			if cell.ColStart > maxCol {
				maxCol = cell.ColStart
			}
		}

		rows := make([][]string, maxRow+1)
		for i := range rows {
			rows[i] = make([]string, maxCol+1)
		}

		for _, cell := range t.Cells {
			rows[cell.RowStart][cell.ColStart] = cell.Text
		}

		table := ocr.Table{Rows: rows}
		if len(rows) > 0 {
			table.Headers = rows[0]
			if len(rows) > 1 {
				table.Rows = rows[1:]
			}
		}
		result.Tables = append(result.Tables, table)
	}

	return result, nil
}

// RecognizeInvoice 票据识别
func (e *TencentOCREngine) RecognizeInvoice(ctx context.Context, req *ocr.OCRRequest) (*ocr.InvoiceResult, error) {
	e.mu.Lock()
	e.totalCalls++
	e.lastUsed = time.Now()
	e.mu.Unlock()

	startTime := time.Now()
	result := &ocr.InvoiceResult{
		OCRResult: ocr.OCRResult{
			Engine: e.Name(),
			Metadata: &ocr.OCRMeta{
				ProcessTime: startTime,
			},
		},
	}

	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imgData.RawBytes)

	payload := map[string]interface{}{
		"ImageBase64": imageBase64,
	}

	respBody, err := e.callAPI(ctx, "VatInvoiceOCR", payload)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	result.Latency = time.Since(startTime).Milliseconds()

	var invoiceResp struct {
		Response struct {
			VatInvoiceInfos []struct {
				Name  string `json:"Name"`
				Value string `json:"Value"`
			} `json:"VatInvoiceInfos"`
			Error struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}

	if err := json.Unmarshal(respBody, &invoiceResp); err != nil {
		e.recordFailure()
		return result, err
	}

	if invoiceResp.Response.Error.Code != "" {
		e.recordFailure()
		return result, fmt.Errorf("invoice ocr error: %s", invoiceResp.Response.Error.Message)
	}

	result.Success = true
	result.InvoiceType = "vat_invoice"
	result.ExtraFields = make(map[string]string)

	for _, info := range invoiceResp.Response.VatInvoiceInfos {
		switch info.Name {
		case "发票代码":
			result.InvoiceCode = info.Value
		case "发票号码":
			result.InvoiceNo = info.Value
		case "开票日期":
			result.Date = info.Value
		case "价税合计(小写)":
			result.TotalAmount = info.Value
		case "税额":
			result.TaxAmount = info.Value
		case "销售方名称":
			result.SellerName = info.Value
		case "购买方名称":
			result.BuyerName = info.Value
		default:
			result.ExtraFields[info.Name] = info.Value
		}
	}

	return result, nil
}

// RecognizeIDCard 身份证识别
func (e *TencentOCREngine) RecognizeIDCard(ctx context.Context, req *ocr.OCRRequest) (*ocr.IDCardResult, error) {
	e.mu.Lock()
	e.totalCalls++
	e.lastUsed = time.Now()
	e.mu.Unlock()

	startTime := time.Now()
	result := &ocr.IDCardResult{
		OCRResult: ocr.OCRResult{
			Engine: e.Name(),
			Metadata: &ocr.OCRMeta{
				ProcessTime: startTime,
			},
		},
	}

	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imgData.RawBytes)

	payload := map[string]interface{}{
		"ImageBase64": imageBase64,
		"CardSide":    "FRONT",
	}

	respBody, err := e.callAPI(ctx, "IDCardOCR", payload)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	result.Latency = time.Since(startTime).Milliseconds()

	var idcardResp struct {
		Response struct {
			Name      string `json:"Name"`
			Sex       string `json:"Sex"`
			Nation    string `json:"Nation"`
			Birth     string `json:"Birth"`
			Address   string `json:"Address"`
			IdNum     string `json:"IdNum"`
			Authority string `json:"Authority"`
			ValidDate string `json:"ValidDate"`
			Error     struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}

	if err := json.Unmarshal(respBody, &idcardResp); err != nil {
		e.recordFailure()
		return result, err
	}

	if idcardResp.Response.Error.Code != "" {
		e.recordFailure()
		return result, fmt.Errorf("idcard ocr error: %s", idcardResp.Response.Error.Message)
	}

	result.Success = true
	result.CardType = "id_card"
	result.Name = idcardResp.Response.Name
	result.Gender = idcardResp.Response.Sex
	result.Ethnicity = idcardResp.Response.Nation
	result.Birthday = idcardResp.Response.Birth
	result.Address = idcardResp.Response.Address
	result.IDNumber = idcardResp.Response.IdNum
	result.Authority = idcardResp.Response.Authority
	result.ExpiryDate = idcardResp.Response.ValidDate

	return result, nil
}

// MemoryUsage 返回内存占用
func (e *TencentOCREngine) MemoryUsage() int {
	return 10 // 云服务不占用本地内存
}

// IsLoaded 是否已加载
func (e *TencentOCREngine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// Load 加载引擎
func (e *TencentOCREngine) Load(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.SecretID == "" || e.config.SecretKey == "" {
		return fmt.Errorf("tencent ocr secret id or secret key not configured")
	}

	e.loaded = true
	return nil
}

// Unload 卸载引擎
func (e *TencentOCREngine) Unload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaded = false
	return nil
}

// Health 健康检查
func (e *TencentOCREngine) Health(ctx context.Context) error {
	if e.config.SecretID == "" || e.config.SecretKey == "" {
		return fmt.Errorf("api key not configured")
	}
	return nil
}

func (e *TencentOCREngine) recordFailure() {
	e.mu.Lock()
	e.failedCalls++
	e.mu.Unlock()
}

// Stats 获取统计信息
func (e *TencentOCREngine) Stats() ocr.EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	avgLatency := float64(0)
	if e.totalCalls > 0 {
		avgLatency = float64(e.totalLatency) / float64(e.totalCalls)
	}

	return ocr.EngineStatus{
		Engine:      e.Name(),
		Available:   e.config.SecretID != "",
		Loaded:      e.loaded,
		MemoryMB:    e.MemoryUsage(),
		LastUsed:    e.lastUsed,
		TotalCalls:  e.totalCalls,
		FailedCalls: e.failedCalls,
		AvgLatency:  avgLatency,
	}
}

// 辅助函数
func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

var _ ocr.Engine = (*TencentOCREngine)(nil)
