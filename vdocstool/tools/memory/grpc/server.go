// Package grpc 实现Memory服务的gRPC接口
package grpc

import (
	"context"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/cloudwego/eino/vdocstool/tools/memory/grpc/pb"
	"github.com/cloudwego/eino/vdocstool/tools/memory/rest"
)

// ServerConfig gRPC服务器配置
type ServerConfig struct {
	Address    string            `json:"address"`
	MaxMsgSize int               `json:"max_msg_size"`
	APIKeys    map[string]string `json:"api_keys"` // key -> service_name
	RateLimit  int               `json:"rate_limit"`
}

// DefaultServerConfig 返回默认配置
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Address:    ":50051",
		MaxMsgSize: 16 * 1024 * 1024, // 16MB
		APIKeys:    make(map[string]string),
		RateLimit:  100,
	}
}

// Server gRPC服务器
type Server struct {
	pb.UnimplementedMemoryServiceServer
	config        *ServerConfig
	memoryService rest.MemoryService
	grpcServer    *grpc.Server
	rateLimiter   *RateLimiter
	mu            sync.RWMutex
}

// RateLimiter 速率限制器
type RateLimiter struct {
	limit    int
	window   time.Duration
	counters map[string]*counter
	mu       sync.Mutex
}

type counter struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:    limit,
		window:   window,
		counters: make(map[string]*counter),
	}
}

// Allow 检查是否允许请求
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	c, exists := r.counters[key]
	if !exists || now.After(c.resetTime) {
		r.counters[key] = &counter{
			count:     1,
			resetTime: now.Add(r.window),
		}
		return true
	}

	if c.count >= r.limit {
		return false
	}
	c.count++
	return true
}

// NewServer 创建gRPC服务器
func NewServer(config *ServerConfig, memoryService rest.MemoryService) *Server {
	return &Server{
		config:        config,
		memoryService: memoryService,
		rateLimiter:   NewRateLimiter(config.RateLimit, time.Minute),
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return err
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.config.MaxMsgSize),
		grpc.MaxSendMsgSize(s.config.MaxMsgSize),
		grpc.UnaryInterceptor(s.unaryInterceptor),
	}

	s.grpcServer = grpc.NewServer(opts...)
	pb.RegisterMemoryServiceServer(s.grpcServer, s)

	return s.grpcServer.Serve(lis)
}

// Stop 停止服务器
func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// unaryInterceptor 统一拦截器
func (s *Server) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// 认证
	apiKey, err := s.extractAPIKey(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "missing or invalid API key")
	}

	serviceName, valid := s.config.APIKeys[apiKey]
	if !valid && len(s.config.APIKeys) > 0 {
		return nil, status.Error(codes.Unauthenticated, "invalid API key")
	}

	// 速率限制
	if !s.rateLimiter.Allow(apiKey) {
		return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}

	// 添加服务名到context
	ctx = context.WithValue(ctx, "service_name", serviceName)

	return handler(ctx, req)
}

// extractAPIKey 从元数据中提取API Key
func (s *Server) extractAPIKey(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	keys := md.Get("x-api-key")
	if len(keys) == 0 {
		keys = md.Get("authorization")
	}
	if len(keys) == 0 {
		// 如果没有配置API Key，允许匿名访问
		if len(s.config.APIKeys) == 0 {
			return "anonymous", nil
		}
		return "", status.Error(codes.Unauthenticated, "missing API key")
	}

	return keys[0], nil
}

// Store 存储消息
func (s *Server) Store(ctx context.Context, req *pb.StoreRequest) (*pb.StoreResponse, error) {
	restReq := &rest.StoreRequest{
		SessionID: req.SessionId,
		Source:    req.Source,
		Metadata:  req.Metadata,
	}

	if req.Message != nil {
		restReq.Message = rest.MessageInput{
			Role:    req.Message.Role,
			Content: req.Message.Content,
			TopicID: req.Message.TopicId,
		}
		if req.Message.Timestamp > 0 {
			restReq.Message.Timestamp = time.Unix(req.Message.Timestamp, 0)
		}
	}

	if req.Options != nil {
		restReq.Options = &rest.StoreOptions{
			ExtractEntities:   req.Options.ExtractEntities,
			GenerateEmbedding: req.Options.GenerateEmbedding,
			Importance:        req.Options.Importance,
		}
	}

	resp, err := s.memoryService.Store(ctx, restReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "store failed: %v", err)
	}

	return &pb.StoreResponse{
		MessageId:         resp.MessageID,
		Tier:              resp.Tier,
		TokenCount:        int32(resp.TokenCount),
		EntitiesExtracted: resp.EntitiesExtracted,
		ArchiveTriggered:  resp.ArchiveTriggered,
	}, nil
}

// Retrieve 检索上下文
func (s *Server) Retrieve(ctx context.Context, req *pb.RetrieveRequest) (*pb.RetrieveResponse, error) {
	restReq := &rest.RetrieveRequest{
		SessionID: req.SessionId,
		Query:     req.Query,
	}

	if req.Options != nil {
		restReq.Options = &rest.RetrieveOptions{
			TokenBudget:     int(req.Options.TokenBudget),
			TopicID:         req.Options.TopicId,
			Filters:         req.Options.Filters,
			IncludeSummary:  req.Options.IncludeSummary,
			IncludeEntities: req.Options.IncludeEntities,
		}
		if req.Options.TimeRange != nil {
			restReq.Options.TimeRange = &rest.TimeRange{
				Start: time.Unix(req.Options.TimeRange.Start, 0),
				End:   time.Unix(req.Options.TimeRange.End, 0),
			}
		}
	}

	resp, err := s.memoryService.Retrieve(ctx, restReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "retrieve failed: %v", err)
	}

	result := &pb.RetrieveResponse{
		Summary:     resp.Summary,
		TotalTokens: int32(resp.TotalTokens),
	}

	for _, item := range resp.Context {
		result.Context = append(result.Context, &pb.ContextItem{
			MessageId:      item.MessageID,
			Role:           item.Role,
			Content:        item.Content,
			Timestamp:      item.Timestamp.Unix(),
			RelevanceScore: item.RelevanceScore,
			Tier:           item.Tier,
		})
	}

	for _, entity := range resp.Entities {
		result.Entities = append(result.Entities, &pb.EntityInfo{
			Name:     entity.Name,
			Type:     entity.Type,
			Mentions: int32(entity.Mentions),
		})
	}

	if resp.SearchStats != nil {
		result.SearchStats = &pb.SearchStats{
			L1Hits:          int32(resp.SearchStats.L1Hits),
			L2Hits:          int32(resp.SearchStats.L2Hits),
			L3Hits:          int32(resp.SearchStats.L3Hits),
			SearchLatencyMs: resp.SearchStats.SearchLatencyMs,
		}
	}

	return result, nil
}

// BatchStore 批量存储
func (s *Server) BatchStore(ctx context.Context, req *pb.BatchStoreRequest) (*pb.BatchStoreResponse, error) {
	restReq := &rest.BatchStoreRequest{
		SessionID: req.SessionId,
	}

	for _, msg := range req.Messages {
		input := rest.MessageInput{
			Role:    msg.Role,
			Content: msg.Content,
			TopicID: msg.TopicId,
		}
		if msg.Timestamp > 0 {
			input.Timestamp = time.Unix(msg.Timestamp, 0)
		}
		restReq.Messages = append(restReq.Messages, input)
	}

	if req.Options != nil {
		restReq.Options = &rest.BatchStoreOptions{
			PreserveOrder:      req.Options.PreserveOrder,
			ParallelProcessing: req.Options.ParallelProcessing,
		}
	}

	resp, err := s.memoryService.BatchStore(ctx, restReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "batch store failed: %v", err)
	}

	result := &pb.BatchStoreResponse{
		SuccessCount: int32(resp.SuccessCount),
		FailedCount:  int32(resp.FailedCount),
		TotalTokens:  int32(resp.TotalTokens),
	}

	for _, r := range resp.Results {
		result.Results = append(result.Results, &pb.StoreResponse{
			MessageId:         r.MessageID,
			Tier:              r.Tier,
			TokenCount:        int32(r.TokenCount),
			EntitiesExtracted: r.EntitiesExtracted,
			ArchiveTriggered:  r.ArchiveTriggered,
		})
	}

	return result, nil
}

// BatchRetrieve 批量检索
func (s *Server) BatchRetrieve(ctx context.Context, req *pb.BatchRetrieveRequest) (*pb.BatchRetrieveResponse, error) {
	restReq := &rest.BatchRetrieveRequest{}

	for _, r := range req.Requests {
		restReq.Requests = append(restReq.Requests, rest.SingleRetrieveRequest{
			SessionID: r.SessionId,
			Query:     r.Query,
		})
	}

	if req.Options != nil {
		restReq.Options = &rest.BatchRetrieveOptions{
			Parallel:  req.Options.Parallel,
			TimeoutMs: int(req.Options.TimeoutMs),
		}
	}

	resp, err := s.memoryService.BatchRetrieve(ctx, restReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "batch retrieve failed: %v", err)
	}

	result := &pb.BatchRetrieveResponse{}

	for _, r := range resp.Results {
		pbResp := &pb.RetrieveResponse{
			Summary:     r.Summary,
			TotalTokens: int32(r.TotalTokens),
		}
		for _, item := range r.Context {
			pbResp.Context = append(pbResp.Context, &pb.ContextItem{
				MessageId:      item.MessageID,
				Role:           item.Role,
				Content:        item.Content,
				Timestamp:      item.Timestamp.Unix(),
				RelevanceScore: item.RelevanceScore,
				Tier:           item.Tier,
			})
		}
		result.Results = append(result.Results, pbResp)
	}

	return result, nil
}

// GetSession 获取会话信息
func (s *Server) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.SessionInfo, error) {
	resp, err := s.memoryService.GetSession(ctx, req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "session not found: %v", err)
	}

	return &pb.SessionInfo{
		SessionId:      resp.SessionID,
		MessageCount:   int32(resp.MessageCount),
		TokenCount:     int32(resp.TokenCount),
		TopicCount:     int32(resp.TopicCount),
		CurrentTopicId: resp.CurrentTopicID,
		CreatedAt:      resp.CreatedAt.Unix(),
		LastActiveAt:   resp.LastActiveAt.Unix(),
	}, nil
}

// DeleteSession 删除会话
func (s *Server) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*pb.DeleteSessionResponse, error) {
	err := s.memoryService.DeleteSession(ctx, req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete session failed: %v", err)
	}

	return &pb.DeleteSessionResponse{Deleted: true}, nil
}

// SwitchTopic 切换主题
func (s *Server) SwitchTopic(ctx context.Context, req *pb.SwitchTopicRequest) (*pb.SwitchTopicResponse, error) {
	restReq := &rest.SwitchTopicRequest{
		SessionID: req.SessionId,
		NewTopic:  req.NewTopic,
	}

	resp, err := s.memoryService.SwitchTopic(ctx, restReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "switch topic failed: %v", err)
	}

	return &pb.SwitchTopicResponse{
		CapsuleId:    resp.CapsuleID,
		TopicTitle:   resp.TopicTitle,
		MessageCount: int32(resp.MessageCount),
		TokenCount:   int32(resp.TokenCount),
	}, nil
}

// RecallTopic 召回主题
func (s *Server) RecallTopic(ctx context.Context, req *pb.RecallTopicRequest) (*pb.RecallTopicResponse, error) {
	restReq := &rest.RecallTopicRequest{
		SessionID:  req.SessionId,
		TopicQuery: req.TopicQuery,
	}

	resp, err := s.memoryService.RecallTopic(ctx, restReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "recall topic failed: %v", err)
	}

	return &pb.RecallTopicResponse{
		CapsuleId:    resp.CapsuleID,
		TopicTitle:   resp.TopicTitle,
		LoadedFromL3: resp.LoadedFromL3,
		Summary:      resp.Summary,
	}, nil
}

// ListTopics 列出主题
func (s *Server) ListTopics(ctx context.Context, req *pb.ListTopicsRequest) (*pb.ListTopicsResponse, error) {
	topics, err := s.memoryService.ListTopics(ctx, req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list topics failed: %v", err)
	}

	result := &pb.ListTopicsResponse{}
	for _, t := range topics {
		result.Topics = append(result.Topics, &pb.TopicInfo{
			TopicId:      t.TopicID,
			Title:        t.Title,
			MessageCount: int32(t.MessageCount),
			TokenCount:   int32(t.TokenCount),
			CreatedAt:    t.CreatedAt.Unix(),
			LastActiveAt: t.LastActiveAt.Unix(),
			Tier:         t.Tier,
		})
	}

	return result, nil
}

// GetEntityRelations 获取实体关系
func (s *Server) GetEntityRelations(ctx context.Context, req *pb.GetEntityRelationsRequest) (*pb.GetEntityRelationsResponse, error) {
	relations, err := s.memoryService.GetEntityRelations(ctx, req.EntityName, int(req.Depth))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get entity relations failed: %v", err)
	}

	result := &pb.GetEntityRelationsResponse{
		EntityName: req.EntityName,
		Depth:      req.Depth,
	}

	for _, r := range relations {
		result.Relations = append(result.Relations, &pb.Relation{
			Source: r.Source,
			Target: r.Target,
			Type:   r.Type,
			Weight: r.Weight,
		})
	}

	return result, nil
}

// Summarize 生成摘要
func (s *Server) Summarize(ctx context.Context, req *pb.SummarizeRequest) (*pb.SummaryResponse, error) {
	resp, err := s.memoryService.Summarize(ctx, req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "summarize failed: %v", err)
	}

	return &pb.SummaryResponse{
		Summary:      resp.Summary,
		MessageCount: int32(resp.MessageCount),
		TokenCount:   int32(resp.TokenCount),
		KeyEntities:  resp.KeyEntities,
	}, nil
}

// Archive 归档会话
func (s *Server) Archive(ctx context.Context, req *pb.ArchiveRequest) (*pb.ArchiveResponse, error) {
	restReq := &rest.ArchiveRequest{
		SessionID:  req.SessionId,
		TopicTitle: req.TopicTitle,
	}

	resp, err := s.memoryService.Archive(ctx, restReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "archive failed: %v", err)
	}

	return &pb.ArchiveResponse{
		CapsuleId: resp.CapsuleID,
		ArchiveId: resp.ArchiveID,
		Archived:  resp.Archived,
	}, nil
}

// GetStats 获取统计
func (s *Server) GetStats(ctx context.Context, _ *pb.GetStatsRequest) (*pb.StatsResponse, error) {
	stats, err := s.memoryService.GetStats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stats failed: %v", err)
	}

	result := &pb.StatsResponse{}

	if stats.L1Stats != nil {
		result.L1 = &pb.TierStats{
			SessionCount: int32(stats.L1Stats.SessionCount),
			MessageCount: int32(stats.L1Stats.MessageCount),
			TokenCount:   stats.L1Stats.TokenCount,
			SizeBytes:    stats.L1Stats.SizeBytes,
		}
	}

	if stats.L2Stats != nil {
		result.L2 = &pb.TierStats{
			SessionCount: int32(stats.L2Stats.SessionCount),
			MessageCount: int32(stats.L2Stats.MessageCount),
			TokenCount:   stats.L2Stats.TokenCount,
			SizeBytes:    stats.L2Stats.SizeBytes,
		}
	}

	if stats.L3Stats != nil {
		result.L3 = &pb.TierStats{
			SessionCount: int32(stats.L3Stats.SessionCount),
			MessageCount: int32(stats.L3Stats.MessageCount),
			TokenCount:   stats.L3Stats.TokenCount,
			SizeBytes:    stats.L3Stats.SizeBytes,
		}
	}

	if stats.Total != nil {
		result.Total = &pb.TotalStats{
			TotalSessions:  int32(stats.Total.TotalSessions),
			TotalMessages:  int32(stats.Total.TotalMessages),
			TotalTokens:    stats.Total.TotalTokens,
			TotalSizeBytes: stats.Total.TotalSizeBytes,
		}
	}

	return result, nil
}

// Health 健康检查
func (s *Server) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Timestamp: time.Now().Unix(),
	}, nil
}
