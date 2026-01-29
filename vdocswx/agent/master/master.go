// Package master 实现主Agent，负责协调各子Agent进行文章润色
package master

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/cache"
	"github.com/cloudwego/eino/vdocswx/config"
	"github.com/cloudwego/eino/vdocswx/llm"
	"github.com/cloudwego/eino/vdocswx/stats"
)

// Message 消息
type Message struct {
	Role    string
	Content string
}

// MasterAgent 主Agent实现
type MasterAgent struct {
	cfg          *config.Config
	llmManager   *llm.LLMManager
	subAgents    map[string]agent.SubAgent
	cache        cache.Cache
	stats        *stats.Collector
	intentRecog  *IntentRecognizer
	planner      *Planner
	sessions     sync.Map // sessionID -> *Session
}

// Session 会话状态
type Session struct {
	ID           string
	History      []*Message
	CurrentDraft string
	Changes      []agent.Change
	CreatedAt    time.Time
	UpdatedAt    time.Time
	mu           sync.RWMutex
}

// NewMasterAgent 创建主Agent
func NewMasterAgent(
	cfg *config.Config,
	llmMgr *llm.LLMManager,
	subAgents map[string]agent.SubAgent,
	cacheImpl cache.Cache,
	statsCollector *stats.Collector,
) (*MasterAgent, error) {
	ma := &MasterAgent{
		cfg:        cfg,
		llmManager: llmMgr,
		subAgents:  subAgents,
		cache:      cacheImpl,
		stats:      statsCollector,
	}
	
	// 初始化意图识别器
	ma.intentRecog = NewIntentRecognizer(llmMgr)
	
	// 初始化规划器
	ma.planner = NewPlanner(cfg, subAgents)
	
	return ma, nil
}

// Polish 执行文章润色
func (ma *MasterAgent) Polish(ctx context.Context, input *agent.ArticleInput) (*agent.PolishResult, error) {
	startTime := time.Now()
	
	// 1. 检查缓存
	cacheKey := ma.generateCacheKey(input)
	if ma.cache != nil {
		if cached, ok := ma.cache.Get(ctx, cacheKey); ok {
			if result, ok := cached.(*agent.PolishResult); ok {
				ma.stats.RecordCacheHit("polish_result")
				return result, nil
			}
		}
		ma.stats.RecordCacheMiss("polish_result")
	}
	
	// 2. 识别文章类型
	if input.Type == agent.ArticleTypeUnknown || input.Type == "" {
		input.Type = ma.detectArticleType(ctx, input.Content)
	}
	
	// 3. 意图识别
	intent := ma.intentRecog.Recognize(ctx, input.UserContext, input.Content)
	
	// 4. 生成执行计划
	plan := ma.planner.CreatePlan(ctx, intent, input)
	
	// 5. 执行计划
	result, err := ma.executePlan(ctx, plan, input)
	if err != nil {
		return nil, fmt.Errorf("execute plan failed: %w", err)
	}
	
	// 6. 合并结果
	result.Statistics.ProcessingTime = time.Since(startTime).Milliseconds()
	
	// 7. 缓存结果
	if ma.cache != nil {
		ma.cache.Set(ctx, cacheKey, result)
	}
	
	return result, nil
}

// executePlan 执行润色计划
func (ma *MasterAgent) executePlan(ctx context.Context, plan *agent.Plan, input *agent.ArticleInput) (*agent.PolishResult, error) {
	result := &agent.PolishResult{
		OriginalContent: input.Content,
		SubAgentResults: make(map[string]*agent.SubAgentResult),
	}
	
	currentContent := input.Content
	
	if plan.Parallel {
		// 并行执行
		var wg sync.WaitGroup
		resultChan := make(chan *stepResult, len(plan.Steps))
		
		for _, step := range plan.Steps {
			wg.Add(1)
			go func(s agent.PlanStep) {
				defer wg.Done()
				subAgent, ok := ma.subAgents[s.AgentName]
				if !ok {
					resultChan <- &stepResult{
						agentName: s.AgentName,
						err:       fmt.Errorf("agent %s not found", s.AgentName),
					}
					return
				}
				
				subResult, err := subAgent.Process(ctx, currentContent, input.Type)
				resultChan <- &stepResult{
					agentName: s.AgentName,
					result:    subResult,
					err:       err,
				}
			}(step)
		}
		
		// 等待所有任务完成
		go func() {
			wg.Wait()
			close(resultChan)
		}()
		
		// 收集结果
		for sr := range resultChan {
			if sr.err != nil {
				result.SubAgentResults[sr.agentName] = &agent.SubAgentResult{
					AgentName: sr.agentName,
					Success:   false,
					Error:     sr.err.Error(),
				}
				continue
			}
			result.SubAgentResults[sr.agentName] = sr.result
			result.Changes = append(result.Changes, sr.result.Changes...)
		}
	} else {
		// 串行执行
		for _, step := range plan.Steps {
			subAgent, ok := ma.subAgents[step.AgentName]
			if !ok {
				continue
			}
			
			subResult, err := subAgent.Process(ctx, currentContent, input.Type)
			if err != nil {
				result.SubAgentResults[step.AgentName] = &agent.SubAgentResult{
					AgentName: step.AgentName,
					Success:   false,
					Error:     err.Error(),
				}
				continue
			}
			
			result.SubAgentResults[step.AgentName] = subResult
			result.Changes = append(result.Changes, subResult.Changes...)
			
			// 更新当前内容用于下一个Agent
			if subResult.Output != "" {
				currentContent = subResult.Output
			}
		}
	}
	
	// 最终合并所有修改生成润色后内容
	result.PolishedContent = ma.applyChanges(input.Content, result.Changes)
	result.Statistics = ma.calculateStatistics(input.Content, result.PolishedContent, result.Changes)
	
	return result, nil
}

// stepResult 步骤执行结果
type stepResult struct {
	agentName string
	result    *agent.SubAgentResult
	err       error
}

// Stream 流式润色处理
func (ma *MasterAgent) Stream(ctx context.Context, input *agent.ArticleInput) (<-chan *agent.StreamEvent, error) {
	eventChan := make(chan *agent.StreamEvent, 100)
	
	go func() {
		defer close(eventChan)
		
		// 发送开始事件
		eventChan <- &agent.StreamEvent{
			Type:    agent.StreamEventTypeProgress,
			Content: "开始分析文章...",
			Agent:   "master",
		}
		
		// 执行润色
		result, err := ma.Polish(ctx, input)
		if err != nil {
			eventChan <- &agent.StreamEvent{
				Type:  agent.StreamEventTypeError,
				Error: err,
				Agent: "master",
			}
			return
		}
		
		// 发送修改事件
		for _, change := range result.Changes {
			eventChan <- &agent.StreamEvent{
				Type:    agent.StreamEventTypeChange,
				Content: fmt.Sprintf("[%s] %s -> %s", change.Type, change.Original, change.Modified),
				Agent:   "master",
			}
		}
		
		// 发送完成事件
		eventChan <- &agent.StreamEvent{
			Type:    agent.StreamEventTypeDone,
			Content: result.PolishedContent,
			Agent:   "master",
			Done:    true,
		}
	}()
	
	return eventChan, nil
}

// Chat 多轮对话交互
func (ma *MasterAgent) Chat(ctx context.Context, sessionID string, message string) (*agent.ChatResponse, error) {
	// 获取或创建会话
	session := ma.getOrCreateSession(sessionID)
	
	session.mu.Lock()
	defer session.mu.Unlock()
	
	// 添加用户消息
	session.History = append(session.History, &Message{Role: "user", Content: message})
	
	// 识别意图
	intent := ma.intentRecog.Recognize(ctx, message, session.CurrentDraft)
	
	var response *agent.ChatResponse
	
	switch intent.Type {
	case agent.IntentTypePolish:
		// 执行全文润色
		input := &agent.ArticleInput{
			Content:     session.CurrentDraft,
			UserContext: message,
		}
		result, err := ma.Polish(ctx, input)
		if err != nil {
			return nil, err
		}
		session.CurrentDraft = result.PolishedContent
		session.Changes = append(session.Changes, result.Changes...)
		
		response = &agent.ChatResponse{
			Message:       fmt.Sprintf("已完成润色，共进行了 %d 处修改", len(result.Changes)),
			Modifications: result.Changes,
			Suggestions:   result.Suggestions,
			SessionID:     sessionID,
		}
		
	case agent.IntentTypeModify:
		// 执行指定修改
		response = &agent.ChatResponse{
			Message:   "已根据您的要求进行修改",
			SessionID: sessionID,
		}
		
	case agent.IntentTypeQuestion:
		// 回答问题
		answer, err := ma.answerQuestion(ctx, message, session.CurrentDraft)
		if err != nil {
			return nil, err
		}
		response = &agent.ChatResponse{
			Message:   answer,
			SessionID: sessionID,
		}
		
	default:
		// 默认处理
		response = &agent.ChatResponse{
			Message:   "请告诉我您想要对文章进行什么操作？您可以要求润色、修改特定部分或询问问题。",
			SessionID: sessionID,
		}
	}
	
	// 添加助手回复
	session.History = append(session.History, &Message{Role: "assistant", Content: response.Message})
	session.UpdatedAt = time.Now()
	
	return response, nil
}

// GetHistory 获取会话历史
func (ma *MasterAgent) GetHistory(ctx context.Context, sessionID string) ([]*Message, error) {
	if session, ok := ma.sessions.Load(sessionID); ok {
		s := session.(*Session)
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.History, nil
	}
	return nil, fmt.Errorf("session %s not found", sessionID)
}

// SetSessionDraft 设置会话草稿
func (ma *MasterAgent) SetSessionDraft(sessionID, draft string) {
	session := ma.getOrCreateSession(sessionID)
	session.mu.Lock()
	defer session.mu.Unlock()
	session.CurrentDraft = draft
	session.UpdatedAt = time.Now()
}

// getOrCreateSession 获取或创建会话
func (ma *MasterAgent) getOrCreateSession(sessionID string) *Session {
	if session, ok := ma.sessions.Load(sessionID); ok {
		return session.(*Session)
	}
	
	session := &Session{
		ID:        sessionID,
		History:   make([]*Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	ma.sessions.Store(sessionID, session)
	return session
}

// detectArticleType 检测文章类型
func (ma *MasterAgent) detectArticleType(ctx context.Context, content string) agent.ArticleType {
	// 简单的关键词检测
	techKeywords := []string{"代码", "函数", "API", "服务器", "数据库", "算法", "程序", "开发", "接口", "框架"}
	techCount := 0
	for _, kw := range techKeywords {
		if containsString(content, kw) {
			techCount++
		}
	}
	
	if techCount >= 3 {
		return agent.ArticleTypeTech
	}
	return agent.ArticleTypeNonTech
}

// generateCacheKey 生成缓存键
func (ma *MasterAgent) generateCacheKey(input *agent.ArticleInput) string {
	return fmt.Sprintf("polish:%s:%s", input.Type, hashString(input.Content))
}

// applyChanges 应用修改到原文
func (ma *MasterAgent) applyChanges(original string, changes []agent.Change) string {
	// 简单实现：按顺序应用修改
	result := original
	for _, change := range changes {
		if change.Original != "" && change.Modified != "" {
			result = replaceFirst(result, change.Original, change.Modified)
		}
	}
	return result
}

// calculateStatistics 计算统计信息
func (ma *MasterAgent) calculateStatistics(original, polished string, changes []agent.Change) agent.Statistics {
	stats := agent.Statistics{
		OriginalCharCount: len([]rune(original)),
		PolishedCharCount: len([]rune(polished)),
		TotalChanges:      len(changes),
	}
	
	for _, change := range changes {
		switch change.Type {
		case agent.ChangeTypeGrammar:
			stats.GrammarChanges++
		case agent.ChangeTypeStyle:
			stats.StyleChanges++
		case agent.ChangeTypeStructure:
			stats.StructureChanges++
		}
	}
	
	return stats
}

// answerQuestion 回答用户问题
func (ma *MasterAgent) answerQuestion(ctx context.Context, question, content string) (string, error) {
	if ma.llmManager == nil {
		return "抱歉，LLM服务不可用", nil
	}
	
	prompt := fmt.Sprintf(`基于以下文章内容，回答用户的问题：

文章内容：
%s

用户问题：%s

请给出专业、准确的回答：`, content, question)
	
	return ma.llmManager.GenerateText(ctx, prompt, "question")
}

// 辅助函数

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hashString(s string) string {
	// 简单hash实现
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%x", h)
}

func replaceFirst(s, old, new string) string {
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}
