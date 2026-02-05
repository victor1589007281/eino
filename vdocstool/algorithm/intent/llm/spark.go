// Package llm 讯飞星火客户端
package llm

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
	"github.com/gorilla/websocket"
)

// SparkClient 讯飞星火客户端
type SparkClient struct {
	config *types.LLMEngineConfig
	appID  string
}

// NewSparkClient 创建讯飞星火客户端
func NewSparkClient(cfg *types.LLMEngineConfig) (*SparkClient, error) {
	// APIKey 格式: appid:apikey:apisecret
	parts := strings.Split(cfg.APIKey, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid api key format, expected 'appid:apikey:apisecret'")
	}

	return &SparkClient{
		config: cfg,
		appID:  parts[0],
	}, nil
}

// getHostURL 根据模型获取 WebSocket URL
func (c *SparkClient) getHostURL() string {
	model := c.config.Model
	switch model {
	case "spark-v4.0", "spark-4.0", "4.0":
		return "wss://spark-api.xf-yun.com/v4.0/chat"
	case "spark-v3.5", "spark-3.5", "3.5":
		return "wss://spark-api.xf-yun.com/v3.5/chat"
	case "spark-v3.1", "spark-3.1", "3.1":
		return "wss://spark-api.xf-yun.com/v3.1/chat"
	case "spark-v2.1", "spark-2.1", "2.1":
		return "wss://spark-api.xf-yun.com/v2.1/chat"
	case "spark-lite", "lite":
		return "wss://spark-api.xf-yun.com/v1.1/chat"
	default:
		return "wss://spark-api.xf-yun.com/v3.5/chat"
	}
}

// getDomain 根据模型获取 domain
func (c *SparkClient) getDomain() string {
	model := c.config.Model
	switch model {
	case "spark-v4.0", "spark-4.0", "4.0":
		return "4.0Ultra"
	case "spark-v3.5", "spark-3.5", "3.5":
		return "generalv3.5"
	case "spark-v3.1", "spark-3.1", "3.1":
		return "generalv3"
	case "spark-v2.1", "spark-2.1", "2.1":
		return "generalv2"
	case "spark-lite", "lite":
		return "lite"
	default:
		return "generalv3.5"
	}
}

// generateAuthURL 生成鉴权 URL
func (c *SparkClient) generateAuthURL() (string, error) {
	parts := strings.Split(c.config.APIKey, ":")
	apiKey := parts[1]
	apiSecret := parts[2]

	hostURL := c.getHostURL()
	u, err := url.Parse(hostURL)
	if err != nil {
		return "", err
	}

	date := time.Now().UTC().Format(time.RFC1123)

	// 构建签名原文
	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET %s HTTP/1.1", u.Host, date, u.Path)

	// HMAC-SHA256 签名
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 构建 authorization
	authorizationOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`,
		apiKey, signature)
	authorization := base64.StdEncoding.EncodeToString([]byte(authorizationOrigin))

	// 构建 URL
	v := url.Values{}
	v.Set("authorization", authorization)
	v.Set("date", date)
	v.Set("host", u.Host)

	return hostURL + "?" + v.Encode(), nil
}

// 讯飞星火 WebSocket 请求/响应结构
type sparkRequest struct {
	Header    sparkReqHeader    `json:"header"`
	Parameter sparkReqParameter `json:"parameter"`
	Payload   sparkReqPayload   `json:"payload"`
}

type sparkReqHeader struct {
	AppID string `json:"app_id"`
	UID   string `json:"uid,omitempty"`
}

type sparkReqParameter struct {
	Chat sparkChatParam `json:"chat"`
}

type sparkChatParam struct {
	Domain      string  `json:"domain"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
}

type sparkReqPayload struct {
	Message sparkReqMessage `json:"message"`
}

type sparkReqMessage struct {
	Text []sparkMessage `json:"text"`
}

type sparkMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type sparkResponse struct {
	Header struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		SID     string `json:"sid"`
		Status  int    `json:"status"`
	} `json:"header"`
	Payload struct {
		Choices struct {
			Status int            `json:"status"`
			Seq    int            `json:"seq"`
			Text   []sparkMessage `json:"text"`
		} `json:"choices"`
		Usage struct {
			Text struct {
				QuestionTokens   int `json:"question_tokens"`
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"text"`
		} `json:"usage"`
	} `json:"payload"`
}

// Complete 完成请求
func (c *SparkClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// 生成鉴权 URL
	authURL, err := c.generateAuthURL()
	if err != nil {
		return nil, fmt.Errorf("generate auth url: %w", err)
	}

	// 建立 WebSocket 连接
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %w", err)
	}
	defer conn.Close()

	// 构建请求
	messages := make([]sparkMessage, len(req.Messages))
	for i, m := range req.Messages {
		role := m.Role
		if role == "system" {
			role = "user" // 星火不支持 system role，转为 user
		}
		messages[i] = sparkMessage{
			Role:    role,
			Content: m.Content,
		}
	}

	sparkReq := sparkRequest{
		Header: sparkReqHeader{
			AppID: c.appID,
		},
		Parameter: sparkReqParameter{
			Chat: sparkChatParam{
				Domain:      c.getDomain(),
				Temperature: req.Temperature,
				MaxTokens:   req.MaxTokens,
			},
		},
		Payload: sparkReqPayload{
			Message: sparkReqMessage{
				Text: messages,
			},
		},
	}

	// 发送请求
	if err := conn.WriteJSON(sparkReq); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// 接收响应
	var fullText strings.Builder
	var usage *Usage
	var wg sync.WaitGroup
	var readErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			var resp sparkResponse
			if err := conn.ReadJSON(&resp); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					readErr = err
				}
				return
			}

			if resp.Header.Code != 0 {
				readErr = fmt.Errorf("spark error: %d - %s", resp.Header.Code, resp.Header.Message)
				return
			}

			// 累积文本
			for _, text := range resp.Payload.Choices.Text {
				fullText.WriteString(text.Content)
			}

			// 更新 usage
			if resp.Payload.Usage.Text.TotalTokens > 0 {
				usage = &Usage{
					PromptTokens:     resp.Payload.Usage.Text.PromptTokens,
					CompletionTokens: resp.Payload.Usage.Text.CompletionTokens,
					TotalTokens:      resp.Payload.Usage.Text.TotalTokens,
				}
			}

			// 检查是否结束
			if resp.Header.Status == 2 {
				return
			}
		}
	}()

	wg.Wait()

	if readErr != nil {
		return nil, readErr
	}

	return &CompletionResponse{
		Text:         fullText.String(),
		FinishReason: "stop",
		Usage:        usage,
	}, nil
}

// HealthCheck 健康检查
func (c *SparkClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err := c.Complete(ctx, &CompletionRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		MaxTokens:   5,
		Temperature: 0,
	})

	return err
}

// Close 关闭
func (c *SparkClient) Close() error {
	return nil
}
