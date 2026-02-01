package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// QuestionDesignerAgent designs interview questions based on JD analysis.
type QuestionDesignerAgent struct {
	name   string
	model  model.ToolCallingChatModel
	config *QuestionDesignerConfig
}

// QuestionDesignerConfig defines configuration for question designer.
type QuestionDesignerConfig struct {
	MaxQuestionsPerRound int
	DifficultyMix        map[QuestionDifficulty]float64
	QuestionTypeMix      map[QuestionType]float64
	EnableFollowUps      bool
	IncludeHints         bool
}

// NewQuestionDesignerAgent creates a new question designer agent.
func NewQuestionDesignerAgent(chatModel model.ToolCallingChatModel, cfg *QuestionDesignerConfig) (*QuestionDesignerAgent, error) {
	if cfg == nil {
		cfg = &QuestionDesignerConfig{
			MaxQuestionsPerRound: 5,
			DifficultyMix: map[QuestionDifficulty]float64{
				DifficultyBasic:        0.2,
				DifficultyIntermediate: 0.4,
				DifficultyAdvanced:     0.3,
				DifficultyExpert:       0.1,
			},
			QuestionTypeMix: map[QuestionType]float64{
				QuestionTypeTechnical:         0.4,
				QuestionTypeBehavioral:        0.2,
				QuestionTypeScenario:          0.2,
				QuestionTypeSystemDesign:      0.1,
				QuestionTypeProjectExperience: 0.1,
			},
			EnableFollowUps: true,
			IncludeHints:    true,
		}
	}

	return &QuestionDesignerAgent{
		name:   "question_designer",
		model:  chatModel,
		config: cfg,
	}, nil
}

// Name returns the agent name.
func (a *QuestionDesignerAgent) Name(ctx context.Context) string {
	return a.name
}

// Run executes the question design process.
func (a *QuestionDesignerAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		// Extract JD from input
		jd, err := extractJDFromInput(input)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to extract JD: %w", err),
			})
			return
		}

		// Design interview plan
		plan, err := a.designInterviewPlan(ctx, jd)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to design interview plan: %w", err),
			})
			return
		}

		// Create result event
		resultJSON, _ := json.Marshal(plan)
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
func (a *QuestionDesignerAgent) CanHandle(ctx context.Context, phase InterviewPhase) bool {
	return phase == PhaseQuestionDesign
}

// GetCapabilities returns the capabilities of the agent.
func (a *QuestionDesignerAgent) GetCapabilities() []string {
	return []string{
		"design_questions",
		"create_interview_plan",
		"generate_follow_ups",
		"set_evaluation_criteria",
	}
}

func (a *QuestionDesignerAgent) designInterviewPlan(ctx context.Context, jd *JobDescription) (*InterviewPlan, error) {
	// Build prompt for designing questions
	prompt := a.buildDesignPrompt(jd)

	messages := []*schema.Message{
		schema.SystemMessage(questionDesignerSystemPrompt),
		schema.UserMessage(prompt),
	}

	response, err := a.model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate interview plan: %w", err)
	}

	// Parse response into InterviewPlan
	plan, err := a.parseDesignResponse(response.Content, jd)
	if err != nil {
		return nil, fmt.Errorf("failed to parse design response: %w", err)
	}

	return plan, nil
}

func (a *QuestionDesignerAgent) buildDesignPrompt(jd *JobDescription) string {
	skillsJSON, _ := json.Marshal(jd.RequiredSkills)
	intentJSON, _ := json.Marshal(jd.HiringIntent)

	return fmt.Sprintf(`基于以下职位分析结果，设计一套完整的面试方案：

职位: %s
级别: %s
必需技能: %s
用人意图: %s
工作职责: %v

请设计面试方案，输出JSON格式：
{
  "total_duration_minutes": 60,
  "rounds": [
    {
      "name": "轮次名称",
      "type": "screening/technical/behavioral/system_design",
      "duration_minutes": 15,
      "focus_skills": ["技能1", "技能2"],
      "pass_criteria": "通过标准描述",
      "questions": [
        {
          "content": "问题内容",
          "type": "technical/behavioral/scenario/coding/system_design/project_experience/soft_skills",
          "difficulty": "basic/intermediate/advanced/expert",
          "category": "分类",
          "skills": ["相关技能"],
          "expected_answer": "期望答案要点",
          "follow_ups": ["追问1", "追问2"],
          "evaluation_criteria": [
            {
              "name": "评估维度",
              "description": "描述",
              "max_score": 10,
              "weight": 0.3
            }
          ],
          "time_limit_minutes": 5,
          "weight": 0.2,
          "hints": ["提示1"]
        }
      ]
    }
  ],
  "focus_areas": [
    {
      "name": "重点领域",
      "weight": 0.3,
      "skills": ["技能1"],
      "min_score": 0.6
    }
  ],
  "difficulty_level": "intermediate"
}

设计原则：
1. 问题覆盖所有必需技能
2. 难度由浅入深，逐步递进
3. 包含追问以深入评估
4. 设置明确的评估标准
5. 时间分配合理`, jd.Title, jd.Level, string(skillsJSON), string(intentJSON), jd.Responsibilities)
}

func (a *QuestionDesignerAgent) parseDesignResponse(response string, jd *JobDescription) (*InterviewPlan, error) {
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	plan := &InterviewPlan{
		ID:               uuid.New().String(),
		JobDescriptionID: jd.ID,
	}

	if err := json.Unmarshal([]byte(jsonStr), plan); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Generate IDs for rounds and questions
	for i := range plan.Rounds {
		plan.Rounds[i].ID = uuid.New().String()
		for j := range plan.Rounds[i].Questions {
			plan.Rounds[i].Questions[j].ID = uuid.New().String()
		}
	}

	return plan, nil
}

func extractJDFromInput(input *adk.AgentInput) (*JobDescription, error) {
	if input == nil || len(input.Messages) == 0 {
		return nil, fmt.Errorf("no input messages")
	}

	for _, msg := range input.Messages {
		if msg.Content != "" {
			jd := &JobDescription{}
			if err := json.Unmarshal([]byte(msg.Content), jd); err == nil {
				return jd, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid JD found in input")
}

const questionDesignerSystemPrompt = `你是一位资深的技术面试官和面试设计专家，擅长根据职位要求设计高质量的面试题目。

你的职责是：
1. 根据JD分析结果设计面试方案
2. 创建覆盖各项技能的面试题目
3. 设计合理的难度梯度
4. 制定明确的评估标准

设计原则：
- 题目要具有区分度，能有效识别候选人水平
- 难度要符合职位级别要求
- 每个技能点至少有一道相关题目
- 追问要能深入挖掘候选人能力
- 评估标准要客观、可量化

题目类型说明：
- technical: 技术知识类，考察理论和实践
- behavioral: 行为面试类，考察过往经历和处理方式
- scenario: 场景题，考察解决问题能力
- coding: 编程题，考察实际编码能力
- system_design: 系统设计题，考察架构能力
- project_experience: 项目经验，深入了解过往项目
- soft_skills: 软技能，考察沟通协作等能力

输出要求：
- 使用结构化JSON格式
- 题目内容清晰明确
- 评估标准具体可执行
- 时间分配合理`
