package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ScoreCalculatorTool calculates interview scores.
type ScoreCalculatorTool struct {
	config *ScoreConfig
}

// ScoreConfig defines scoring configuration.
type ScoreConfig struct {
	PassThreshold      float64            `json:"pass_threshold"`
	WeightByDifficulty map[string]float64 `json:"weight_by_difficulty"`
	WeightByRound      map[string]float64 `json:"weight_by_round"`
}

// ScoreInput represents the input for score calculation.
type ScoreInput struct {
	Answers []AnswerScore `json:"answers"`
	Rounds  []RoundInfo   `json:"rounds,omitempty"`
}

// AnswerScore represents a single answer's score.
type AnswerScore struct {
	QuestionID  string  `json:"question_id"`
	RoundID     string  `json:"round_id"`
	Score       float64 `json:"score"`
	MaxScore    float64 `json:"max_score"`
	Difficulty  string  `json:"difficulty"`
	Weight      float64 `json:"weight,omitempty"`
	SkillScores map[string]float64 `json:"skill_scores,omitempty"`
}

// RoundInfo represents round information.
type RoundInfo struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

// ScoreOutput represents the calculated scores.
type ScoreOutput struct {
	TotalScore      float64            `json:"total_score"`
	TotalMaxScore   float64            `json:"total_max_score"`
	Percentage      float64            `json:"percentage"`
	Passed          bool               `json:"passed"`
	RoundScores     map[string]RoundScore `json:"round_scores"`
	SkillScores     map[string]SkillScoreResult `json:"skill_scores"`
	DifficultyBreakdown map[string]float64 `json:"difficulty_breakdown"`
	Grade           string             `json:"grade"`
	Recommendation  string             `json:"recommendation"`
}

// RoundScore represents score for a round.
type RoundScore struct {
	RoundID    string  `json:"round_id"`
	RoundName  string  `json:"round_name"`
	Score      float64 `json:"score"`
	MaxScore   float64 `json:"max_score"`
	Percentage float64 `json:"percentage"`
	Passed     bool    `json:"passed"`
}

// SkillScoreResult represents aggregated skill score.
type SkillScoreResult struct {
	Skill         string  `json:"skill"`
	Score         float64 `json:"score"`
	MaxScore      float64 `json:"max_score"`
	Percentage    float64 `json:"percentage"`
	QuestionCount int     `json:"question_count"`
}

// NewScoreCalculatorTool creates a new score calculator tool.
func NewScoreCalculatorTool(cfg *ScoreConfig) *ScoreCalculatorTool {
	if cfg == nil {
		cfg = &ScoreConfig{
			PassThreshold: 60.0,
			WeightByDifficulty: map[string]float64{
				"basic":        1.0,
				"intermediate": 1.5,
				"advanced":     2.0,
				"expert":       2.5,
			},
			WeightByRound: map[string]float64{
				"screening":     0.15,
				"technical":     0.40,
				"behavioral":    0.20,
				"system_design": 0.25,
			},
		}
	}
	return &ScoreCalculatorTool{config: cfg}
}

// Info returns the tool information.
func (t *ScoreCalculatorTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "score_calculator",
		Description: "计算面试分数，支持按轮次、难度、技能维度计算",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"answers": map[string]any{
					"type":        "array",
					"description": "答题得分列表",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"question_id": map[string]any{"type": "string"},
							"round_id":    map[string]any{"type": "string"},
							"score":       map[string]any{"type": "number"},
							"max_score":   map[string]any{"type": "number"},
							"difficulty":  map[string]any{"type": "string"},
							"weight":      map[string]any{"type": "number"},
							"skill_scores": map[string]any{
								"type": "object",
								"additionalProperties": map[string]any{"type": "number"},
							},
						},
					},
				},
				"rounds": map[string]any{
					"type":        "array",
					"description": "轮次信息",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":     map[string]any{"type": "string"},
							"name":   map[string]any{"type": "string"},
							"weight": map[string]any{"type": "number"},
						},
					},
				},
			},
			"required": []string{"answers"},
		},
	}, nil
}

// InvokableRun executes the tool.
func (t *ScoreCalculatorTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params ScoreInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	output := t.calculate(params)
	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
}

func (t *ScoreCalculatorTool) calculate(input ScoreInput) *ScoreOutput {
	output := &ScoreOutput{
		RoundScores:         make(map[string]RoundScore),
		SkillScores:         make(map[string]SkillScoreResult),
		DifficultyBreakdown: make(map[string]float64),
	}

	// Build round info map
	roundMap := make(map[string]RoundInfo)
	for _, round := range input.Rounds {
		roundMap[round.ID] = round
	}

	// Calculate scores by round
	roundScoreSum := make(map[string]float64)
	roundMaxSum := make(map[string]float64)
	roundCount := make(map[string]int)

	// Calculate scores by difficulty
	difficultyScoreSum := make(map[string]float64)
	difficultyMaxSum := make(map[string]float64)

	// Calculate scores by skill
	skillScoreSum := make(map[string]float64)
	skillMaxSum := make(map[string]float64)
	skillCount := make(map[string]int)

	for _, answer := range input.Answers {
		// Apply difficulty weight
		weight := t.config.WeightByDifficulty[answer.Difficulty]
		if weight == 0 {
			weight = 1.0
		}
		if answer.Weight > 0 {
			weight = answer.Weight
		}

		weightedScore := answer.Score * weight
		weightedMax := answer.MaxScore * weight

		// Total
		output.TotalScore += weightedScore
		output.TotalMaxScore += weightedMax

		// By round
		roundScoreSum[answer.RoundID] += weightedScore
		roundMaxSum[answer.RoundID] += weightedMax
		roundCount[answer.RoundID]++

		// By difficulty
		difficultyScoreSum[answer.Difficulty] += answer.Score
		difficultyMaxSum[answer.Difficulty] += answer.MaxScore

		// By skill
		for skill, score := range answer.SkillScores {
			skillScoreSum[skill] += score
			skillMaxSum[skill] += 1.0 // Assume max 1.0 per skill per question
			skillCount[skill]++
		}
	}

	// Calculate percentages
	if output.TotalMaxScore > 0 {
		output.Percentage = (output.TotalScore / output.TotalMaxScore) * 100
	}
	output.Passed = output.Percentage >= t.config.PassThreshold

	// Round scores
	for roundID, score := range roundScoreSum {
		maxScore := roundMaxSum[roundID]
		percentage := 0.0
		if maxScore > 0 {
			percentage = (score / maxScore) * 100
		}
		
		roundInfo := roundMap[roundID]
		output.RoundScores[roundID] = RoundScore{
			RoundID:    roundID,
			RoundName:  roundInfo.Name,
			Score:      score,
			MaxScore:   maxScore,
			Percentage: percentage,
			Passed:     percentage >= t.config.PassThreshold,
		}
	}

	// Skill scores
	for skill, score := range skillScoreSum {
		maxScore := skillMaxSum[skill]
		percentage := 0.0
		if maxScore > 0 {
			percentage = (score / maxScore) * 100
		}
		output.SkillScores[skill] = SkillScoreResult{
			Skill:         skill,
			Score:         score,
			MaxScore:      maxScore,
			Percentage:    percentage,
			QuestionCount: skillCount[skill],
		}
	}

	// Difficulty breakdown
	for diff, score := range difficultyScoreSum {
		maxScore := difficultyMaxSum[diff]
		if maxScore > 0 {
			output.DifficultyBreakdown[diff] = (score / maxScore) * 100
		}
	}

	// Determine grade and recommendation
	output.Grade = t.determineGrade(output.Percentage)
	output.Recommendation = t.determineRecommendation(output.Percentage, output.RoundScores)

	return output
}

func (t *ScoreCalculatorTool) determineGrade(percentage float64) string {
	switch {
	case percentage >= 90:
		return "A+"
	case percentage >= 85:
		return "A"
	case percentage >= 80:
		return "A-"
	case percentage >= 75:
		return "B+"
	case percentage >= 70:
		return "B"
	case percentage >= 65:
		return "B-"
	case percentage >= 60:
		return "C+"
	case percentage >= 55:
		return "C"
	case percentage >= 50:
		return "C-"
	default:
		return "D"
	}
}

func (t *ScoreCalculatorTool) determineRecommendation(percentage float64, roundScores map[string]RoundScore) string {
	// Check if all critical rounds passed
	criticalRoundsPassed := true
	for _, round := range roundScores {
		if round.RoundName == "technical" || round.RoundName == "system_design" {
			if !round.Passed {
				criticalRoundsPassed = false
				break
			}
		}
	}

	switch {
	case percentage >= 85 && criticalRoundsPassed:
		return "strong_hire"
	case percentage >= 70 && criticalRoundsPassed:
		return "hire"
	case percentage >= 55:
		return "maybe"
	default:
		return "no_hire"
	}
}

// CalculateWeightedAverage calculates weighted average of scores.
func CalculateWeightedAverage(scores []float64, weights []float64) float64 {
	if len(scores) == 0 || len(scores) != len(weights) {
		return 0
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for i := range scores {
		weightedSum += scores[i] * weights[i]
		totalWeight += weights[i]
	}

	if totalWeight == 0 {
		return 0
	}

	return weightedSum / totalWeight
}

// NormalizeScore normalizes a score to 0-100 range.
func NormalizeScore(score, minScore, maxScore float64) float64 {
	if maxScore == minScore {
		return 50
	}
	normalized := (score - minScore) / (maxScore - minScore) * 100
	return math.Max(0, math.Min(100, normalized))
}
