package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// JDAnalyzerAgent analyzes job descriptions and identifies hiring intent.
type JDAnalyzerAgent struct {
	name      string
	model     model.ToolCallingChatModel
	config    *JDAnalyzerConfig
}

// JDAnalyzerConfig defines configuration for JD analyzer.
type JDAnalyzerConfig struct {
	SkillCategories []string
	MinSkillsRequired int
}

// NewJDAnalyzerAgent creates a new JD analyzer agent.
func NewJDAnalyzerAgent(chatModel model.ToolCallingChatModel, cfg *JDAnalyzerConfig) (*JDAnalyzerAgent, error) {
	if cfg == nil {
		cfg = &JDAnalyzerConfig{
			SkillCategories: []string{
				"programming", "framework", "database", "cloud",
				"system_design", "algorithm", "soft_skills", "leadership",
			},
			MinSkillsRequired: 3,
		}
	}

	return &JDAnalyzerAgent{
		name:   "jd_analyzer",
		model:  chatModel,
		config: cfg,
	}, nil
}

// Name returns the agent name.
func (a *JDAnalyzerAgent) Name(ctx context.Context) string {
	return a.name
}

// Run executes the JD analysis.
func (a *JDAnalyzerAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		// Extract JD content from input
		jdContent := extractJDContent(input)
		if jdContent == "" {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("no JD content found in input"),
			})
			return
		}

		// Build analysis prompt
		prompt := a.buildAnalysisPrompt(jdContent)

		// Call the model
		messages := []*schema.Message{
			schema.SystemMessage(jdAnalyzerSystemPrompt),
			schema.UserMessage(prompt),
		}

		response, err := a.model.Generate(ctx, messages)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to analyze JD: %w", err),
			})
			return
		}

		// Parse the response into JobDescription
		jd, err := a.parseAnalysisResponse(response.Content, jdContent)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to parse analysis response: %w", err),
			})
			return
		}

		// Create result event
		resultJSON, _ := json.Marshal(jd)
		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentEventOutput{
				MessageOutput: schema.AssistantMessage(string(resultJSON), nil),
			},
			Action: &adk.AgentAction{
				Exit: true,
			},
		})
	}()

	return iterator
}

// CanHandle returns true if the agent can handle the given phase.
func (a *JDAnalyzerAgent) CanHandle(ctx context.Context, phase InterviewPhase) bool {
	return phase == PhaseJDAnalysis
}

// GetCapabilities returns the capabilities of the agent.
func (a *JDAnalyzerAgent) GetCapabilities() []string {
	return []string{
		"parse_jd",
		"extract_skills",
		"identify_intent",
		"analyze_requirements",
	}
}

func (a *JDAnalyzerAgent) buildAnalysisPrompt(jdContent string) string {
	return fmt.Sprintf(`请分析以下职位描述(JD)，提取关键信息并识别用人意图：

<job_description>
%s
</job_description>

请按照以下JSON格式输出分析结果：
{
  "title": "职位名称",
  "level": "级别(junior/mid/senior/lead/principal)",
  "required_skills": [
    {
      "name": "技能名称",
      "category": "分类(programming/framework/database/cloud/system_design/algorithm/soft_skills/leadership)",
      "priority": 1-3(1=必须,2=重要,3=加分项),
      "expected_level": "期望水平(beginner/intermediate/advanced/expert)",
      "weight": 0.0-1.0
    }
  ],
  "preferred_skills": [...],
  "responsibilities": ["职责1", "职责2"],
  "requirements": ["要求1", "要求2"],
  "years_experience": {"min": 0, "max": 0},
  "education_required": "学历要求",
  "keywords": ["关键词1", "关键词2"],
  "hiring_intent": {
    "primary_focus": "technical/leadership/specialist/generalist",
    "team_role": "individual_contributor/tech_lead/manager",
    "growth_potential": "high/medium/low",
    "urgency": "urgent/normal/flexible",
    "key_competencies": ["核心能力1", "核心能力2"],
    "culture_fit_traits": ["文化特质1", "文化特质2"],
    "potential_challenges": ["潜在挑战1"]
  }
}

请确保：
1. 准确识别所有必需技能和优先级
2. 分析用人意图和团队定位
3. 提取隐含的期望和要求`, jdContent)
}

func (a *JDAnalyzerAgent) parseAnalysisResponse(response string, rawContent string) (*JobDescription, error) {
	// Try to extract JSON from response
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	jd := &JobDescription{
		ID:         uuid.New().String(),
		RawContent: rawContent,
	}

	if err := json.Unmarshal([]byte(jsonStr), jd); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Set default values for required fields
	if len(jd.RequiredSkills) == 0 {
		return nil, fmt.Errorf("no required skills found in JD")
	}

	// Calculate weights if not set
	a.normalizeSkillWeights(jd)

	return jd, nil
}

func (a *JDAnalyzerAgent) normalizeSkillWeights(jd *JobDescription) {
	totalWeight := 0.0
	for i := range jd.RequiredSkills {
		if jd.RequiredSkills[i].Weight == 0 {
			// Set default weight based on priority
			switch jd.RequiredSkills[i].Priority {
			case 1:
				jd.RequiredSkills[i].Weight = 1.0
			case 2:
				jd.RequiredSkills[i].Weight = 0.7
			case 3:
				jd.RequiredSkills[i].Weight = 0.4
			default:
				jd.RequiredSkills[i].Weight = 0.5
			}
		}
		totalWeight += jd.RequiredSkills[i].Weight
	}

	// Normalize weights to sum to 1
	if totalWeight > 0 {
		for i := range jd.RequiredSkills {
			jd.RequiredSkills[i].Weight /= totalWeight
		}
	}
}

func extractJDContent(input *adk.AgentInput) string {
	if input == nil || len(input.Messages) == 0 {
		return ""
	}

	// Look for JD content in messages
	for _, msg := range input.Messages {
		if msg.Role == schema.User && msg.Content != "" {
			return msg.Content
		}
	}
	return ""
}

func extractJSON(text string) string {
	// Try to find JSON block
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(text); i++ {
		if text[i] == '{' {
			depth++
		} else if text[i] == '}' {
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

// CreateJDAnalyzerGraph creates a graph for JD analysis.
func CreateJDAnalyzerGraph(ctx context.Context, chatModel model.ToolCallingChatModel) (*compose.CompiledGraph[map[string]any, *schema.Message], error) {
	graph := compose.NewGraph[map[string]any, *schema.Message]()

	// Add JD parser node
	jdParserLambda := compose.InvokableLambda(func(ctx context.Context, input map[string]any) ([]*schema.Message, error) {
		jdContent, ok := input["jd_content"].(string)
		if !ok {
			return nil, fmt.Errorf("missing jd_content in input")
		}

		return []*schema.Message{
			schema.SystemMessage(jdAnalyzerSystemPrompt),
			schema.UserMessage(fmt.Sprintf("请分析以下JD:\n%s", jdContent)),
		}, nil
	})

	err := graph.AddLambdaNode("prepare_messages", jdParserLambda)
	if err != nil {
		return nil, err
	}

	// Add chat model node
	err = graph.AddChatModelNode("analyze", chatModel)
	if err != nil {
		return nil, err
	}

	// Add edges
	err = graph.AddEdge(compose.START, "prepare_messages")
	if err != nil {
		return nil, err
	}
	err = graph.AddEdge("prepare_messages", "analyze")
	if err != nil {
		return nil, err
	}
	err = graph.AddEdge("analyze", compose.END)
	if err != nil {
		return nil, err
	}

	return graph.Compile(ctx)
}

const jdAnalyzerSystemPrompt = `你是一位专业的招聘分析专家，擅长分析职位描述(JD)并识别用人意图。

你的职责是：
1. 准确解析JD中的各项要求
2. 识别必需技能和优先级
3. 分析隐含的用人意图和团队需求
4. 评估职位的发展潜力和挑战

分析原则：
- 区分"必须"和"优先"的技能要求
- 识别技术深度vs广度的期望
- 判断是执行者还是领导者角色
- 发现潜在的团队文化暗示

输出格式要求：
- 使用结构化JSON格式
- 技能分类要准确
- 权重分配要合理
- 用人意图分析要有洞察力`
