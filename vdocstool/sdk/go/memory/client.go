// Package memory 提供Memory服务的Go SDK
package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/cloudwego/eino/vdocstool/tools/memory/grpc/pb"
)

// ClientConfig 客户端配置
type ClientConfig struct {
	// Protocol 协议类型: "grpc" 或 "rest"
	Protocol string `json:"protocol"`
	// Address 服务地址
	Address string `json:"address"`
	// APIKey API密钥
	APIKey string `json:"api_key"`
	// Timeout 请求超时时间
	Timeout time.Duration `json:"timeout"`
	// MaxRetries 最大重试次数
	MaxRetries int `json:"max_retries"`
}

// DefaultClientConfig 返回默认配置
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Protocol:   "grpc",
		Address:    "localhost:50051",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
}

// Client Memory客户端接口
type Client interface {
	// Store 存储消息
	Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error)
	// Retrieve 检索上下文
	Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error)
	// BatchStore 批量存储
	BatchStore(ctx context.Context, req *BatchStoreRequest) (*BatchStoreResponse, error)
	// BatchRetrieve 批量检索
	BatchRetrieve(ctx context.Context, req *BatchRetrieveRequest) (*BatchRetrieveResponse, error)
	// GetSession 获取会话信息
	GetSession(ctx context.Context, sessionID string) (*SessionInfo, error)
	// DeleteSession 删除会话
	DeleteSession(ctx context.Context, sessionID string) error
	// SwitchTopic 切换主题
	SwitchTopic(ctx context.Context, sessionID, newTopic string) (*SwitchTopicResponse, error)
	// RecallTopic 召回主题
	RecallTopic(ctx context.Context, sessionID, topicQuery string) (*RecallTopicResponse, error)
	// ListTopics 列出主题
	ListTopics(ctx context.Context, sessionID string) ([]*TopicInfo, error)
	// GetEntityRelations 获取实体关系
	GetEntityRelations(ctx context.Context, entityName string, depth int) (*EntityRelationsResponse, error)
	// Summarize 生成摘要
	Summarize(ctx context.Context, sessionID string) (*SummaryResponse, error)
	// Archive 归档会话
	Archive(ctx context.Context, req *ArchiveRequest) (*ArchiveResponse, error)
	// GetStats 获取统计
	GetStats(ctx context.Context) (*StatsResponse, error)
	// Health 健康检查
	Health(ctx context.Context) (*HealthResponse, error)
	// Close 关闭客户端
	Close() error
}

// NewClient 创建客户端
func NewClient(config *ClientConfig) (Client, error) {
	if config == nil {
		config = DefaultClientConfig()
	}

	switch config.Protocol {
	case "grpc":
		return newGRPCClient(config)
	case "rest":
		return newRESTClient(config)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}
}

// ==================== gRPC Client ====================

type grpcClient struct {
	config *ClientConfig
	conn   *grpc.ClientConn
	client pb.MemoryServiceClient
}

func newGRPCClient(config *ClientConfig) (*grpcClient, error) {
	conn, err := grpc.NewClient(config.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	return &grpcClient{
		config: config,
		conn:   conn,
		client: pb.NewMemoryServiceClient(conn),
	}, nil
}

func (c *grpcClient) contextWithAuth(ctx context.Context) context.Context {
	if c.config.APIKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", c.config.APIKey)
	}
	return ctx
}

func (c *grpcClient) Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error) {
	ctx = c.contextWithAuth(ctx)

	pbReq := &pb.StoreRequest{
		SessionId: req.SessionID,
		Source:    req.Source,
		Metadata:  req.Metadata,
	}

	if req.Message != nil {
		pbReq.Message = &pb.MessageInput{
			Role:    req.Message.Role,
			Content: req.Message.Content,
			TopicId: req.Message.TopicID,
		}
		if req.Message.Timestamp != nil {
			pbReq.Message.Timestamp = req.Message.Timestamp.Unix()
		}
	}

	if req.Options != nil {
		pbReq.Options = &pb.StoreOptions{
			ExtractEntities:   req.Options.ExtractEntities,
			GenerateEmbedding: req.Options.GenerateEmbedding,
			Importance:        req.Options.Importance,
		}
	}

	resp, err := c.client.Store(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &StoreResponse{
		MessageID:         resp.MessageId,
		Tier:              resp.Tier,
		TokenCount:        int(resp.TokenCount),
		EntitiesExtracted: resp.EntitiesExtracted,
		ArchiveTriggered:  resp.ArchiveTriggered,
	}, nil
}

func (c *grpcClient) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	ctx = c.contextWithAuth(ctx)

	pbReq := &pb.RetrieveRequest{
		SessionId: req.SessionID,
		Query:     req.Query,
	}

	if req.Options != nil {
		pbReq.Options = &pb.RetrieveOptions{
			TokenBudget:     int32(req.Options.TokenBudget),
			TopicId:         req.Options.TopicID,
			Filters:         req.Options.Filters,
			IncludeSummary:  req.Options.IncludeSummary,
			IncludeEntities: req.Options.IncludeEntities,
		}
		if req.Options.TimeRange != nil {
			pbReq.Options.TimeRange = &pb.TimeRange{
				Start: req.Options.TimeRange.Start.Unix(),
				End:   req.Options.TimeRange.End.Unix(),
			}
		}
	}

	resp, err := c.client.Retrieve(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	result := &RetrieveResponse{
		Summary:     resp.Summary,
		TotalTokens: int(resp.TotalTokens),
	}

	for _, item := range resp.Context {
		result.Context = append(result.Context, &ContextItem{
			MessageID:      item.MessageId,
			Role:           item.Role,
			Content:        item.Content,
			Timestamp:      time.Unix(item.Timestamp, 0),
			RelevanceScore: item.RelevanceScore,
			Tier:           item.Tier,
		})
	}

	for _, entity := range resp.Entities {
		result.Entities = append(result.Entities, &EntityInfo{
			Name:     entity.Name,
			Type:     entity.Type,
			Mentions: int(entity.Mentions),
		})
	}

	if resp.SearchStats != nil {
		result.SearchStats = &SearchStats{
			L1Hits:          int(resp.SearchStats.L1Hits),
			L2Hits:          int(resp.SearchStats.L2Hits),
			L3Hits:          int(resp.SearchStats.L3Hits),
			SearchLatencyMs: resp.SearchStats.SearchLatencyMs,
		}
	}

	return result, nil
}

func (c *grpcClient) BatchStore(ctx context.Context, req *BatchStoreRequest) (*BatchStoreResponse, error) {
	ctx = c.contextWithAuth(ctx)

	pbReq := &pb.BatchStoreRequest{
		SessionId: req.SessionID,
	}

	for _, msg := range req.Messages {
		pbMsg := &pb.MessageInput{
			Role:    msg.Role,
			Content: msg.Content,
			TopicId: msg.TopicID,
		}
		if msg.Timestamp != nil {
			pbMsg.Timestamp = msg.Timestamp.Unix()
		}
		pbReq.Messages = append(pbReq.Messages, pbMsg)
	}

	if req.Options != nil {
		pbReq.Options = &pb.BatchStoreOptions{
			PreserveOrder:      req.Options.PreserveOrder,
			ParallelProcessing: req.Options.ParallelProcessing,
		}
	}

	resp, err := c.client.BatchStore(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	result := &BatchStoreResponse{
		SuccessCount: int(resp.SuccessCount),
		FailedCount:  int(resp.FailedCount),
		TotalTokens:  int(resp.TotalTokens),
	}

	for _, r := range resp.Results {
		result.Results = append(result.Results, &StoreResponse{
			MessageID:         r.MessageId,
			Tier:              r.Tier,
			TokenCount:        int(r.TokenCount),
			EntitiesExtracted: r.EntitiesExtracted,
			ArchiveTriggered:  r.ArchiveTriggered,
		})
	}

	return result, nil
}

func (c *grpcClient) BatchRetrieve(ctx context.Context, req *BatchRetrieveRequest) (*BatchRetrieveResponse, error) {
	ctx = c.contextWithAuth(ctx)

	pbReq := &pb.BatchRetrieveRequest{}

	for _, r := range req.Requests {
		pbReq.Requests = append(pbReq.Requests, &pb.SingleRetrieveRequest{
			SessionId: r.SessionID,
			Query:     r.Query,
		})
	}

	if req.Options != nil {
		pbReq.Options = &pb.BatchRetrieveOptions{
			Parallel:  req.Options.Parallel,
			TimeoutMs: int32(req.Options.TimeoutMs),
		}
	}

	resp, err := c.client.BatchRetrieve(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	result := &BatchRetrieveResponse{}

	for _, r := range resp.Results {
		rr := &RetrieveResponse{
			Summary:     r.Summary,
			TotalTokens: int(r.TotalTokens),
		}
		for _, item := range r.Context {
			rr.Context = append(rr.Context, &ContextItem{
				MessageID:      item.MessageId,
				Role:           item.Role,
				Content:        item.Content,
				Timestamp:      time.Unix(item.Timestamp, 0),
				RelevanceScore: item.RelevanceScore,
				Tier:           item.Tier,
			})
		}
		result.Results = append(result.Results, rr)
	}

	return result, nil
}

func (c *grpcClient) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.GetSession(ctx, &pb.GetSessionRequest{SessionId: sessionID})
	if err != nil {
		return nil, err
	}

	return &SessionInfo{
		SessionID:      resp.SessionId,
		MessageCount:   int(resp.MessageCount),
		TokenCount:     int(resp.TokenCount),
		TopicCount:     int(resp.TopicCount),
		CurrentTopicID: resp.CurrentTopicId,
		CreatedAt:      time.Unix(resp.CreatedAt, 0),
		LastActiveAt:   time.Unix(resp.LastActiveAt, 0),
	}, nil
}

func (c *grpcClient) DeleteSession(ctx context.Context, sessionID string) error {
	ctx = c.contextWithAuth(ctx)

	_, err := c.client.DeleteSession(ctx, &pb.DeleteSessionRequest{SessionId: sessionID})
	return err
}

func (c *grpcClient) SwitchTopic(ctx context.Context, sessionID, newTopic string) (*SwitchTopicResponse, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.SwitchTopic(ctx, &pb.SwitchTopicRequest{
		SessionId: sessionID,
		NewTopic:  newTopic,
	})
	if err != nil {
		return nil, err
	}

	return &SwitchTopicResponse{
		CapsuleID:    resp.CapsuleId,
		TopicTitle:   resp.TopicTitle,
		MessageCount: int(resp.MessageCount),
		TokenCount:   int(resp.TokenCount),
	}, nil
}

func (c *grpcClient) RecallTopic(ctx context.Context, sessionID, topicQuery string) (*RecallTopicResponse, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.RecallTopic(ctx, &pb.RecallTopicRequest{
		SessionId:  sessionID,
		TopicQuery: topicQuery,
	})
	if err != nil {
		return nil, err
	}

	return &RecallTopicResponse{
		CapsuleID:    resp.CapsuleId,
		TopicTitle:   resp.TopicTitle,
		LoadedFromL3: resp.LoadedFromL3,
		Summary:      resp.Summary,
	}, nil
}

func (c *grpcClient) ListTopics(ctx context.Context, sessionID string) ([]*TopicInfo, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.ListTopics(ctx, &pb.ListTopicsRequest{SessionId: sessionID})
	if err != nil {
		return nil, err
	}

	var topics []*TopicInfo
	for _, t := range resp.Topics {
		topics = append(topics, &TopicInfo{
			TopicID:      t.TopicId,
			Title:        t.Title,
			MessageCount: int(t.MessageCount),
			TokenCount:   int(t.TokenCount),
			CreatedAt:    time.Unix(t.CreatedAt, 0),
			LastActiveAt: time.Unix(t.LastActiveAt, 0),
			Tier:         t.Tier,
		})
	}

	return topics, nil
}

func (c *grpcClient) GetEntityRelations(ctx context.Context, entityName string, depth int) (*EntityRelationsResponse, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.GetEntityRelations(ctx, &pb.GetEntityRelationsRequest{
		EntityName: entityName,
		Depth:      int32(depth),
	})
	if err != nil {
		return nil, err
	}

	result := &EntityRelationsResponse{
		EntityName: resp.EntityName,
		Depth:      int(resp.Depth),
	}

	for _, r := range resp.Relations {
		result.Relations = append(result.Relations, &Relation{
			Source: r.Source,
			Target: r.Target,
			Type:   r.Type,
			Weight: r.Weight,
		})
	}

	return result, nil
}

func (c *grpcClient) Summarize(ctx context.Context, sessionID string) (*SummaryResponse, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.Summarize(ctx, &pb.SummarizeRequest{SessionId: sessionID})
	if err != nil {
		return nil, err
	}

	return &SummaryResponse{
		Summary:      resp.Summary,
		MessageCount: int(resp.MessageCount),
		TokenCount:   int(resp.TokenCount),
		KeyEntities:  resp.KeyEntities,
	}, nil
}

func (c *grpcClient) Archive(ctx context.Context, req *ArchiveRequest) (*ArchiveResponse, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.Archive(ctx, &pb.ArchiveRequest{
		SessionId:  req.SessionID,
		TopicTitle: req.TopicTitle,
	})
	if err != nil {
		return nil, err
	}

	return &ArchiveResponse{
		CapsuleID: resp.CapsuleId,
		ArchiveID: resp.ArchiveId,
		Archived:  resp.Archived,
	}, nil
}

func (c *grpcClient) GetStats(ctx context.Context) (*StatsResponse, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.GetStats(ctx, &pb.GetStatsRequest{})
	if err != nil {
		return nil, err
	}

	return &StatsResponse{
		L1: TierStats{
			SessionCount: int(resp.L1.SessionCount),
			MessageCount: int(resp.L1.MessageCount),
			TokenCount:   int(resp.L1.TokenCount),
			SizeBytes:    int(resp.L1.SizeBytes),
		},
		L2: TierStats{
			SessionCount: int(resp.L2.SessionCount),
			MessageCount: int(resp.L2.MessageCount),
			TokenCount:   int(resp.L2.TokenCount),
			SizeBytes:    int(resp.L2.SizeBytes),
		},
		L3: TierStats{
			SessionCount: int(resp.L3.SessionCount),
			MessageCount: int(resp.L3.MessageCount),
			TokenCount:   int(resp.L3.TokenCount),
			SizeBytes:    int(resp.L3.SizeBytes),
		},
		Total: TotalStats{
			TotalSessions:  int(resp.Total.TotalSessions),
			TotalMessages:  int(resp.Total.TotalMessages),
			TotalTokens:    int(resp.Total.TotalTokens),
			TotalSizeBytes: int(resp.Total.TotalSizeBytes),
		},
	}, nil
}

func (c *grpcClient) Health(ctx context.Context) (*HealthResponse, error) {
	ctx = c.contextWithAuth(ctx)

	resp, err := c.client.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		return nil, err
	}

	return &HealthResponse{
		Status:    resp.Status,
		Version:   resp.Version,
		Timestamp: time.Unix(resp.Timestamp, 0),
	}, nil
}

func (c *grpcClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ==================== REST Client ====================

type restClient struct {
	config     *ClientConfig
	httpClient *http.Client
	baseURL    string
}

func newRESTClient(config *ClientConfig) (*restClient, error) {
	return &restClient{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		baseURL: config.Address,
	}, nil
}

func (c *restClient) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %v", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("X-API-Key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %v", err)
		}
	}

	return nil
}

func (c *restClient) Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error) {
	var resp StoreResponse
	err := c.doRequest(ctx, "POST", "/api/v1/memory/store", req, &resp)
	return &resp, err
}

func (c *restClient) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	var resp RetrieveResponse
	err := c.doRequest(ctx, "POST", "/api/v1/memory/retrieve", req, &resp)
	return &resp, err
}

func (c *restClient) BatchStore(ctx context.Context, req *BatchStoreRequest) (*BatchStoreResponse, error) {
	var resp BatchStoreResponse
	err := c.doRequest(ctx, "POST", "/api/v1/memory/batch/store", req, &resp)
	return &resp, err
}

func (c *restClient) BatchRetrieve(ctx context.Context, req *BatchRetrieveRequest) (*BatchRetrieveResponse, error) {
	var resp BatchRetrieveResponse
	err := c.doRequest(ctx, "POST", "/api/v1/memory/batch/retrieve", req, &resp)
	return &resp, err
}

func (c *restClient) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	var resp SessionInfo
	err := c.doRequest(ctx, "GET", "/api/v1/memory/sessions/"+sessionID, nil, &resp)
	return &resp, err
}

func (c *restClient) DeleteSession(ctx context.Context, sessionID string) error {
	return c.doRequest(ctx, "DELETE", "/api/v1/memory/sessions/"+sessionID, nil, nil)
}

func (c *restClient) SwitchTopic(ctx context.Context, sessionID, newTopic string) (*SwitchTopicResponse, error) {
	var resp SwitchTopicResponse
	req := map[string]string{"new_topic": newTopic}
	err := c.doRequest(ctx, "POST", "/api/v1/memory/sessions/"+sessionID+"/topics/switch", req, &resp)
	return &resp, err
}

func (c *restClient) RecallTopic(ctx context.Context, sessionID, topicQuery string) (*RecallTopicResponse, error) {
	var resp RecallTopicResponse
	req := map[string]string{"topic_query": topicQuery}
	err := c.doRequest(ctx, "POST", "/api/v1/memory/sessions/"+sessionID+"/topics/recall", req, &resp)
	return &resp, err
}

func (c *restClient) ListTopics(ctx context.Context, sessionID string) ([]*TopicInfo, error) {
	var resp struct {
		Topics []*TopicInfo `json:"topics"`
	}
	err := c.doRequest(ctx, "GET", "/api/v1/memory/sessions/"+sessionID+"/topics", nil, &resp)
	return resp.Topics, err
}

func (c *restClient) GetEntityRelations(ctx context.Context, entityName string, depth int) (*EntityRelationsResponse, error) {
	var resp EntityRelationsResponse
	path := fmt.Sprintf("/api/v1/memory/entities/%s/relations?depth=%d", entityName, depth)
	err := c.doRequest(ctx, "GET", path, nil, &resp)
	return &resp, err
}

func (c *restClient) Summarize(ctx context.Context, sessionID string) (*SummaryResponse, error) {
	var resp SummaryResponse
	err := c.doRequest(ctx, "POST", "/api/v1/memory/sessions/"+sessionID+"/summarize", nil, &resp)
	return &resp, err
}

func (c *restClient) Archive(ctx context.Context, req *ArchiveRequest) (*ArchiveResponse, error) {
	var resp ArchiveResponse
	err := c.doRequest(ctx, "POST", "/api/v1/memory/archive", req, &resp)
	return &resp, err
}

func (c *restClient) GetStats(ctx context.Context) (*StatsResponse, error) {
	var resp StatsResponse
	err := c.doRequest(ctx, "GET", "/api/v1/memory/stats", nil, &resp)
	return &resp, err
}

func (c *restClient) Health(ctx context.Context) (*HealthResponse, error) {
	var resp HealthResponse
	err := c.doRequest(ctx, "GET", "/api/v1/memory/health", nil, &resp)
	return &resp, err
}

func (c *restClient) Close() error {
	return nil
}
