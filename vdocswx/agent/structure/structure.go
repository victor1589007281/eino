// Package structure 实现结构调整子Agent
package structure

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/llm"
)

// StructureAgent 结构调整Agent
type StructureAgent struct {
	llmManager *llm.LLMManager
}

// ArticleStructure 文章结构
type ArticleStructure struct {
	Title         string
	Sections      []Section
	HasIntro      bool
	HasConclusion bool
	TotalWords    int
}

// Section 章节
type Section struct {
	Level       int
	Title       string
	Content     string
	Words       int
	SubSections []Section
}

// NewStructureAgent 创建结构调整Agent
func NewStructureAgent(llmMgr *llm.LLMManager) *StructureAgent {
	return &StructureAgent{
		llmManager: llmMgr,
	}
}

// Name 返回Agent名称
func (sa *StructureAgent) Name() string {
	return "structure"
}

// CanHandle 判断是否能处理该类型文章
func (sa *StructureAgent) CanHandle(articleType agent.ArticleType) bool {
	return true
}

// Process 处理文章内容
func (sa *StructureAgent) Process(ctx context.Context, content string, articleType agent.ArticleType) (*agent.SubAgentResult, error) {
	startTime := time.Now()
	
	result := &agent.SubAgentResult{
		AgentName: sa.Name(),
		Success:   true,
		Changes:   make([]agent.Change, 0),
	}
	
	// 1. 解析文章结构
	structure := sa.parseStructure(content)
	
	// 2. 基础结构检查
	basicChanges := sa.basicStructureCheck(structure, articleType)
	result.Changes = append(result.Changes, basicChanges...)
	
	// 3. 使用LLM进行深度结构分析（如果可用）
	if sa.llmManager != nil {
		llmChanges, llmOutput, tokens, err := sa.llmStructureOptimize(ctx, content, structure, articleType)
		if err == nil {
			result.Changes = append(result.Changes, llmChanges...)
			result.Output = llmOutput
			result.TokensUsed = tokens
		} else {
			result.Output = content
		}
	} else {
		result.Output = content
	}
	
	result.Duration = time.Since(startTime).Milliseconds()
	
	return result, nil
}

// parseStructure 解析文章结构
func (sa *StructureAgent) parseStructure(content string) *ArticleStructure {
	structure := &ArticleStructure{
		Sections:   make([]Section, 0),
		TotalWords: len([]rune(content)),
	}
	
	lines := strings.Split(content, "\n")
	
	// 标题正则
	h1Pattern := regexp.MustCompile(`^#\s+(.+)$`)
	h2Pattern := regexp.MustCompile(`^##\s+(.+)$`)
	
	var currentSection *Section
	var contentBuilder strings.Builder
	
	for i, line := range lines {
		if match := h1Pattern.FindStringSubmatch(line); len(match) > 1 {
			structure.Title = match[1]
		} else if match := h2Pattern.FindStringSubmatch(line); len(match) > 1 {
			// 保存上一个section
			if currentSection != nil {
				currentSection.Content = contentBuilder.String()
				currentSection.Words = len([]rune(currentSection.Content))
				structure.Sections = append(structure.Sections, *currentSection)
			}
			
			currentSection = &Section{
				Level: 2,
				Title: match[1],
			}
			contentBuilder.Reset()
		} else {
			if i == 0 && structure.Title == "" {
				// 第一行可能是标题（非markdown格式）
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 0 && len(trimmed) < 100 {
					structure.Title = trimmed
				}
			}
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		}
	}
	
	// 保存最后一个section
	if currentSection != nil {
		currentSection.Content = contentBuilder.String()
		currentSection.Words = len([]rune(currentSection.Content))
		structure.Sections = append(structure.Sections, *currentSection)
	}
	
	// 检查是否有引言和结语
	if len(structure.Sections) > 0 {
		firstSection := strings.ToLower(structure.Sections[0].Title)
		lastSection := strings.ToLower(structure.Sections[len(structure.Sections)-1].Title)
		
		introKeywords := []string{"引言", "前言", "背景", "introduction", "overview"}
		conclusionKeywords := []string{"结语", "总结", "结论", "conclusion", "summary"}
		
		for _, kw := range introKeywords {
			if strings.Contains(firstSection, kw) {
				structure.HasIntro = true
				break
			}
		}
		
		for _, kw := range conclusionKeywords {
			if strings.Contains(lastSection, kw) {
				structure.HasConclusion = true
				break
			}
		}
	}
	
	return structure
}

// basicStructureCheck 基础结构检查
func (sa *StructureAgent) basicStructureCheck(structure *ArticleStructure, articleType agent.ArticleType) []agent.Change {
	changes := make([]agent.Change, 0)
	
	// 检查标题
	if structure.Title == "" {
		changes = append(changes, agent.Change{
			Type:       agent.ChangeTypeStructure,
			Original:   "",
			Modified:   "",
			Reason:     "文章缺少标题，建议添加一个吸引人的标题",
			Confidence: 0.9,
		})
	}
	
	// 检查章节数量
	if structure.TotalWords > 1000 && len(structure.Sections) < 2 {
		changes = append(changes, agent.Change{
			Type:       agent.ChangeTypeStructure,
			Original:   "",
			Modified:   "",
			Reason:     "文章较长但缺少章节划分，建议使用小标题分隔内容，提高可读性",
			Confidence: 0.85,
		})
	}
	
	// 检查引言和结语
	if structure.TotalWords > 500 && !structure.HasIntro {
		changes = append(changes, agent.Change{
			Type:       agent.ChangeTypeStructure,
			Original:   "",
			Modified:   "",
			Reason:     "建议添加引言部分，引出文章主题，吸引读者继续阅读",
			Confidence: 0.7,
		})
	}
	
	if structure.TotalWords > 800 && !structure.HasConclusion {
		changes = append(changes, agent.Change{
			Type:       agent.ChangeTypeStructure,
			Original:   "",
			Modified:   "",
			Reason:     "建议添加总结部分，归纳要点，加深读者印象",
			Confidence: 0.7,
		})
	}
	
	return changes
}

// llmStructureOptimize 使用LLM进行结构优化
func (sa *StructureAgent) llmStructureOptimize(ctx context.Context, content string, structure *ArticleStructure, articleType agent.ArticleType) ([]agent.Change, string, int, error) {
	prompt := sa.buildStructurePrompt(content, structure, articleType)
	
	result, err := sa.llmManager.GenerateText(ctx, prompt, "structure")
	if err != nil {
		return nil, content, 0, err
	}
	
	changes, optimizedContent := sa.parseLLMResult(content, result)
	tokens := len(prompt)/4 + len(result)/4
	
	return changes, optimizedContent, tokens, nil
}

// buildStructurePrompt 构建结构优化提示
func (sa *StructureAgent) buildStructurePrompt(content string, structure *ArticleStructure, articleType agent.ArticleType) string {
	typeDesc := "通用"
	if articleType == agent.ArticleTypeTech {
		typeDesc = "技术类"
	}
	
	// 构建当前结构描述
	var structureDesc strings.Builder
	structureDesc.WriteString(fmt.Sprintf("标题: %s\n", structure.Title))
	structureDesc.WriteString(fmt.Sprintf("总字数: %d\n", structure.TotalWords))
	structureDesc.WriteString("章节:\n")
	for i, section := range structure.Sections {
		structureDesc.WriteString(fmt.Sprintf("  %d. %s (%d字)\n", i+1, section.Title, section.Words))
	}
	
	return fmt.Sprintf(`你是一位专业的文章结构优化专家。请分析以下%s微信公众号文章的结构。

当前文章结构:
%s

微信公众号文章结构建议:
1. 标题应吸引眼球，15-30字为宜
2. 开头要有吸引力，快速切入主题
3. 正文分3-5个小节为宜
4. 每节300-500字，段落短小
5. 有清晰的过渡和总结

请分析并提供优化建议：
[问题]: 发现的结构问题
[建议]: 具体的优化建议

如果结构良好，请返回"结构合理，无需调整"。

文章内容：
%s`, typeDesc, structureDesc.String(), content)
}

// parseLLMResult 解析LLM返回结果
func (sa *StructureAgent) parseLLMResult(original, result string) ([]agent.Change, string) {
	changes := make([]agent.Change, 0)
	
	if strings.Contains(result, "无需调整") || strings.Contains(result, "结构合理") {
		return changes, original
	}
	
	lines := strings.Split(result, "\n")
	var currentProblem, currentSuggestion string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[问题]:") || strings.HasPrefix(line, "[问题]：") {
			currentProblem = strings.TrimPrefix(strings.TrimPrefix(line, "[问题]:"), "[问题]：")
			currentProblem = strings.TrimSpace(currentProblem)
		} else if strings.HasPrefix(line, "[建议]:") || strings.HasPrefix(line, "[建议]：") {
			currentSuggestion = strings.TrimPrefix(strings.TrimPrefix(line, "[建议]:"), "[建议]：")
			currentSuggestion = strings.TrimSpace(currentSuggestion)
			
			if currentProblem != "" {
				changes = append(changes, agent.Change{
					Type:       agent.ChangeTypeStructure,
					Original:   "",
					Modified:   "",
					Reason:     fmt.Sprintf("%s - %s", currentProblem, currentSuggestion),
					Confidence: 0.75,
				})
			}
			
			currentProblem = ""
			currentSuggestion = ""
		}
	}
	
	return changes, original
}
