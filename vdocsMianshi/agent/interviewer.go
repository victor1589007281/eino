package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// InterviewerAgent conducts the actual interview.
type InterviewerAgent struct {
	name    string
	model   model.ToolCallingChatModel
	config  *InterviewerConfig
	session *InterviewSession
}

// InterviewerConfig defines configuration for interviewer.
type InterviewerConfig struct {
	EnableAdaptive      bool
	AskFollowUps        bool
	MaxFollowUpsPerQ    int
	ProvideHints        bool
	TimeWarningThreshold float64 // percentage of time remaining to warn
	StreamResponses     bool
}

// NewInterviewerAgent creates a new interviewer agent.
func NewInterviewerAgent(chatModel model.ToolCallingChatModel, cfg *InterviewerConfig) (*InterviewerAgent, error) {
	if cfg == nil {
		cfg = &InterviewerConfig{
			EnableAdaptive:       true,
			AskFollowUps:         true,
			MaxFollowUpsPerQ:     2,
			ProvideHints:         true,
			TimeWarningThreshold: 0.2,
			StreamResponses:      true,
		}
	}

	return &InterviewerAgent{
		name:   "interviewer",
		model:  chatModel,
		config: cfg,
	}, nil
}

// Name returns the agent name.
func (a *InterviewerAgent) Name(ctx context.Context) string {
	return a.name
}

// Run executes the interview process.
func (a *InterviewerAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		// Initialize or continue session
		session, err := a.getOrCreateSession(input)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to initialize session: %w", err),
			})
			return
		}
		a.session = session

		// Process candidate answer if provided
		if hasAnswer(input) {
			answer, err := a.processAnswer(ctx, input)
			if err != nil {
				generator.Send(&adk.AgentEvent{
					Err: fmt.Errorf("failed to process answer: %w", err),
				})
				return
			}
			session.Answers = append(session.Answers, *answer)

			// Check if we should ask follow-up
			if a.shouldAskFollowUp(answer) {
				followUp := a.generateFollowUp(ctx, session, answer)
				generator.Send(&adk.AgentEvent{
					Output: &adk.AgentEventOutput{
						MessageOutput: schema.AssistantMessage(followUp, nil),
					},
				})
				return
			}

			// Move to next question
			session.CurrentQuestion++
		}

		// Check if current round is complete
		if a.isRoundComplete(session) {
			session.CurrentRound++
			session.CurrentQuestion = 0
		}

		// Check if interview is complete
		if a.isInterviewComplete(session) {
			session.Status = "completed"
			resultJSON, _ := json.Marshal(session)
			generator.Send(&adk.AgentEvent{
				Output: &adk.AgentEventOutput{
					MessageOutput: schema.AssistantMessage(string(resultJSON), nil),
				},
				Action: &adk.AgentAction{
					Exit: true,
				},
			})
			return
		}

		// Get and ask next question
		nextQuestion := a.getNextQuestion(session)
		if nextQuestion == nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("no more questions available"),
			})
			return
		}

		questionPrompt := a.formatQuestion(nextQuestion)
		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentEventOutput{
				MessageOutput: schema.AssistantMessage(questionPrompt, nil),
			},
		})
	}()

	return iterator
}

// CanHandle returns true if the agent can handle the given phase.
func (a *InterviewerAgent) CanHandle(ctx context.Context, phase InterviewPhase) bool {
	return phase == PhaseInterview
}

// GetCapabilities returns the capabilities of the agent.
func (a *InterviewerAgent) GetCapabilities() []string {
	return []string{
		"conduct_interview",
		"ask_questions",
		"process_answers",
		"generate_follow_ups",
		"manage_session",
	}
}

func (a *InterviewerAgent) getOrCreateSession(input *adk.AgentInput) (*InterviewSession, error) {
	if a.session != nil && a.session.Status == "in_progress" {
		return a.session, nil
	}

	// Extract plan and candidate from input
	plan, candidate, err := extractSessionInput(input)
	if err != nil {
		return nil, err
	}

	return &InterviewSession{
		ID:              uuid.New().String(),
		Candidate:       candidate,
		Plan:            plan,
		CurrentRound:    0,
		CurrentQuestion: 0,
		Answers:         make([]Answer, 0),
		StartTime:       time.Now().Format(time.RFC3339),
		Status:          "in_progress",
	}, nil
}

func (a *InterviewerAgent) processAnswer(ctx context.Context, input *adk.AgentInput) (*Answer, error) {
	answerContent := extractAnswerContent(input)
	if answerContent == "" {
		return nil, fmt.Errorf("no answer content found")
	}

	currentQuestion := a.getCurrentQuestion()
	if currentQuestion == nil {
		return nil, fmt.Errorf("no current question")
	}

	// Evaluate the answer
	evaluation, err := a.evaluateAnswer(ctx, currentQuestion, answerContent)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate answer: %w", err)
	}

	return &Answer{
		QuestionID: currentQuestion.ID,
		Content:    answerContent,
		Duration:   0, // TODO: Track actual duration
		Evaluation: evaluation,
		Timestamp:  time.Now().Format(time.RFC3339),
	}, nil
}

func (a *InterviewerAgent) evaluateAnswer(ctx context.Context, question *Question, answerContent string) (*AnswerEval, error) {
	prompt := fmt.Sprintf(`请评估以下面试回答：

问题: %s
期望答案要点: %s
候选人回答: %s

评估维度:
%s

请按以下JSON格式输出评估结果：
{
  "score": 0.0-10.0,
  "max_score": 10.0,
  "breakdown": {
    "维度1": 分数,
    "维度2": 分数
  },
  "strengths": ["优点1", "优点2"],
  "weaknesses": ["不足1", "不足2"],
  "notes": "评估备注",
  "skill_demonstrated": [
    {
      "skill": "技能名称",
      "level": "none/basic/intermediate/advanced/expert",
      "evidence": "具体表现",
      "score": 0.0-1.0
    }
  ]
}`,
		question.Content,
		question.ExpectedAnswer,
		answerContent,
		formatEvaluationCriteria(question.EvaluationCriteria))

	messages := []*schema.Message{
		schema.SystemMessage(evaluatorSystemPrompt),
		schema.UserMessage(prompt),
	}

	response, err := a.model.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSON(response.Content)
	if jsonStr == "" {
		return nil, fmt.Errorf("no valid JSON in evaluation response")
	}

	var eval AnswerEval
	if err := json.Unmarshal([]byte(jsonStr), &eval); err != nil {
		return nil, err
	}

	return &eval, nil
}

func (a *InterviewerAgent) getCurrentQuestion() *Question {
	if a.session == nil || a.session.Plan == nil {
		return nil
	}

	if a.session.CurrentRound >= len(a.session.Plan.Rounds) {
		return nil
	}

	round := &a.session.Plan.Rounds[a.session.CurrentRound]
	if a.session.CurrentQuestion >= len(round.Questions) {
		return nil
	}

	return &round.Questions[a.session.CurrentQuestion]
}

func (a *InterviewerAgent) getNextQuestion(session *InterviewSession) *Question {
	if session == nil || session.Plan == nil {
		return nil
	}

	if session.CurrentRound >= len(session.Plan.Rounds) {
		return nil
	}

	round := &session.Plan.Rounds[session.CurrentRound]
	if session.CurrentQuestion >= len(round.Questions) {
		return nil
	}

	return &round.Questions[session.CurrentQuestion]
}

func (a *InterviewerAgent) formatQuestion(q *Question) string {
	result := fmt.Sprintf("【问题 - %s】\n\n%s", q.Difficulty.String(), q.Content)

	if q.TimeLimit > 0 {
		result += fmt.Sprintf("\n\n⏱ 建议时间: %d分钟", q.TimeLimit)
	}

	return result
}

func (a *InterviewerAgent) shouldAskFollowUp(answer *Answer) bool {
	if !a.config.AskFollowUps {
		return false
	}

	if len(answer.FollowUpAsked) >= a.config.MaxFollowUpsPerQ {
		return false
	}

	// Check if answer needs clarification
	if answer.Evaluation != nil && answer.Evaluation.Score < 6 {
		return true
	}

	return false
}

func (a *InterviewerAgent) generateFollowUp(ctx context.Context, session *InterviewSession, answer *Answer) string {
	question := a.getCurrentQuestion()
	if question == nil || len(question.FollowUps) == 0 {
		return "能否再详细说明一下？"
	}

	// Select appropriate follow-up based on answer evaluation
	followUpIdx := len(answer.FollowUpAsked)
	if followUpIdx < len(question.FollowUps) {
		answer.FollowUpAsked = append(answer.FollowUpAsked, question.FollowUps[followUpIdx])
		return question.FollowUps[followUpIdx]
	}

	return "能否从另一个角度来解释一下？"
}

func (a *InterviewerAgent) isRoundComplete(session *InterviewSession) bool {
	if session.CurrentRound >= len(session.Plan.Rounds) {
		return true
	}

	round := &session.Plan.Rounds[session.CurrentRound]
	return session.CurrentQuestion >= len(round.Questions)
}

func (a *InterviewerAgent) isInterviewComplete(session *InterviewSession) bool {
	return session.CurrentRound >= len(session.Plan.Rounds)
}

func hasAnswer(input *adk.AgentInput) bool {
	if input == nil || len(input.Messages) == 0 {
		return false
	}

	for _, msg := range input.Messages {
		if msg.Role == schema.User && msg.Content != "" {
			// Check if it's an answer (not a command)
			if msg.Extra != nil {
				if msgType, ok := msg.Extra["type"].(string); ok && msgType == "answer" {
					return true
				}
			}
		}
	}
	return false
}

func extractAnswerContent(input *adk.AgentInput) string {
	if input == nil || len(input.Messages) == 0 {
		return ""
	}

	for _, msg := range input.Messages {
		if msg.Role == schema.User && msg.Content != "" {
			return msg.Content
		}
	}
	return ""
}

func extractSessionInput(input *adk.AgentInput) (*InterviewPlan, *Candidate, error) {
	if input == nil || len(input.Messages) == 0 {
		return nil, nil, fmt.Errorf("no input messages")
	}

	var plan *InterviewPlan
	var candidate *Candidate

	for _, msg := range input.Messages {
		if msg.Content != "" {
			// Try to parse as session init data
			var initData struct {
				Plan      *InterviewPlan `json:"plan"`
				Candidate *Candidate     `json:"candidate"`
			}
			if err := json.Unmarshal([]byte(msg.Content), &initData); err == nil {
				plan = initData.Plan
				candidate = initData.Candidate
				break
			}
		}
	}

	if plan == nil {
		return nil, nil, fmt.Errorf("no interview plan found in input")
	}

	if candidate == nil {
		candidate = &Candidate{
			ID:   uuid.New().String(),
			Name: "Unknown",
		}
	}

	return plan, candidate, nil
}

func formatEvaluationCriteria(criteria []EvaluationCriterion) string {
	if len(criteria) == 0 {
		return "- 回答的准确性\n- 表达的清晰度\n- 思路的完整性"
	}

	result := ""
	for _, c := range criteria {
		result += fmt.Sprintf("- %s (最高%v分, 权重%.0f%%): %s\n",
			c.Name, c.MaxScore, c.Weight*100, c.Description)
	}
	return result
}

const evaluatorSystemPrompt = `你是一位专业的技术面试评估专家，擅长客观评估候选人的回答。

评估原则：
1. 客观公正，基于事实评估
2. 关注回答的核心要点
3. 考虑回答的深度和广度
4. 识别候选人展示的技能水平

评分标准：
- 9-10分: 优秀，超出预期
- 7-8分: 良好，达到预期
- 5-6分: 合格，基本达到要求
- 3-4分: 不足，部分达到要求
- 1-2分: 较差，未达到基本要求

请客观评估并提供具体的优点和改进建议。`
