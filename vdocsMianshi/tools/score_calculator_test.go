package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScoreCalculatorTool(t *testing.T) {
	tool := NewScoreCalculatorTool(nil)
	assert.NotNil(t, tool)
	assert.NotNil(t, tool.config)
}

func TestScoreCalculatorTool_Info(t *testing.T) {
	tool := NewScoreCalculatorTool(nil)
	info, err := tool.Info(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "score_calculator", info.Name)
	assert.NotEmpty(t, info.Description)
}

func TestScoreCalculatorTool_BasicCalculation(t *testing.T) {
	tool := NewScoreCalculatorTool(nil)

	input := ScoreInput{
		Answers: []AnswerScore{
			{QuestionID: "q1", Score: 8, MaxScore: 10, Difficulty: "basic"},
			{QuestionID: "q2", Score: 7, MaxScore: 10, Difficulty: "intermediate"},
			{QuestionID: "q3", Score: 6, MaxScore: 10, Difficulty: "advanced"},
		},
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var output ScoreOutput
	err = json.Unmarshal([]byte(result), &output)
	require.NoError(t, err)

	assert.Greater(t, output.TotalScore, 0.0)
	assert.Greater(t, output.TotalMaxScore, 0.0)
	assert.Greater(t, output.Percentage, 0.0)
	assert.NotEmpty(t, output.Grade)
	assert.NotEmpty(t, output.Recommendation)
}

func TestScoreCalculatorTool_WithRounds(t *testing.T) {
	tool := NewScoreCalculatorTool(nil)

	input := ScoreInput{
		Answers: []AnswerScore{
			{QuestionID: "q1", RoundID: "r1", Score: 8, MaxScore: 10, Difficulty: "basic"},
			{QuestionID: "q2", RoundID: "r1", Score: 9, MaxScore: 10, Difficulty: "basic"},
			{QuestionID: "q3", RoundID: "r2", Score: 7, MaxScore: 10, Difficulty: "intermediate"},
		},
		Rounds: []RoundInfo{
			{ID: "r1", Name: "technical", Weight: 0.6},
			{ID: "r2", Name: "behavioral", Weight: 0.4},
		},
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var output ScoreOutput
	err = json.Unmarshal([]byte(result), &output)
	require.NoError(t, err)

	assert.NotEmpty(t, output.RoundScores)
	assert.Contains(t, output.RoundScores, "r1")
	assert.Contains(t, output.RoundScores, "r2")
}

func TestScoreCalculatorTool_WithSkillScores(t *testing.T) {
	tool := NewScoreCalculatorTool(nil)

	input := ScoreInput{
		Answers: []AnswerScore{
			{
				QuestionID: "q1",
				Score:      8,
				MaxScore:   10,
				Difficulty: "intermediate",
				SkillScores: map[string]float64{
					"Go":    0.9,
					"MySQL": 0.7,
				},
			},
			{
				QuestionID: "q2",
				Score:      7,
				MaxScore:   10,
				Difficulty: "intermediate",
				SkillScores: map[string]float64{
					"Go":    0.8,
					"Redis": 0.6,
				},
			},
		},
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var output ScoreOutput
	err = json.Unmarshal([]byte(result), &output)
	require.NoError(t, err)

	assert.NotEmpty(t, output.SkillScores)
	assert.Contains(t, output.SkillScores, "Go")
	assert.Equal(t, 2, output.SkillScores["Go"].QuestionCount)
}

func TestScoreCalculatorTool_DetermineGrade(t *testing.T) {
	tool := NewScoreCalculatorTool(nil)

	tests := []struct {
		percentage float64
		expected   string
	}{
		{95, "A+"},
		{87, "A"},
		{82, "A-"},
		{77, "B+"},
		{72, "B"},
		{67, "B-"},
		{62, "C+"},
		{57, "C"},
		{52, "C-"},
		{45, "D"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			grade := tool.determineGrade(tt.percentage)
			assert.Equal(t, tt.expected, grade)
		})
	}
}

func TestScoreCalculatorTool_DetermineRecommendation(t *testing.T) {
	tool := NewScoreCalculatorTool(nil)

	// High score with all rounds passed
	roundScores := map[string]RoundScore{
		"technical": {Passed: true},
		"system_design": {Passed: true},
	}
	rec := tool.determineRecommendation(90, roundScores)
	assert.Equal(t, "strong_hire", rec)

	// Good score with all rounds passed
	rec = tool.determineRecommendation(75, roundScores)
	assert.Equal(t, "hire", rec)

	// Moderate score
	rec = tool.determineRecommendation(60, roundScores)
	assert.Equal(t, "maybe", rec)

	// Low score
	rec = tool.determineRecommendation(40, roundScores)
	assert.Equal(t, "no_hire", rec)
}

func TestCalculateWeightedAverage(t *testing.T) {
	tests := []struct {
		name     string
		scores   []float64
		weights  []float64
		expected float64
	}{
		{
			name:     "equal weights",
			scores:   []float64{80, 90, 70},
			weights:  []float64{1, 1, 1},
			expected: 80,
		},
		{
			name:     "different weights",
			scores:   []float64{80, 90},
			weights:  []float64{0.3, 0.7},
			expected: 87,
		},
		{
			name:     "empty",
			scores:   []float64{},
			weights:  []float64{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateWeightedAverage(tt.scores, tt.weights)
			assert.InDelta(t, tt.expected, result, 0.1)
		})
	}
}

func TestNormalizeScore(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		min      float64
		max      float64
		expected float64
	}{
		{"middle", 50, 0, 100, 50},
		{"low", 0, 0, 100, 0},
		{"high", 100, 0, 100, 100},
		{"above max", 120, 0, 100, 100},
		{"below min", -10, 0, 100, 0},
		{"custom range", 75, 50, 100, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeScore(tt.score, tt.min, tt.max)
			assert.InDelta(t, tt.expected, result, 0.1)
		})
	}
}
