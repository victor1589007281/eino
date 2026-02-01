package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// ReportGeneratorAgent generates comprehensive interview reports.
type ReportGeneratorAgent struct {
	name   string
	model  model.ToolCallingChatModel
	config *ReportGeneratorConfig
}

// ReportGeneratorConfig defines configuration for report generator.
type ReportGeneratorConfig struct {
	ReportFormat     string // markdown, html, json
	IncludeDiagrams  bool
	DiagramStyle     string // mermaid, ascii
	IncludeDetails   bool
	IncludeSuggestions bool
}

// NewReportGeneratorAgent creates a new report generator agent.
func NewReportGeneratorAgent(chatModel model.ToolCallingChatModel, cfg *ReportGeneratorConfig) (*ReportGeneratorAgent, error) {
	if cfg == nil {
		cfg = &ReportGeneratorConfig{
			ReportFormat:       "markdown",
			IncludeDiagrams:    true,
			DiagramStyle:       "mermaid",
			IncludeDetails:     true,
			IncludeSuggestions: true,
		}
	}

	return &ReportGeneratorAgent{
		name:   "report_generator",
		model:  chatModel,
		config: cfg,
	}, nil
}

// Name returns the agent name.
func (a *ReportGeneratorAgent) Name(ctx context.Context) string {
	return a.name
}

// Run executes the report generation process.
func (a *ReportGeneratorAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		// Extract all required data from input
		reportData, err := extractReportData(input)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to extract report data: %w", err),
			})
			return
		}

		// Generate the report
		report, err := a.generateReport(ctx, reportData)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to generate report: %w", err),
			})
			return
		}

		// Format report as specified
		var output string
		switch a.config.ReportFormat {
		case "json":
			outputJSON, _ := json.MarshalIndent(report, "", "  ")
			output = string(outputJSON)
		case "html":
			output = a.formatAsHTML(report)
		default:
			output = a.formatAsMarkdown(report)
		}

		generator.Send(&adk.AgentEvent{
			Output: &adk.AgentEventOutput{
				MessageOutput: schema.AssistantMessage(output, nil),
			},
			Action: &adk.AgentAction{
				Exit: true,
			},
		})
	}()

	return iterator
}

// CanHandle returns true if the agent can handle the given phase.
func (a *ReportGeneratorAgent) CanHandle(ctx context.Context, phase InterviewPhase) bool {
	return phase == PhaseReport
}

// GetCapabilities returns the capabilities of the agent.
func (a *ReportGeneratorAgent) GetCapabilities() []string {
	return []string{
		"generate_report",
		"create_diagrams",
		"format_output",
		"summarize_interview",
	}
}

type reportInputData struct {
	Session      *InterviewSession `json:"session"`
	JD           *JobDescription   `json:"job_description"`
	SkillProfile *SkillProfile     `json:"skill_profile"`
	MatchResult  *MatchResult      `json:"match_result"`
}

func extractReportData(input *adk.AgentInput) (*reportInputData, error) {
	if input == nil || len(input.Messages) == 0 {
		return nil, fmt.Errorf("no input messages")
	}

	data := &reportInputData{}

	for _, msg := range input.Messages {
		if msg.Content != "" {
			if err := json.Unmarshal([]byte(msg.Content), data); err == nil {
				break
			}
		}
	}

	if data.Session == nil {
		return nil, fmt.Errorf("missing interview session")
	}

	return data, nil
}

func (a *ReportGeneratorAgent) generateReport(ctx context.Context, data *reportInputData) (*InterviewReport, error) {
	report := &InterviewReport{
		ID:             uuid.New().String(),
		SessionID:      data.Session.ID,
		Candidate:      data.Session.Candidate,
		JobDescription: data.JD,
		SkillProfile:   data.SkillProfile,
		MatchResult:    data.MatchResult,
		GeneratedAt:    time.Now().Format(time.RFC3339),
	}

	// Calculate overall score
	report.OverallScore = a.calculateOverallScore(data)

	// Determine recommendation
	if data.MatchResult != nil {
		report.Recommendation = data.MatchResult.Recommendation
	} else {
		report.Recommendation = a.determineRecommendation(report.OverallScore)
	}

	// Generate round summaries
	report.RoundSummaries = a.generateRoundSummaries(data.Session)

	// Identify strengths and areas for improvement
	report.Strengths, report.AreasForImprovement = a.identifyKeyPoints(data)

	// Generate executive summary using LLM
	summary, err := a.generateExecutiveSummary(ctx, report)
	if err == nil {
		report.ExecutiveSummary = summary
	}

	// Generate next steps
	report.NextSteps = a.generateNextSteps(report)

	// Generate diagrams if enabled
	if a.config.IncludeDiagrams {
		report.Diagrams = a.generateDiagrams(data)
	}

	return report, nil
}

func (a *ReportGeneratorAgent) calculateOverallScore(data *reportInputData) float64 {
	if data.Session == nil || len(data.Session.Answers) == 0 {
		return 0
	}

	totalScore := 0.0
	totalWeight := 0.0

	for _, answer := range data.Session.Answers {
		if answer.Evaluation != nil {
			totalScore += answer.Evaluation.Score
			totalWeight += answer.Evaluation.MaxScore
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return (totalScore / totalWeight) * 100
}

func (a *ReportGeneratorAgent) determineRecommendation(score float64) string {
	switch {
	case score >= 85:
		return "strong_hire"
	case score >= 70:
		return "hire"
	case score >= 50:
		return "maybe"
	default:
		return "no_hire"
	}
}

func (a *ReportGeneratorAgent) generateRoundSummaries(session *InterviewSession) []RoundSummary {
	if session == nil || session.Plan == nil {
		return nil
	}

	summaries := make([]RoundSummary, 0, len(session.Plan.Rounds))

	for _, round := range session.Plan.Rounds {
		summary := RoundSummary{
			RoundID:   round.ID,
			RoundName: round.Name,
			MaxScore:  float64(len(round.Questions)) * 10,
		}

		// Calculate round score from answers
		roundScore := 0.0
		var highlights, concerns []string

		for _, q := range round.Questions {
			for _, answer := range session.Answers {
				if answer.QuestionID == q.ID && answer.Evaluation != nil {
					roundScore += answer.Evaluation.Score
					if answer.Evaluation.Score >= 8 {
						highlights = append(highlights, answer.Evaluation.Strengths...)
					}
					if answer.Evaluation.Score < 5 {
						concerns = append(concerns, answer.Evaluation.Weaknesses...)
					}
				}
			}
		}

		summary.Score = roundScore
		summary.Passed = roundScore >= summary.MaxScore*0.6
		summary.Highlights = unique(highlights)
		summary.Concerns = unique(concerns)

		summaries = append(summaries, summary)
	}

	return summaries
}

func (a *ReportGeneratorAgent) identifyKeyPoints(data *reportInputData) ([]string, []string) {
	var strengths, improvements []string

	if data.SkillProfile != nil {
		strengths = append(strengths, data.SkillProfile.Strengths...)
		improvements = append(improvements, data.SkillProfile.Weaknesses...)
	}

	// Add from match result
	if data.MatchResult != nil {
		for _, gap := range data.MatchResult.SkillGaps {
			if gap.Importance == "critical" {
				improvements = append(improvements, fmt.Sprintf("%s需要提升到%s水平", gap.Skill, gap.RequiredLevel))
			}
		}
	}

	return unique(strengths), unique(improvements)
}

func (a *ReportGeneratorAgent) generateExecutiveSummary(ctx context.Context, report *InterviewReport) (string, error) {
	prompt := fmt.Sprintf(`请为以下面试结果生成一段简洁的执行摘要（3-4句话）：

候选人: %s
职位: %s
总分: %.1f
推荐: %s
优势: %v
待提升: %v

摘要应包含：候选人整体表现、关键优势、主要关注点、最终建议。`,
		report.Candidate.Name,
		report.JobDescription.Title,
		report.OverallScore,
		report.Recommendation,
		report.Strengths,
		report.AreasForImprovement)

	messages := []*schema.Message{
		schema.SystemMessage("你是一位专业的HR顾问，请生成简洁专业的面试摘要。"),
		schema.UserMessage(prompt),
	}

	response, err := a.model.Generate(ctx, messages)
	if err != nil {
		return "", err
	}

	return response.Content, nil
}

func (a *ReportGeneratorAgent) generateNextSteps(report *InterviewReport) []string {
	var steps []string

	switch report.Recommendation {
	case "strong_hire":
		steps = []string{
			"安排与团队负责人的终面",
			"准备offer细节讨论",
			"进行背景调查",
		}
	case "hire":
		steps = []string{
			"安排复试以确认特定领域能力",
			"进行参考人调查",
			"讨论入职时间安排",
		}
	case "maybe":
		steps = []string{
			"安排针对性技术复试",
			"评估培训可能性",
			"考虑其他合适岗位",
		}
	default:
		steps = []string{
			"发送感谢邮件",
			"保持人才库联系",
			"建议后续提升方向",
		}
	}

	return steps
}

func (a *ReportGeneratorAgent) generateDiagrams(data *reportInputData) []Diagram {
	var diagrams []Diagram

	// Skill radar diagram
	if data.SkillProfile != nil && len(data.SkillProfile.Skills) > 0 {
		diagrams = append(diagrams, a.generateSkillRadar(data.SkillProfile))
	}

	// Score comparison diagram
	if data.MatchResult != nil {
		diagrams = append(diagrams, a.generateScoreBar(data))
	}

	return diagrams
}

func (a *ReportGeneratorAgent) generateSkillRadar(profile *SkillProfile) Diagram {
	var skills []string
	var values []string

	for name, score := range profile.Skills {
		skills = append(skills, name)
		values = append(values, fmt.Sprintf("%.0f", score.Score*100))
	}

	// Generate Mermaid radar chart approximation (using pie chart as radar is not supported)
	content := fmt.Sprintf(`pie title 技能分布
%s`, func() string {
		var lines []string
		for i, skill := range skills {
			lines = append(lines, fmt.Sprintf(`    "%s" : %s`, skill, values[i]))
		}
		return strings.Join(lines, "\n")
	}())

	return Diagram{
		Type:    DiagramTypeSkillRadar,
		Title:   "技能评估分布",
		Content: content,
	}
}

func (a *ReportGeneratorAgent) generateScoreBar(data *reportInputData) Diagram {
	content := `xychart-beta
    title "面试评分对比"
    x-axis [技能匹配, 经验匹配, 综合评分]
    y-axis "分数 (%)" 0 --> 100
    bar [` + fmt.Sprintf("%.0f, %.0f, %.0f",
		data.MatchResult.SkillMatch,
		data.MatchResult.ExperienceMatch,
		data.MatchResult.OverallMatch) + `]`

	return Diagram{
		Type:    DiagramTypeScoreBar,
		Title:   "匹配度评分",
		Content: content,
	}
}

func (a *ReportGeneratorAgent) formatAsMarkdown(report *InterviewReport) string {
	var sb strings.Builder

	sb.WriteString("# 📋 面试评估报告\n\n")
	sb.WriteString(fmt.Sprintf("**报告ID:** %s\n\n", report.ID))
	sb.WriteString(fmt.Sprintf("**生成时间:** %s\n\n", report.GeneratedAt))

	// Candidate Info
	sb.WriteString("## 👤 候选人信息\n\n")
	sb.WriteString(fmt.Sprintf("- **姓名:** %s\n", report.Candidate.Name))
	if report.Candidate.Email != "" {
		sb.WriteString(fmt.Sprintf("- **邮箱:** %s\n", report.Candidate.Email))
	}
	sb.WriteString("\n")

	// Position Info
	sb.WriteString("## 💼 职位信息\n\n")
	sb.WriteString(fmt.Sprintf("- **职位:** %s\n", report.JobDescription.Title))
	sb.WriteString(fmt.Sprintf("- **级别:** %s\n", report.JobDescription.Level))
	sb.WriteString("\n")

	// Executive Summary
	sb.WriteString("## 📝 执行摘要\n\n")
	sb.WriteString(report.ExecutiveSummary + "\n\n")

	// Overall Score
	sb.WriteString("## 📊 总体评分\n\n")
	sb.WriteString(fmt.Sprintf("| 指标 | 分数 |\n"))
	sb.WriteString("|------|------|\n")
	sb.WriteString(fmt.Sprintf("| **综合得分** | **%.1f%%** |\n", report.OverallScore))
	if report.MatchResult != nil {
		sb.WriteString(fmt.Sprintf("| 技能匹配度 | %.1f%% |\n", report.MatchResult.SkillMatch))
		sb.WriteString(fmt.Sprintf("| 经验匹配度 | %.1f%% |\n", report.MatchResult.ExperienceMatch))
	}
	sb.WriteString(fmt.Sprintf("| **最终建议** | **%s** |\n", translateRecommendation(report.Recommendation)))
	sb.WriteString("\n")

	// Round Summaries
	if len(report.RoundSummaries) > 0 {
		sb.WriteString("## 🎯 各轮表现\n\n")
		for _, round := range report.RoundSummaries {
			passIcon := "❌"
			if round.Passed {
				passIcon = "✅"
			}
			sb.WriteString(fmt.Sprintf("### %s %s\n\n", passIcon, round.RoundName))
			sb.WriteString(fmt.Sprintf("- 得分: %.1f / %.1f\n", round.Score, round.MaxScore))
			if len(round.Highlights) > 0 {
				sb.WriteString("- 亮点: " + strings.Join(round.Highlights[:min(3, len(round.Highlights))], ", ") + "\n")
			}
			if len(round.Concerns) > 0 {
				sb.WriteString("- 关注: " + strings.Join(round.Concerns[:min(3, len(round.Concerns))], ", ") + "\n")
			}
			sb.WriteString("\n")
		}
	}

	// Strengths
	if len(report.Strengths) > 0 {
		sb.WriteString("## ✅ 优势\n\n")
		for _, s := range report.Strengths {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
		sb.WriteString("\n")
	}

	// Areas for Improvement
	if len(report.AreasForImprovement) > 0 {
		sb.WriteString("## 📈 待提升领域\n\n")
		for _, s := range report.AreasForImprovement {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
		sb.WriteString("\n")
	}

	// Skill Gaps
	if report.MatchResult != nil && len(report.MatchResult.SkillGaps) > 0 {
		sb.WriteString("## ⚠️ 技能差距\n\n")
		sb.WriteString("| 技能 | 要求水平 | 实际水平 | 重要程度 |\n")
		sb.WriteString("|------|----------|----------|----------|\n")
		for _, gap := range report.MatchResult.SkillGaps {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				gap.Skill, gap.RequiredLevel, gap.ActualLevel, translateImportance(gap.Importance)))
		}
		sb.WriteString("\n")
	}

	// Diagrams
	if len(report.Diagrams) > 0 {
		sb.WriteString("## 📈 图表分析\n\n")
		for _, diagram := range report.Diagrams {
			sb.WriteString(fmt.Sprintf("### %s\n\n", diagram.Title))
			sb.WriteString("```mermaid\n")
			sb.WriteString(diagram.Content + "\n")
			sb.WriteString("```\n\n")
		}
	}

	// Next Steps
	if len(report.NextSteps) > 0 {
		sb.WriteString("## 🚀 建议后续步骤\n\n")
		for i, step := range report.NextSteps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("*本报告由 Interview Expert Agent 自动生成*\n")

	return sb.String()
}

func (a *ReportGeneratorAgent) formatAsHTML(report *InterviewReport) string {
	// Convert markdown to HTML (simplified)
	md := a.formatAsMarkdown(report)
	// TODO: Implement proper markdown to HTML conversion
	return "<html><body><pre>" + md + "</pre></body></html>"
}

func translateRecommendation(rec string) string {
	switch rec {
	case "strong_hire":
		return "🌟 强烈推荐录用"
	case "hire":
		return "✅ 建议录用"
	case "maybe":
		return "🤔 待定"
	case "no_hire":
		return "❌ 不建议录用"
	default:
		return rec
	}
}

func translateImportance(imp string) string {
	switch imp {
	case "critical":
		return "关键"
	case "important":
		return "重要"
	case "nice_to_have":
		return "加分项"
	default:
		return imp
	}
}

func unique(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
