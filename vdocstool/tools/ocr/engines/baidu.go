// Package engines 百度 OCR 引擎实现
package engines

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

// BaiduOCREngine 百度 OCR 引擎
type BaiduOCREngine struct {
	config       *BaiduOCRConfig
	client       *http.Client
	accessToken  string
	tokenExpiry  time.Time
	mu           sync.RWMutex
	loaded       bool
	lastUsed     time.Time
	totalCalls   int64
	failedCalls  int64
	totalLatency int64
}

// BaiduOCRConfig 百度 OCR 配置
type BaiduOCRConfig struct {
	APIKey      string
	SecretKey   string
	Timeout     time.Duration
	EnableHigh  bool // 是否使用高精度版本
}

// DefaultBaiduOCRConfig 默认配置
func DefaultBaiduOCRConfig() *BaiduOCRConfig {
	return &BaiduOCRConfig{
		Timeout:    30 * time.Second,
		EnableHigh: false,
	}
}

// NewBaiduOCREngine 创建百度 OCR 引擎
func NewBaiduOCREngine(config *BaiduOCRConfig) *BaiduOCREngine {
	if config == nil {
		config = DefaultBaiduOCRConfig()
	}
	return &BaiduOCREngine{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

// Name 返回引擎名称
func (e *BaiduOCREngine) Name() ocr.EngineType {
	return ocr.EngineBaidu
}

// Layer 返回引擎层级
func (e *BaiduOCREngine) Layer() ocr.EngineLayer {
	return ocr.LayerSmallModel // 云服务归类为小模型层
}

// SupportedScenes 支持的场景
func (e *BaiduOCREngine) SupportedScenes() []ocr.SceneType {
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

// getAccessToken 获取 access_token
func (e *BaiduOCREngine) getAccessToken(ctx context.Context) (string, error) {
	e.mu.RLock()
	if e.accessToken != "" && time.Now().Before(e.tokenExpiry) {
		token := e.accessToken
		e.mu.RUnlock()
		return token, nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	// 双重检查
	if e.accessToken != "" && time.Now().Before(e.tokenExpiry) {
		return e.accessToken, nil
	}

	// 获取新 token
	tokenURL := fmt.Sprintf(
		"https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s",
		e.config.APIKey, e.config.SecretKey,
	)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf("get token failed: %s - %s", result.Error, result.ErrorDesc)
	}

	e.accessToken = result.AccessToken
	e.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second) // 提前5分钟过期

	return e.accessToken, nil
}

// Recognize 执行 OCR 识别
func (e *BaiduOCREngine) Recognize(ctx context.Context, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
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

	// 获取 access_token
	token, err := e.getAccessToken(ctx)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("get access token failed: %v", err)
		return result, err
	}

	// 准备图像数据
	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("load image failed: %v", err)
		return result, err
	}

	result.Metadata.ImageWidth = imgData.Width
	result.Metadata.ImageHeight = imgData.Height

	// 转为 base64
	imageBase64 := base64.StdEncoding.EncodeToString(imgData.RawBytes)

	// 选择 API
	apiURL := "https://aip.baidubce.com/rest/2.0/ocr/v1/general_basic"
	if e.config.EnableHigh {
		apiURL = "https://aip.baidubce.com/rest/2.0/ocr/v1/accurate_basic"
	}
	apiURL = fmt.Sprintf("%s?access_token=%s", apiURL, token)

	// 构建请求
	formData := url.Values{}
	formData.Set("image", imageBase64)
	if req.DetectAngle {
		formData.Set("detect_direction", "true")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("create request failed: %v", err)
		return result, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("request failed: %v", err)
		return result, err
	}
	defer resp.Body.Close()

	latency := time.Since(startTime).Milliseconds()
	result.Latency = latency

	e.mu.Lock()
	e.totalLatency += latency
	e.mu.Unlock()

	// 解析响应
	var ocrResp struct {
		WordsResultNum int `json:"words_result_num"`
		WordsResult    []struct {
			Words string `json:"words"`
		} `json:"words_result"`
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ocrResp); err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("decode response failed: %v", err)
		return result, err
	}

	if ocrResp.ErrorCode != 0 {
		e.recordFailure()
		result.Error = fmt.Sprintf("baidu ocr error: %d - %s", ocrResp.ErrorCode, ocrResp.ErrorMsg)
		return result, fmt.Errorf("baidu ocr: %s", result.Error)
	}

	// 构建结果
	result.Success = true
	var fullText string

	for _, item := range ocrResp.WordsResult {
		if fullText != "" {
			fullText += "\n"
		}
		fullText += item.Words

		result.Blocks = append(result.Blocks, ocr.TextBlock{
			Text:      item.Words,
			BlockType: "text",
		})
	}

	result.FullText = fullText

	return result, nil
}

// RecognizeTable 表格识别
func (e *BaiduOCREngine) RecognizeTable(ctx context.Context, req *ocr.OCRRequest) (*ocr.TableResult, error) {
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

	token, err := e.getAccessToken(ctx)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imgData.RawBytes)

	// 表格识别 API
	apiURL := fmt.Sprintf("https://aip.baidubce.com/rest/2.0/ocr/v1/table?access_token=%s", token)

	formData := url.Values{}
	formData.Set("image", imageBase64)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		e.recordFailure()
		return result, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		e.recordFailure()
		return result, err
	}
	defer resp.Body.Close()

	result.Latency = time.Since(startTime).Milliseconds()

	var tableResp struct {
		TablesResult []struct {
			Body []struct {
				CellLocation []struct {
					X int `json:"x"`
					Y int `json:"y"`
				} `json:"cell_location"`
				Words string `json:"words"`
				RowStart int `json:"row_start"`
				RowEnd   int `json:"row_end"`
				ColStart int `json:"col_start"`
				ColEnd   int `json:"col_end"`
			} `json:"body"`
		} `json:"tables_result"`
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tableResp); err != nil {
		e.recordFailure()
		return result, err
	}

	if tableResp.ErrorCode != 0 {
		e.recordFailure()
		result.Error = tableResp.ErrorMsg
		return result, fmt.Errorf("table ocr error: %s", tableResp.ErrorMsg)
	}

	result.Success = true

	// 转换表格数据
	for _, t := range tableResp.TablesResult {
		// 找出最大行列
		maxRow, maxCol := 0, 0
		for _, cell := range t.Body {
			if cell.RowEnd > maxRow {
				maxRow = cell.RowEnd
			}
			if cell.ColEnd > maxCol {
				maxCol = cell.ColEnd
			}
		}

		// 创建表格
		rows := make([][]string, maxRow+1)
		for i := range rows {
			rows[i] = make([]string, maxCol+1)
		}

		for _, cell := range t.Body {
			rows[cell.RowStart][cell.ColStart] = cell.Words
		}

		table := ocr.Table{
			Rows: rows,
		}
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
func (e *BaiduOCREngine) RecognizeInvoice(ctx context.Context, req *ocr.OCRRequest) (*ocr.InvoiceResult, error) {
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

	token, err := e.getAccessToken(ctx)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imgData.RawBytes)

	// 增值税发票识别 API
	apiURL := fmt.Sprintf("https://aip.baidubce.com/rest/2.0/ocr/v1/vat_invoice?access_token=%s", token)

	formData := url.Values{}
	formData.Set("image", imageBase64)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		e.recordFailure()
		return result, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		e.recordFailure()
		return result, err
	}
	defer resp.Body.Close()

	result.Latency = time.Since(startTime).Milliseconds()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var invoiceResp struct {
		WordsResult struct {
			InvoiceCode      string `json:"InvoiceCode"`
			InvoiceNum       string `json:"InvoiceNum"`
			InvoiceDate      string `json:"InvoiceDate"`
			TotalAmount      string `json:"TotalAmount"`
			TotalTax         string `json:"TotalTax"`
			AmountInFiguers  string `json:"AmountInFiguers"`
			SellerName       string `json:"SellerName"`
			PurchaserName    string `json:"PurchaserName"`
			CommodityName    []struct {
				Word string `json:"word"`
			} `json:"CommodityName"`
		} `json:"words_result"`
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
	}

	if err := json.Unmarshal(bodyBytes, &invoiceResp); err != nil {
		e.recordFailure()
		return result, err
	}

	if invoiceResp.ErrorCode != 0 {
		e.recordFailure()
		result.Error = invoiceResp.ErrorMsg
		return result, fmt.Errorf("invoice ocr error: %s", invoiceResp.ErrorMsg)
	}

	result.Success = true
	result.InvoiceType = "vat_invoice"
	result.InvoiceCode = invoiceResp.WordsResult.InvoiceCode
	result.InvoiceNo = invoiceResp.WordsResult.InvoiceNum
	result.Date = invoiceResp.WordsResult.InvoiceDate
	result.TotalAmount = invoiceResp.WordsResult.AmountInFiguers
	result.TaxAmount = invoiceResp.WordsResult.TotalTax
	result.SellerName = invoiceResp.WordsResult.SellerName
	result.BuyerName = invoiceResp.WordsResult.PurchaserName

	return result, nil
}

// RecognizeIDCard 身份证识别
func (e *BaiduOCREngine) RecognizeIDCard(ctx context.Context, req *ocr.OCRRequest) (*ocr.IDCardResult, error) {
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

	token, err := e.getAccessToken(ctx)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		return result, err
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imgData.RawBytes)

	// 身份证识别 API
	apiURL := fmt.Sprintf("https://aip.baidubce.com/rest/2.0/ocr/v1/idcard?access_token=%s", token)

	formData := url.Values{}
	formData.Set("image", imageBase64)
	formData.Set("id_card_side", "front") // front/back

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		e.recordFailure()
		return result, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		e.recordFailure()
		return result, err
	}
	defer resp.Body.Close()

	result.Latency = time.Since(startTime).Milliseconds()

	var idcardResp struct {
		WordsResult struct {
			Name struct {
				Words string `json:"words"`
			} `json:"姓名"`
			Sex struct {
				Words string `json:"words"`
			} `json:"性别"`
			Nation struct {
				Words string `json:"words"`
			} `json:"民族"`
			Birthday struct {
				Words string `json:"words"`
			} `json:"出生"`
			Address struct {
				Words string `json:"words"`
			} `json:"住址"`
			IDNumber struct {
				Words string `json:"words"`
			} `json:"公民身份号码"`
		} `json:"words_result"`
		ErrorCode int    `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&idcardResp); err != nil {
		e.recordFailure()
		return result, err
	}

	if idcardResp.ErrorCode != 0 {
		e.recordFailure()
		result.Error = idcardResp.ErrorMsg
		return result, fmt.Errorf("idcard ocr error: %s", idcardResp.ErrorMsg)
	}

	result.Success = true
	result.CardType = "id_card"
	result.Name = idcardResp.WordsResult.Name.Words
	result.Gender = idcardResp.WordsResult.Sex.Words
	result.Ethnicity = idcardResp.WordsResult.Nation.Words
	result.Birthday = idcardResp.WordsResult.Birthday.Words
	result.Address = idcardResp.WordsResult.Address.Words
	result.IDNumber = idcardResp.WordsResult.IDNumber.Words

	return result, nil
}

// MemoryUsage 返回内存占用
func (e *BaiduOCREngine) MemoryUsage() int {
	return 10 // 云服务不占用本地内存
}

// IsLoaded 是否已加载
func (e *BaiduOCREngine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// Load 加载引擎
func (e *BaiduOCREngine) Load(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.APIKey == "" || e.config.SecretKey == "" {
		return fmt.Errorf("baidu ocr api key or secret key not configured")
	}

	e.loaded = true
	return nil
}

// Unload 卸载引擎
func (e *BaiduOCREngine) Unload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaded = false
	e.accessToken = ""
	return nil
}

// Health 健康检查
func (e *BaiduOCREngine) Health(ctx context.Context) error {
	if e.config.APIKey == "" || e.config.SecretKey == "" {
		return fmt.Errorf("api key not configured")
	}
	_, err := e.getAccessToken(ctx)
	return err
}

func (e *BaiduOCREngine) recordFailure() {
	e.mu.Lock()
	e.failedCalls++
	e.mu.Unlock()
}

// Stats 获取统计信息
func (e *BaiduOCREngine) Stats() ocr.EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	avgLatency := float64(0)
	if e.totalCalls > 0 {
		avgLatency = float64(e.totalLatency) / float64(e.totalCalls)
	}

	return ocr.EngineStatus{
		Engine:      e.Name(),
		Available:   e.config.APIKey != "",
		Loaded:      e.loaded,
		MemoryMB:    e.MemoryUsage(),
		LastUsed:    e.lastUsed,
		TotalCalls:  e.totalCalls,
		FailedCalls: e.failedCalls,
		AvgLatency:  avgLatency,
	}
}

// 确保实现了接口
var _ ocr.Engine = (*BaiduOCREngine)(nil)
