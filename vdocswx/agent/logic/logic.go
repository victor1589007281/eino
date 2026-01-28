// Package logic 实现逻辑优化子Agent
package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/llm"
)

// LogicAgent 逻辑优化Agent
type LogicAgent struct {
	llmManager *llm.LLMManager
}

// NewLogicAgent 创建逻辑优化Agent
func NewLogicAgent(llmMgr *llm.LLMManager) *LogicAgent {
	return &LogicAgent{
		llmManager: llmMgr,
	}
}

// Name 返回Agent名称
func (la *LogicAgent) Name() string {
	return "logic"
}

// CanHandle 判断是否能处理该类型文章
func (la *LogicAgent) CanHandle(articleType agent.ArticleType) bool {
	return true
}

// Process 处理文章内容
func (la *LogicAgent) Process(ctx context.Context, content string, articleType agent.ArticleType) (*agent.SubAgentResult, error) {
	startTime := time.Now()
	
	result := &agent.SubAgentResult{
		AgentName: la.Name(),
		Success:   true,
		Changes:   make([]agent.Change, 0),
	}
	
	// 1. 基础逻辑检查
	basicChanges := la.basicLogicCheck(content)
	result.Changes = append(result.Changes, basicChanges...)
	
	// 2. 使用LLM进行深度逻辑分析
	llmChanges, llmOutput, tokens, err := la.llmLogicOptimize(ctx, content, articleType)
	if err != nil {
		result.Output = content
		result.TokensUsed = 0
	} else {
		result.Changes = append(result.Changes, llmChanges...)
		result.Output = llmOutput
		result.TokensUsed = tokens
	}
	
	result.Duration = time.Since(startTime).Milliseconds()
	
	return result, nil
}

// basicLogicCheck 基础逻辑检查
func (la *LogicAgent) basicLogicCheck(content string) []agent.Change {
	changes := make([]agent.Change, 0)
	
	// 检查过渡词使用
	changes = append(changes, la.checkTransitions(content)...)
	
	// 检查因果关系
	changes = append(changes, la.checkCausality(content)...)
	
	// 检查列举完整性
	changes = append(changes, la.checkEnumeration(content)...)
	
	return changes
}

// checkTransitions 检查过渡词使用
func (la *LogicAgent) checkTransitions(content string) []agent.Change {
	changes := make([]agent.Change, 0)
	
	paragraphs := strings.Split(content, "\n\n")
	if len(paragraphs) > 3 {
		transitionWords := []string{"首先", "其次", "然后", "最后", "此外", "另外", "因此", "所以", "但是", "然而", "总之", "综上"}
		transitionCount := 0
		
		for _, word := range transitionWords {
			transitionCount += strings.Count(content, word)
		}
		
		if float64(transitionCount)/float64(len(paragraphs)) < 0.3 {
			changes = append(changes, agent.Change{
				Type:       agent.ChangeTypeLogic,
				Original:   "",
				Modified:   "",
				Reason:     "文章段落间过渡词较少，建议增加过渡词提高连贯性（如：首先、其次、因此、然而等）",
				Confidence: 0.7,
			})
		}
	}
	
	return changes
}

// checkCausality 检查因果关系
func (la *LogicAgent) checkCausality(content string) []agent.Change {
	changes := make([]agent.Change, 0)
	
	// 检查"因为...所以..."配对
	becauseCount := strings.Count(content, "因为")
	soCount := strings.Count(content, "所以")
	
	// 如果"因为"和"所以"数量差异较大，可能存在逻辑问题
	if becauseCount > 0 && soCount == 0 {
		changes = append(changes, agent.Change{
			Type:       agent.ChangeTypeLogic,
			Original:   "",
			Modified:   "",
			Reason:     "发现"因为"但缺少相应的"所以"结论，建议补充因果推理的结论部分",
			Confidence: 0.6,
		})
	}
	
	return changes
}

// checkEnumeration 检查列举完整性
func (la *LogicAgent) checkEnumeration(content string) []agent.Change {
	changes := make([]agent.Change, 0)
	
	// 检查"第一...第二..."序列
	patterns := []struct {
		first  string
		second string
		third  string
	}{
		{"第一", "第二", "第三"},
		{"首先", "其次", "最后"},
		{"一是", "二是", "三是"},
	}
	
	for _, p := range patterns {
		firstCount := strings.Count(content, p.first)
		secondCount := strings.Count(content, p.second)
		
		if firstCount > 0 && secondCount == 0 {
			changes = append(changes, agent.Change{
				Type:       agent.ChangeTypeLogic,
				Original:   "",
				Modified:   "",
				Reason:     fmt.Sprintf("发现"%s"但缺少后续"%s"，建议补充完整的列举结构", p.first, p.second),
				Confidence: 0.65,
			})
		}
	}
	
	return changes
}

// llmLogicOptimize 使用LLM进行逻辑优化
func (la *LogicAgent) llmLogicOptimize(ctx context.Context, content string, articleType agent.ArticleType) ([]agent.Change, string, int, error) {
	prompt := la.buildLogicPrompt(content, articleType)
	
	result, err := la.llmManager.GenerateText(ctx, prompt, "logic")
	if err != nil {
		return nil, content, 0, err
	}
	
	changes, optimizedContent := la.parseLLMResult(content, result)
	tokens := len(prompt)/4 + len(result)/4
	
	return changes, optimizedContent, tokens, nil
}

// buildLogicPrompt 构建逻辑优化提示
func (la *LogicAgent) buildLogicPrompt(content string, articleType agent.ArticleType) string {
	typeDesc := "通用"
	if articleType == agent.ArticleTypeTech {
		typeDesc = "技术类"
	}
	
	return fmt.Sprintf(`你是一位专业的文章逻辑分析专家。请分析以下%s微信公众号文章的逻辑结构。

分析要点：
1. 论点是否清晰明确
2. 论据是否充分支持论点
3. 段落之间的逻辑关系是否顺畅
4. 是否存在逻辑跳跃或断层
5. 因果关系是否正确
6. 列举是否完整有序
7. 总结是否呼应开头

请按以下格式返回优化建议：
[问题]: 发现的逻辑问题
[位置]: 问题所在位置（段落或句子）
[建议]: 优化建议
[修改]: 如果需要具体修改，提供修改后的文本

如果逻辑结构良好，请返回"逻辑清晰，无需调整"。

文章内容：
%s`, typeDesc, content)
}

// parseLLMResult 解析LLM返回结果
func (la *LogicAgent) parseLLMResult(original, result string) ([]agent.Change, string) {
	changes := make([]agent.Change, 0)
	optimized := original
	
	if strings.Contains(result, "无需调整") || strings.Contains(result, "逻辑清晰") {
		return changes, original
	}
	
	lines := strings.Split(result, "\n")
	var currentProblem, currentLocation, currentSuggestion, currentModification string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[问题]:") || strings.HasPrefix(line, "[问题]：") {
			currentProblem = strings.TrimPrefix(strings.TrimPrefix(line, "[问题]:"), "[问题]：")
			currentProblem = strings.TrimSpace(currentProblem)
		} else if strings.HasPrefix(line, "[位置]:") || strings.HasPrefix(line, "[位置]：") {
			currentLocation = strings.TrimPrefix(strings.TrimPrefix(line, "[位置]:"), "[位置]：")
			currentLocation = strings.TrimSpace(currentLocation)
		} else if strings.HasPrefix(line, "[建议]:") || strings.HasPrefix(line, "[建议]：") {
			currentSuggestion = strings.TrimPrefix(strings.TrimPrefix(line, "[建议]:"), "[建议]：")
			currentSuggestion = strings.TrimSpace(currentSuggestion)
		} else if strings.HasPrefix(line, "[修改]:") || strings.HasPrefix(line, "[修改]：") {
			currentModification = strings.TrimPrefix(strings.TrimPrefix(line, "[修改]:"), "[修改]：")
			currentModification = strings.TrimSpace(currentModification)
			
			// 记录修改
			reason := currentProblem
			if currentSuggestion != "" {
				reason = fmt.Sprintf("%s - %s", currentProblem, currentSuggestion)
			}
			
			changes = append(changes, agent.Change{
				Type:       agent.ChangeTypeLogic,
				Original:   currentLocation,
				Modified:   currentModification,
				Reason:     reason,
				Confidence: 0.75,
			})
			
			// 重置
			currentProblem = ""
			currentLocation = ""
			currentSuggestion = ""
			currentModification = ""
		}
	}
	
	return changes, optimized
}
