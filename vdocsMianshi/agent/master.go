package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// MasterInterviewerAgent orchestrates the entire interview process.
type MasterInterviewerAgent struct {
	name       string
	model      model.ToolCallingChatModel
	config     *MasterConfig
	subAgents  map[string]SubAgent
	tools      []*compose.ToolNode
}

// MasterConfig defines configuration for master agent.
type MasterConfig struct {
	MaxIterations   int
	EnableStreaming bool
	AutoProgress    bool
}

// NewMasterInterviewerAgent creates a new master interviewer agent.
func NewMasterInterviewerAgent(
	chatModel model.ToolCallingChatModel,
	cfg *MasterConfig,
	subAgents map[string]SubAgent,
) (*MasterInterviewerAgent, error) {
	if cfg == nil {
		cfg = &MasterConfig{
			MaxIterations:   30,
			EnableStreaming: true,
			AutoProgress:    true,
		}
	}

	return &MasterInterviewerAgent{
		name:      "master_interviewer",
		model:     chatModel,
		config:    cfg,
		subAgents: subAgents,
	}, nil
}

// Name returns the agent name.
func (a *MasterInterviewerAgent) Name(ctx context.Context) string {
	return a.name
}

// Run executes the complete interview workflow.
func (a *MasterInterviewerAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		// Phase 1: JD Analysis
		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentEventOutput{
				MessageOutput: schema.AssistantMessage("📋 开始分析职位描述...", nil),
			},
		})

		jdAgent, ok := a.subAgents["jd_analyzer"]
		if !ok {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("JD analyzer agent not found"),
			})
			return
		}

		jdResult, err := a.runSubAgent(ctx, jdAgent, input)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("JD analysis failed: %w", err),
			})
			return
		}

		var jd JobDescription
		if err := json.Unmarshal([]byte(jdResult), &jd); err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to parse JD result: %w", err),
			})
			return
		}

		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentEventOutput{
				MessageOutput: schema.AssistantMessage(fmt.Sprintf("✅ JD分析完成\n职位: %s\n级别: %s\n必需技能: %d项", jd.Title, jd.Level, len(jd.RequiredSkills)), nil),
			},
		})

		// Phase 2: Question Design
		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentEventOutput{
				MessageOutput: schema.AssistantMessage("📝 设计面试方案和题目...", nil),
			},
		})

		questionAgent, ok := a.subAgents["question_designer"]
		if !ok {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("question designer agent not found"),
			})
			return
		}

		questionInput := &adk.AgentInput{
			Messages: []*schema.Message{
				schema.UserMessage(jdResult),
			},
		}

		planResult, err := a.runSubAgent(ctx, questionAgent, questionInput)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("question design failed: %w", err),
			})
			return
		}

		var plan InterviewPlan
		if err := json.Unmarshal([]byte(planResult), &plan); err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to parse plan result: %w", err),
			})
			return
		}

		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentEventOutput{
				MessageOutput: schema.AssistantMessage(fmt.Sprintf("✅ 面试方案设计完成\n轮次: %d\n总时长: %d分钟", len(plan.Rounds), plan.TotalDuration), nil),
			},
		})

		// Output the complete plan
		planJSON, _ := json.MarshalIndent(struct {
			JobDescription *JobDescription `json:"job_description"`
			InterviewPlan  *InterviewPlan  `json:"interview_plan"`
		}{
			JobDescription: &jd,
			InterviewPlan:  &plan,
		}, "", "  ")

		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentEventOutput{
				MessageOutput: schema.AssistantMessage(fmt.Sprintf("```json\n%s\n```", string(planJSON)), nil),
			},
			Action: &adk.AgentAction{
				Exit: true,
			},
		})
	}()

	return iterator
}

// RunInterview runs the complete interview process including candidate interaction.
func (a *MasterInterviewerAgent) RunInterview(ctx context.Context, jd *JobDescription, plan *InterviewPlan, candidate *Candidate) (*InterviewReport, error) {
	// Create interview session
	session := &InterviewSession{
		Candidate:       candidate,
		Plan:            plan,
		CurrentRound:    0,
		CurrentQuestion: 0,
		Answers:         make([]Answer, 0),
		Status:          "in_progress",
	}

	// Get interviewer agent
	interviewerAgent, ok := a.subAgents["interviewer"]
	if !ok {
		return nil, fmt.Errorf("interviewer agent not found")
	}

	// Run interview session
	sessionJSON, _ := json.Marshal(struct {
		Plan      *InterviewPlan `json:"plan"`
		Candidate *Candidate     `json:"candidate"`
	}{
		Plan:      plan,
		Candidate: candidate,
	})

	interviewInput := &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(string(sessionJSON)),
		},
	}

	// This would typically be interactive with the candidate
	// For now, we'll simulate completion
	sessionResult, err := a.runSubAgent(ctx, interviewerAgent, interviewInput)
	if err != nil {
		return nil, fmt.Errorf("interview failed: %w", err)
	}

	if err := json.Unmarshal([]byte(sessionResult), session); err != nil {
		return nil, fmt.Errorf("failed to parse session result: %w", err)
	}

	// Run skill evaluation
	evaluatorAgent, ok := a.subAgents["skill_evaluator"]
	if !ok {
		return nil, fmt.Errorf("skill evaluator agent not found")
	}

	evalInput, _ := json.Marshal(struct {
		Session        *InterviewSession `json:"session"`
		JobDescription *JobDescription   `json:"job_description"`
	}{
		Session:        session,
		JobDescription: jd,
	})

	evalResult, err := a.runSubAgent(ctx, evaluatorAgent, &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(string(evalInput)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	var evalData struct {
		SkillProfile *SkillProfile `json:"skill_profile"`
		MatchResult  *MatchResult  `json:"match_result"`
	}
	if err := json.Unmarshal([]byte(evalResult), &evalData); err != nil {
		return nil, fmt.Errorf("failed to parse evaluation result: %w", err)
	}

	// Generate report
	reportAgent, ok := a.subAgents["report_generator"]
	if !ok {
		return nil, fmt.Errorf("report generator agent not found")
	}

	reportInput, _ := json.Marshal(struct {
		Session        *InterviewSession `json:"session"`
		JobDescription *JobDescription   `json:"job_description"`
		SkillProfile   *SkillProfile     `json:"skill_profile"`
		MatchResult    *MatchResult      `json:"match_result"`
	}{
		Session:        session,
		JobDescription: jd,
		SkillProfile:   evalData.SkillProfile,
		MatchResult:    evalData.MatchResult,
	})

	reportResult, err := a.runSubAgent(ctx, reportAgent, &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(string(reportInput)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("report generation failed: %w", err)
	}

	var report InterviewReport
	if err := json.Unmarshal([]byte(reportResult), &report); err != nil {
		// If JSON parsing fails, the result might be markdown
		report.ExecutiveSummary = reportResult
	}

	return &report, nil
}

func (a *MasterInterviewerAgent) runSubAgent(ctx context.Context, agent SubAgent, input *adk.AgentInput) (string, error) {
	iter := agent.Run(ctx, input)

	var result string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return "", event.Err
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			result = event.Output.MessageOutput.Content
		}
	}

	return result, nil
}

// CanHandle returns true for all phases as master orchestrates everything.
func (a *MasterInterviewerAgent) CanHandle(ctx context.Context, phase InterviewPhase) bool {
	return true
}

// GetCapabilities returns the capabilities of the master agent.
func (a *MasterInterviewerAgent) GetCapabilities() []string {
	return []string{
		"orchestrate_interview",
		"coordinate_agents",
		"manage_workflow",
		"track_progress",
	}
}

// CreateMasterInterviewerGraph creates a graph for the master interviewer workflow.
func CreateMasterInterviewerGraph(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	subAgents map[string]SubAgent,
) (*compose.CompiledGraph[map[string]any, *schema.Message], error) {
	graph := compose.NewGraph[map[string]any, *schema.Message]()

	// Add JD analysis node
	jdAnalyzer := compose.InvokableLambda(func(ctx context.Context, input map[string]any) ([]*schema.Message, error) {
		jdContent, ok := input["jd_content"].(string)
		if !ok {
			return nil, fmt.Errorf("missing jd_content")
		}
		return []*schema.Message{
			schema.SystemMessage(masterSystemPrompt),
			schema.UserMessage(fmt.Sprintf("请分析以下JD并设计面试方案:\n%s", jdContent)),
		}, nil
	})

	err := graph.AddLambdaNode("prepare_jd", jdAnalyzer)
	if err != nil {
		return nil, err
	}

	// Add chat model node
	err = graph.AddChatModelNode("process", chatModel)
	if err != nil {
		return nil, err
	}

	// Add edges
	err = graph.AddEdge(compose.START, "prepare_jd")
	if err != nil {
		return nil, err
	}
	err = graph.AddEdge("prepare_jd", "process")
	if err != nil {
		return nil, err
	}
	err = graph.AddEdge("process", compose.END)
	if err != nil {
		return nil, err
	}

	return graph.Compile(ctx)
}

const masterSystemPrompt = `你是一位专业的面试官Agent协调者，负责编排整个面试流程。

你的职责是：
1. 协调各个子Agent完成面试任务
2. 监控面试进度和质量
3. 确保面试流程顺利进行
4. 整合各阶段结果

面试流程：
1. JD分析 - 理解职位需求和用人意图
2. 方案设计 - 设计面试题目和评估标准
3. 面试执行 - 进行实际面试问答
4. 技能评估 - 评估候选人技能水平
5. 报告生成 - 输出完整面试报告

协调原则：
- 确保信息在各阶段正确传递
- 及时处理异常情况
- 保持面试的专业性和客观性
- 提供清晰的进度反馈`
