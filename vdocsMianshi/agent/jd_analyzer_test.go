package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "simple json",
			input: `Here is the result: {"name": "test"}`,
			expected: `{"name": "test"}`,
		},
		{
			name: "nested json",
			input: `Result: {"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name: "no json",
			input: "No json here",
			expected: "",
		},
		{
			name: "json with array",
			input: `Data: {"items": [1, 2, 3]}`,
			expected: `{"items": [1, 2, 3]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSkill_Validation(t *testing.T) {
	skill := Skill{
		Name:          "Go",
		Category:      "programming",
		Priority:      1,
		ExpectedLevel: "advanced",
		Weight:        0.3,
	}

	assert.Equal(t, "Go", skill.Name)
	assert.Equal(t, "programming", skill.Category)
	assert.Equal(t, 1, skill.Priority)
	assert.Equal(t, "advanced", skill.ExpectedLevel)
	assert.Equal(t, 0.3, skill.Weight)
}

func TestJobDescription_Fields(t *testing.T) {
	jd := &JobDescription{
		ID:    "test-id",
		Title: "高级Go开发工程师",
		Level: "senior",
		RequiredSkills: []Skill{
			{Name: "Go", Priority: 1, Weight: 0.5},
			{Name: "MySQL", Priority: 2, Weight: 0.3},
		},
		YearsExperience: ExperienceRange{Min: 3, Max: 5},
	}

	assert.Equal(t, "test-id", jd.ID)
	assert.Equal(t, "高级Go开发工程师", jd.Title)
	assert.Equal(t, "senior", jd.Level)
	assert.Len(t, jd.RequiredSkills, 2)
	assert.Equal(t, 3, jd.YearsExperience.Min)
	assert.Equal(t, 5, jd.YearsExperience.Max)
}

func TestHiringIntent_Analysis(t *testing.T) {
	intent := &HiringIntent{
		PrimaryFocus:    "technical",
		TeamRole:        "individual_contributor",
		GrowthPotential: "high",
		Urgency:         "normal",
		KeyCompetencies: []string{"分布式系统", "高并发"},
		CultureFit:      []string{"团队协作", "持续学习"},
	}

	assert.Equal(t, "technical", intent.PrimaryFocus)
	assert.Equal(t, "individual_contributor", intent.TeamRole)
	assert.Equal(t, "high", intent.GrowthPotential)
	assert.Contains(t, intent.KeyCompetencies, "分布式系统")
}

func TestInterviewPhase_String(t *testing.T) {
	tests := []struct {
		phase    InterviewPhase
		expected string
	}{
		{PhaseJDAnalysis, "jd_analysis"},
		{PhaseQuestionDesign, "question_design"},
		{PhaseInterview, "interview"},
		{PhaseEvaluation, "evaluation"},
		{PhaseReport, "report"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.phase.String())
		})
	}
}

func TestQuestionDifficulty_String(t *testing.T) {
	tests := []struct {
		difficulty QuestionDifficulty
		expected   string
	}{
		{DifficultyBasic, "basic"},
		{DifficultyIntermediate, "intermediate"},
		{DifficultyAdvanced, "advanced"},
		{DifficultyExpert, "expert"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.difficulty.String())
		})
	}
}

func TestQuestionType_String(t *testing.T) {
	tests := []struct {
		qtype    QuestionType
		expected string
	}{
		{QuestionTypeTechnical, "technical"},
		{QuestionTypeBehavioral, "behavioral"},
		{QuestionTypeScenario, "scenario"},
		{QuestionTypeCoding, "coding"},
		{QuestionTypeSystemDesign, "system_design"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.qtype.String())
		})
	}
}

func TestNormalizeSkillWeights(t *testing.T) {
	jd := &JobDescription{
		RequiredSkills: []Skill{
			{Name: "Go", Priority: 1, Weight: 0},
			{Name: "MySQL", Priority: 2, Weight: 0},
			{Name: "Redis", Priority: 3, Weight: 0},
		},
	}

	analyzer := &JDAnalyzerAgent{
		config: &JDAnalyzerConfig{},
	}

	analyzer.normalizeSkillWeights(jd)

	// Check that weights are normalized
	totalWeight := 0.0
	for _, skill := range jd.RequiredSkills {
		totalWeight += skill.Weight
		assert.Greater(t, skill.Weight, 0.0)
	}

	// Total should be approximately 1.0
	assert.InDelta(t, 1.0, totalWeight, 0.001)
}
