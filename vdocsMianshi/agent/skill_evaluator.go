package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// SkillEvaluatorAgent evaluates candidate skills and generates match result.
type SkillEvaluatorAgent struct {
	name   string
	model  model.ToolCallingChatModel
	config *SkillEvaluatorConfig
}

// SkillEvaluatorConfig defines configuration for skill evaluator.
type SkillEvaluatorConfig struct {
	MinConfidence       float64
	WeightByEvidence    bool
	IncludeRecommendations bool
}

// NewSkillEvaluatorAgent creates a new skill evaluator agent.
func NewSkillEvaluatorAgent(chatModel model.ToolCallingChatModel, cfg *SkillEvaluatorConfig) (*SkillEvaluatorAgent, error) {
	if cfg == nil {
		cfg = &SkillEvaluatorConfig{
			MinConfidence:          0.6,
			WeightByEvidence:       true,
			IncludeRecommendations: true,
		}
	}

	return &SkillEvaluatorAgent{
		name:   "skill_evaluator",
		model:  chatModel,
		config: cfg,
	}, nil
}

// Name returns the agent name.
func (a *SkillEvaluatorAgent) Name(ctx context.Context) string {
	return a.name
}

// Run executes the skill evaluation process.
func (a *SkillEvaluatorAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	go func() {
		defer generator.Close()

		// Extract session data from input
		session, jd, err := extractEvaluationInput(input)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to extract evaluation input: %w", err),
			})
			return
		}

		// Generate skill profile
		skillProfile, err := a.generateSkillProfile(ctx, session)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to generate skill profile: %w", err),
			})
			return
		}

		// Calculate match result
		matchResult, err := a.calculateMatchResult(ctx, skillProfile, jd)
		if err != nil {
			generator.Send(&adk.AgentEvent{
				Err: fmt.Errorf("failed to calculate match result: %w", err),
			})
			return
		}

		// Create result
		result := struct {
			SkillProfile *SkillProfile `json:"skill_profile"`
			MatchResult  *MatchResult  `json:"match_result"`
		}{
			SkillProfile: skillProfile,
			MatchResult:  matchResult,
		}

		resultJSON, _ := json.Marshal(result)
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
func (a *SkillEvaluatorAgent) CanHandle(ctx context.Context, phase InterviewPhase) bool {
	return phase == PhaseEvaluation
}

// GetCapabilities returns the capabilities of the agent.
func (a *SkillEvaluatorAgent) GetCapabilities() []string {
	return []string{
		"evaluate_skills",
		"calculate_match",
		"identify_gaps",
		"generate_recommendations",
	}
}

func (a *SkillEvaluatorAgent) generateSkillProfile(ctx context.Context, session *InterviewSession) (*SkillProfile, error) {
	// Aggregate skill demonstrations from all answers
	skillScores := make(map[string]*SkillScore)

	for _, answer := range session.Answers {
		if answer.Evaluation == nil {
			continue
		}

		for _, demo := range answer.Evaluation.SkillDemonstrated {
			if existing, ok := skillScores[demo.Skill]; ok {
				// Update existing skill score
				existing.Score = (existing.Score*float64(existing.QuestionCount) + demo.Score) / float64(existing.QuestionCount+1)
				existing.QuestionCount++
				existing.Evidence = append(existing.Evidence, demo.Evidence)
			} else {
				skillScores[demo.Skill] = &SkillScore{
					Name:          demo.Skill,
					Score:         demo.Score,
					MaxScore:      1.0,
					Level:         demo.Level,
					Confidence:    0.5, // Initial confidence
					QuestionCount: 1,
					Evidence:      []string{demo.Evidence},
				}
			}
		}
	}

	// Calculate confidence based on question count
	for _, score := range skillScores {
		score.Confidence = math.Min(1.0, 0.5+float64(score.QuestionCount)*0.1)
	}

	// Identify strengths and weaknesses
	strengths, weaknesses := a.identifyStrengthsWeaknesses(skillScores)

	// Generate recommendations using LLM
	recommendations, err := a.generateRecommendations(ctx, skillScores, strengths, weaknesses)
	if err != nil {
		recommendations = []string{"无法生成建议"}
	}

	return &SkillProfile{
		CandidateID:     session.Candidate.ID,
		Skills:          skillScores,
		Strengths:       strengths,
		Weaknesses:      weaknesses,
		Recommendations: recommendations,
		OverallLevel:    a.calculateOverallLevel(skillScores),
	}, nil
}

func (a *SkillEvaluatorAgent) identifyStrengthsWeaknesses(skillScores map[string]*SkillScore) ([]string, []string) {
	type skillRank struct {
		name  string
		score float64
	}

	var rankings []skillRank
	for name, score := range skillScores {
		rankings = append(rankings, skillRank{name, score.Score})
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].score > rankings[j].score
	})

	var strengths, weaknesses []string

	// Top skills are strengths
	for i := 0; i < len(rankings) && i < 3; i++ {
		if rankings[i].score >= 0.7 {
			strengths = append(strengths, rankings[i].name)
		}
	}

	// Bottom skills are weaknesses
	for i := len(rankings) - 1; i >= 0 && len(rankings)-1-i < 3; i-- {
		if rankings[i].score < 0.5 {
			weaknesses = append(weaknesses, rankings[i].name)
		}
	}

	return strengths, weaknesses
}

func (a *SkillEvaluatorAgent) generateRecommendations(ctx context.Context, skillScores map[string]*SkillScore, strengths, weaknesses []string) ([]string, error) {
	if !a.config.IncludeRecommendations {
		return nil, nil
	}

	skillSummary, _ := json.Marshal(skillScores)
	prompt := fmt.Sprintf(`基于以下技能评估结果，为候选人生成职业发展建议：

技能评分: %s
优势: %v
待提升: %v

请提供3-5条具体、可执行的发展建议，直接输出JSON数组格式：
["建议1", "建议2", "建议3"]`, string(skillSummary), strengths, weaknesses)

	messages := []*schema.Message{
		schema.SystemMessage("你是一位职业发展顾问，擅长根据技能评估结果提供发展建议。"),
		schema.UserMessage(prompt),
	}

	response, err := a.model.Generate(ctx, messages)
	if err != nil {
		return nil, err
	}

	var recommendations []string
	jsonStr := extractJSON(response.Content)
	if jsonStr != "" {
		json.Unmarshal([]byte(jsonStr), &recommendations)
	}

	return recommendations, nil
}

func (a *SkillEvaluatorAgent) calculateOverallLevel(skillScores map[string]*SkillScore) string {
	if len(skillScores) == 0 {
		return "unknown"
	}

	totalScore := 0.0
	count := 0
	for _, score := range skillScores {
		totalScore += score.Score
		count++
	}

	avgScore := totalScore / float64(count)

	switch {
	case avgScore >= 0.9:
		return "expert"
	case avgScore >= 0.7:
		return "advanced"
	case avgScore >= 0.5:
		return "intermediate"
	case avgScore >= 0.3:
		return "basic"
	default:
		return "beginner"
	}
}

func (a *SkillEvaluatorAgent) calculateMatchResult(ctx context.Context, profile *SkillProfile, jd *JobDescription) (*MatchResult, error) {
	result := &MatchResult{
		CandidateID:      profile.CandidateID,
		JobDescriptionID: jd.ID,
	}

	// Calculate skill match
	skillMatch, gaps, surplus := a.calculateSkillMatch(profile, jd)
	result.SkillMatch = skillMatch
	result.SkillGaps = gaps
	result.SkillSurplus = surplus

	// Calculate experience match (simplified)
	result.ExperienceMatch = 0.8 // TODO: Implement based on actual experience data

	// Calculate overall match
	result.OverallMatch = skillMatch*0.7 + result.ExperienceMatch*0.3

	// Determine recommendation
	result.Recommendation = a.determineRecommendation(result.OverallMatch, len(gaps))
	result.Confidence = a.calculateMatchConfidence(profile, jd)

	// Generate notes using LLM
	notes, err := a.generateMatchNotes(ctx, result, profile, jd)
	if err == nil {
		result.Notes = notes
	}

	return result, nil
}

func (a *SkillEvaluatorAgent) calculateSkillMatch(profile *SkillProfile, jd *JobDescription) (float64, []SkillGap, []string) {
	totalWeight := 0.0
	weightedScore := 0.0
	var gaps []SkillGap
	var surplus []string

	requiredSkills := make(map[string]Skill)
	for _, skill := range jd.RequiredSkills {
		requiredSkills[skill.Name] = skill
	}

	// Check required skills
	for skillName, required := range requiredSkills {
		totalWeight += required.Weight

		if actual, ok := profile.Skills[skillName]; ok {
			// Candidate has this skill
			levelScore := a.levelToScore(actual.Level)
			requiredScore := a.levelToScore(required.ExpectedLevel)

			if levelScore >= requiredScore {
				weightedScore += required.Weight * 1.0
			} else {
				weightedScore += required.Weight * (levelScore / requiredScore)
				gaps = append(gaps, SkillGap{
					Skill:         skillName,
					RequiredLevel: required.ExpectedLevel,
					ActualLevel:   actual.Level,
					Importance:    a.priorityToImportance(required.Priority),
				})
			}
		} else {
			// Candidate doesn't have this skill
			gaps = append(gaps, SkillGap{
				Skill:         skillName,
				RequiredLevel: required.ExpectedLevel,
				ActualLevel:   "none",
				Importance:    a.priorityToImportance(required.Priority),
			})
		}
	}

	// Check for surplus skills
	for skillName := range profile.Skills {
		if _, required := requiredSkills[skillName]; !required {
			surplus = append(surplus, skillName)
		}
	}

	if totalWeight == 0 {
		return 0, gaps, surplus
	}

	return (weightedScore / totalWeight) * 100, gaps, surplus
}

func (a *SkillEvaluatorAgent) levelToScore(level string) float64 {
	switch level {
	case "expert":
		return 1.0
	case "advanced":
		return 0.8
	case "intermediate":
		return 0.6
	case "basic", "beginner":
		return 0.4
	case "none", "":
		return 0.0
	default:
		return 0.3
	}
}

func (a *SkillEvaluatorAgent) priorityToImportance(priority int) string {
	switch priority {
	case 1:
		return "critical"
	case 2:
		return "important"
	default:
		return "nice_to_have"
	}
}

func (a *SkillEvaluatorAgent) determineRecommendation(overallMatch float64, gapCount int) string {
	if overallMatch >= 85 && gapCount == 0 {
		return "strong_hire"
	} else if overallMatch >= 70 && gapCount <= 2 {
		return "hire"
	} else if overallMatch >= 50 {
		return "maybe"
	}
	return "no_hire"
}

func (a *SkillEvaluatorAgent) calculateMatchConfidence(profile *SkillProfile, jd *JobDescription) float64 {
	// Confidence based on coverage of required skills
	requiredCount := len(jd.RequiredSkills)
	evaluatedCount := 0

	for _, skill := range jd.RequiredSkills {
		if _, ok := profile.Skills[skill.Name]; ok {
			evaluatedCount++
		}
	}

	if requiredCount == 0 {
		return 0.5
	}

	return float64(evaluatedCount) / float64(requiredCount)
}

func (a *SkillEvaluatorAgent) generateMatchNotes(ctx context.Context, result *MatchResult, profile *SkillProfile, jd *JobDescription) (string, error) {
	prompt := fmt.Sprintf(`基于以下匹配分析，生成简短的评估说明：

职位: %s
匹配度: %.1f%%
技能匹配: %.1f%%
技能差距数: %d
建议: %s

请生成2-3句话的评估说明，突出关键发现。`,
		jd.Title, result.OverallMatch, result.SkillMatch, len(result.SkillGaps), result.Recommendation)

	messages := []*schema.Message{
		schema.SystemMessage("你是一位招聘评估专家，请简洁地总结匹配分析结果。"),
		schema.UserMessage(prompt),
	}

	response, err := a.model.Generate(ctx, messages)
	if err != nil {
		return "", err
	}

	return response.Content, nil
}

func extractEvaluationInput(input *adk.AgentInput) (*InterviewSession, *JobDescription, error) {
	if input == nil || len(input.Messages) == 0 {
		return nil, nil, fmt.Errorf("no input messages")
	}

	var session *InterviewSession
	var jd *JobDescription

	for _, msg := range input.Messages {
		if msg.Content != "" {
			var data struct {
				Session        *InterviewSession `json:"session"`
				JobDescription *JobDescription   `json:"job_description"`
			}
			if err := json.Unmarshal([]byte(msg.Content), &data); err == nil {
				session = data.Session
				jd = data.JobDescription
				break
			}
		}
	}

	if session == nil {
		return nil, nil, fmt.Errorf("no interview session found")
	}

	if jd == nil {
		return nil, nil, fmt.Errorf("no job description found")
	}

	return session, jd, nil
}
