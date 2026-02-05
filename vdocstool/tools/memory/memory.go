// Package memory 记忆管理工具
package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/vdocstool/config"
	"github.com/cloudwego/eino/vdocstool/mcp"
	"github.com/cloudwego/eino/vdocstool/tools/memory/capacity"
	"github.com/cloudwego/eino/vdocstool/tools/memory/capsule"
	"github.com/cloudwego/eino/vdocstool/tools/memory/retrieval"
	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// MemoryTool 记忆管理工具
type MemoryTool struct {
	l1             *storage.L1WorkingMemory
	l2             *storage.L2ShortTermMemory
	l3             *storage.L3LongTermMemory
	tierManager    *storage.TierManager
	capsuleManager *capsule.CapsuleManager
	pipeline       *retrieval.Pipeline
	monitor        *capacity.Monitor
	archiver       *capacity.Archiver

	config config.MemoryConfig
}

// NewMemoryTool 创建记忆工具
func NewMemoryTool(cfg config.MemoryConfig) (*MemoryTool, error) {
	// 创建 L1 存储
	l1, err := storage.NewL1WorkingMemory(cfg.L1)
	if err != nil {
		return nil, fmt.Errorf("create L1 storage failed: %w", err)
	}

	// 创建 L2 存储
	l2, err := storage.NewL2ShortTermMemory(cfg.L2)
	if err != nil {
		l1.Close()
		return nil, fmt.Errorf("create L2 storage failed: %w", err)
	}

	// 创建 L3 存储
	l3, err := storage.NewL3LongTermMemory(cfg.L3)
	if err != nil {
		l1.Close()
		l2.Close()
		return nil, fmt.Errorf("create L3 storage failed: %w", err)
	}

	// 创建层级管理器
	tierManager := storage.NewTierManager(l1, l2, l3)

	// 创建检索管道
	pipelineCfg := retrieval.RetrievalConfig{
		SimilarityThreshold: cfg.Retrieval.SimilarityThreshold,
		DefaultTokenBudget:  cfg.Retrieval.DefaultTokenBudget,
		TimeDecayFactor:     cfg.Retrieval.TimeDecayFactor,
		ChannelWeights: retrieval.ChannelWeights{
			SemanticSearch: cfg.Retrieval.ChannelWeights.SemanticSearch,
			EntityGraph:    cfg.Retrieval.ChannelWeights.EntityGraph,
			TemporalNear:   cfg.Retrieval.ChannelWeights.TemporalNear,
			UserPinned:     cfg.Retrieval.ChannelWeights.UserPinned,
		},
	}
	pipeline := retrieval.NewPipeline(l1, l2, l3, pipelineCfg)

	// 创建监控器和归档器
	monitor := capacity.NewMonitor(l1, l2, l3, tierManager)
	archiver := capacity.NewArchiver(l1, l2, l3)

	tool := &MemoryTool{
		l1:             l1,
		l2:             l2,
		l3:             l3,
		tierManager:    tierManager,
		capsuleManager: capsule.NewCapsuleManager(),
		pipeline:       pipeline,
		monitor:        monitor,
		archiver:       archiver,
		config:         cfg,
	}

	// 启动后台任务
	tierManager.Start()
	monitor.Start()

	return tool, nil
}

// Close 关闭工具
func (t *MemoryTool) Close() error {
	t.monitor.Stop()
	t.tierManager.Stop()
	t.l1.Close()
	t.l2.Close()
	t.l3.Close()
	return nil
}

// Register 注册到 MCP Server
func (t *MemoryTool) Register(server *mcp.Server) {
	// 存储记忆
	storeTool := mcp.NewToolBuilder("store_memory", "存储消息到记忆系统（自动分层）").
		AddProperty("session_id", "string", "会话ID", true).
		AddProperty("role", "string", "角色(user/assistant/system)", true).
		AddProperty("content", "string", "消息内容", true).
		AddProperty("topic_id", "string", "主题ID（可选）", false).
		Build()

	server.RegisterTool(storeTool, t.handleStoreMemory)

	// 检索上下文
	retrieveTool := mcp.NewToolBuilder("retrieve_context", "多层检索组装上下文").
		AddProperty("query", "string", "查询内容", true).
		AddProperty("session_id", "string", "会话ID", true).
		AddProperty("token_budget", "integer", "Token预算，默认4000", false).
		AddProperty("topic_id", "string", "锁定的主题ID", false).
		Build()

	server.RegisterTool(retrieveTool, t.handleRetrieveContext)

	// 切换主题
	switchTopicTool := mcp.NewToolBuilder("switch_topic", "切换主题，归档当前L1").
		AddProperty("session_id", "string", "会话ID", true).
		AddProperty("new_topic", "string", "新主题标题", false).
		Build()

	server.RegisterTool(switchTopicTool, t.handleSwitchTopic)

	// 召回主题
	recallTopicTool := mcp.NewToolBuilder("recall_topic", "召回历史主题到L1").
		AddProperty("topic_query", "string", "主题查询", true).
		AddProperty("session_id", "string", "会话ID", true).
		Build()

	server.RegisterTool(recallTopicTool, t.handleRecallTopic)

	// 获取实体关系
	entityTool := mcp.NewToolBuilder("get_entity_relations", "查询实体关联图谱").
		AddProperty("entity_name", "string", "实体名称", true).
		AddProperty("depth", "integer", "关系深度，默认2", false).
		Build()

	server.RegisterTool(entityTool, t.handleGetEntityRelations)

	// 会话摘要
	summarizeTool := mcp.NewToolBuilder("summarize_session", "生成会话摘要").
		AddProperty("session_id", "string", "会话ID", true).
		Build()

	server.RegisterTool(summarizeTool, t.handleSummarizeSession)

	// 手动归档
	archiveTool := mcp.NewToolBuilder("archive_session", "手动归档会话到L3").
		AddProperty("session_id", "string", "会话ID", true).
		AddProperty("topic_title", "string", "主题标题", false).
		Build()

	server.RegisterTool(archiveTool, t.handleArchiveSession)

	// 获取容量统计
	statsTool := mcp.NewToolBuilder("get_capacity_stats", "获取记忆系统容量统计").
		Build()

	server.RegisterTool(statsTool, t.handleGetCapacityStats)
}

// handleStoreMemory 处理存储记忆
func (t *MemoryTool) handleStoreMemory(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, _ := args["session_id"].(string)
	role, _ := args["role"].(string)
	content, _ := args["content"].(string)
	topicID, _ := args["topic_id"].(string)

	if sessionID == "" || role == "" || content == "" {
		return nil, fmt.Errorf("session_id, role and content are required")
	}

	// 创建消息
	msg := &storage.Message{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		TopicID:    topicID,
		Role:       role,
		Content:    content,
		Timestamp:  time.Now(),
		TokenCount: estimateTokens(content),
	}

	// 存储到 L1
	if err := t.l1.AppendMessage(ctx, sessionID, msg); err != nil {
		return nil, err
	}

	// 检查是否需要归档
	shouldArchive, reason, _ := t.l1.ShouldArchive(ctx, sessionID)
	if shouldArchive {
		// 触发归档（异步）
		go func() {
			_, err := t.archiver.ArchiveL1ToL2(context.Background(), &capacity.ArchiveRequest{
				SessionID:  sessionID,
				Strategy:   capacity.StrategyAuto,
				KeepRecent: 5, // 保留最近5条
			})
			if err != nil {
				// log error
			}
		}()
	}

	return StoreMemoryResponse{
		MessageID:          msg.ID,
		Tier:               "L1",
		TokenCount:         msg.TokenCount,
		ArchiveTriggered:   shouldArchive,
		ArchiveTriggerReason: reason,
	}, nil
}

// StoreMemoryResponse 存储响应
type StoreMemoryResponse struct {
	MessageID            string `json:"message_id"`
	Tier                 string `json:"tier"`
	TokenCount           int    `json:"token_count"`
	ArchiveTriggered     bool   `json:"archive_triggered"`
	ArchiveTriggerReason string `json:"archive_trigger_reason,omitempty"`
}

// handleRetrieveContext 处理检索上下文
func (t *MemoryTool) handleRetrieveContext(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, _ := args["query"].(string)
	sessionID, _ := args["session_id"].(string)
	topicID, _ := args["topic_id"].(string)

	tokenBudget := t.config.Retrieval.DefaultTokenBudget
	if v, ok := args["token_budget"].(float64); ok && v > 0 {
		tokenBudget = int(v)
	}

	if query == "" || sessionID == "" {
		return nil, fmt.Errorf("query and session_id are required")
	}

	return t.pipeline.Retrieve(ctx, &retrieval.RetrieveRequest{
		Query:       query,
		SessionID:   sessionID,
		TokenBudget: tokenBudget,
		TopicID:     topicID,
	})
}

// handleSwitchTopic 处理切换主题
func (t *MemoryTool) handleSwitchTopic(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, _ := args["session_id"].(string)
	newTopic, _ := args["new_topic"].(string)

	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	if newTopic == "" {
		newTopic = fmt.Sprintf("Topic_%s", time.Now().Format("20060102_150405"))
	}

	// 归档当前 L1 到 L2
	capsuleData, err := t.tierManager.MigrateL1ToL2(ctx, sessionID, newTopic)
	if err != nil {
		return nil, err
	}

	return SwitchTopicResponse{
		CapsuleID:    capsuleData.ID,
		TopicTitle:   newTopic,
		MessageCount: capsuleData.MessageCount,
		TokenCount:   capsuleData.TokenCount,
	}, nil
}

// SwitchTopicResponse 切换主题响应
type SwitchTopicResponse struct {
	CapsuleID    string `json:"capsule_id"`
	TopicTitle   string `json:"topic_title"`
	MessageCount int    `json:"message_count"`
	TokenCount   int    `json:"token_count"`
}

// handleRecallTopic 处理召回主题
func (t *MemoryTool) handleRecallTopic(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	topicQuery, _ := args["topic_query"].(string)
	sessionID, _ := args["session_id"].(string)

	if topicQuery == "" || sessionID == "" {
		return nil, fmt.Errorf("topic_query and session_id are required")
	}

	// 搜索匹配的胶囊
	capsules, err := t.l2.SearchCapsules(ctx, &storage.CapsuleQuery{
		SessionID:  sessionID,
		TopicTitle: topicQuery,
		Limit:      1,
	})
	if err != nil {
		return nil, err
	}

	if len(capsules) == 0 {
		// 尝试从 L3 查找
		archives, err := t.l3.GetArchiveList(ctx, sessionID)
		if err != nil {
			return nil, err
		}

		for _, archive := range archives {
			if containsIgnoreCase(archive.TopicTitle, topicQuery) {
				// 召回到 L2
				capsuleData, err := t.tierManager.RecallL3ToL2(ctx, archive.ArchiveID)
				if err != nil {
					continue
				}

				// 加载到 L1
				if err := t.tierManager.RecallL2ToL1(ctx, capsuleData.ID, sessionID); err != nil {
					return nil, err
				}

				return RecallTopicResponse{
					CapsuleID:    capsuleData.ID,
					TopicTitle:   capsuleData.TopicTitle,
					LoadedFromL3: true,
					Summary:      capsuleData.Summary,
				}, nil
			}
		}

		return nil, fmt.Errorf("topic not found: %s", topicQuery)
	}

	// 从 L2 召回到 L1
	capsuleData := capsules[0]
	if err := t.tierManager.RecallL2ToL1(ctx, capsuleData.ID, sessionID); err != nil {
		return nil, err
	}

	return RecallTopicResponse{
		CapsuleID:    capsuleData.ID,
		TopicTitle:   capsuleData.TopicTitle,
		LoadedFromL3: false,
		Summary:      capsuleData.Summary,
	}, nil
}

// RecallTopicResponse 召回主题响应
type RecallTopicResponse struct {
	CapsuleID    string `json:"capsule_id"`
	TopicTitle   string `json:"topic_title"`
	LoadedFromL3 bool   `json:"loaded_from_l3"`
	Summary      string `json:"summary"`
}

// handleGetEntityRelations 处理获取实体关系
func (t *MemoryTool) handleGetEntityRelations(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	entityName, _ := args["entity_name"].(string)
	depth := 2
	if v, ok := args["depth"].(float64); ok && v > 0 {
		depth = int(v)
	}

	if entityName == "" {
		return nil, fmt.Errorf("entity_name is required")
	}

	relations, err := t.l3.QueryRelations(ctx, entityName, depth)
	if err != nil {
		return nil, err
	}

	return EntityRelationsResponse{
		EntityName: entityName,
		Relations:  relations,
		Depth:      depth,
	}, nil
}

// EntityRelationsResponse 实体关系响应
type EntityRelationsResponse struct {
	EntityName string             `json:"entity_name"`
	Relations  []*storage.Relation `json:"relations"`
	Depth      int                `json:"depth"`
}

// handleSummarizeSession 处理会话摘要
func (t *MemoryTool) handleSummarizeSession(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, _ := args["session_id"].(string)

	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	// 获取 L1 消息
	messages, err := t.l1.GetAllMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages in session")
	}

	// 创建临时胶囊以生成摘要
	capsuleData, err := t.capsuleManager.CreateCapsule(ctx, sessionID, "Summary", messages)
	if err != nil {
		return nil, err
	}

	return SummarizeSessionResponse{
		Summary:      capsuleData.Summary,
		MessageCount: len(messages),
		TokenCount:   capsuleData.TokenCount,
		Entities:     capsuleData.Entities,
	}, nil
}

// SummarizeSessionResponse 摘要响应
type SummarizeSessionResponse struct {
	Summary      string            `json:"summary"`
	MessageCount int               `json:"message_count"`
	TokenCount   int               `json:"token_count"`
	Entities     []*storage.Entity `json:"entities,omitempty"`
}

// handleArchiveSession 处理手动归档
func (t *MemoryTool) handleArchiveSession(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sessionID, _ := args["session_id"].(string)
	topicTitle, _ := args["topic_title"].(string)

	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	// 先归档到 L2
	result, err := t.archiver.ArchiveL1ToL2(ctx, &capacity.ArchiveRequest{
		SessionID:  sessionID,
		TopicTitle: topicTitle,
		Strategy:   capacity.StrategyFull,
	})
	if err != nil {
		return nil, err
	}

	// 然后归档到 L3
	if result.CapsuleID != "" {
		archiveResult, err := t.archiver.ArchiveL2ToL3(ctx, result.CapsuleID)
		if err == nil {
			result.ArchiveID = archiveResult.ArchiveID
		}
	}

	return result, nil
}

// handleGetCapacityStats 处理获取容量统计
func (t *MemoryTool) handleGetCapacityStats(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return t.monitor.GetCapacityStats(ctx)
}

// estimateTokens 估算 Token 数量
func estimateTokens(text string) int {
	// 简单估算：中文每字约1.5个token，英文每词约1个token
	chineseCount := 0
	wordCount := 0
	inWord := false

	for _, r := range text {
		if r > 127 {
			chineseCount++
			inWord = false
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			if !inWord {
				wordCount++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	return int(float64(chineseCount)*1.5) + wordCount
}

// containsIgnoreCase 不区分大小写的包含检查
func containsIgnoreCase(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
