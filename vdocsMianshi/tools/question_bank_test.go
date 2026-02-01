package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQuestionBankTool(t *testing.T) {
	tool := NewQuestionBankTool()
	assert.NotNil(t, tool)
	assert.NotEmpty(t, tool.questions)
}

func TestQuestionBankTool_Info(t *testing.T) {
	tool := NewQuestionBankTool()
	info, err := tool.Info(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "question_bank", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.NotNil(t, info.InputSchema)
}

func TestQuestionBankTool_SelectBySkill(t *testing.T) {
	tool := NewQuestionBankTool()

	input := QuestionBankInput{
		Skills: []string{"Go", "golang"},
		Count:  3,
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var questions []QuestionTemplate
	err = json.Unmarshal([]byte(result), &questions)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(questions), 3)
}

func TestQuestionBankTool_SelectByDifficulty(t *testing.T) {
	tool := NewQuestionBankTool()

	input := QuestionBankInput{
		Difficulty: "intermediate",
		Count:      5,
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var questions []QuestionTemplate
	err = json.Unmarshal([]byte(result), &questions)
	require.NoError(t, err)

	// All questions should be intermediate difficulty
	for _, q := range questions {
		assert.Equal(t, "intermediate", q.Difficulty)
	}
}

func TestQuestionBankTool_SelectByType(t *testing.T) {
	tool := NewQuestionBankTool()

	input := QuestionBankInput{
		Type:  "technical",
		Count: 5,
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var questions []QuestionTemplate
	err = json.Unmarshal([]byte(result), &questions)
	require.NoError(t, err)

	// All questions should be technical type
	for _, q := range questions {
		assert.Equal(t, "technical", q.Type)
	}
}

func TestQuestionBankTool_SelectByCategory(t *testing.T) {
	tool := NewQuestionBankTool()

	input := QuestionBankInput{
		Category: "system_design",
		Count:    5,
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var questions []QuestionTemplate
	err = json.Unmarshal([]byte(result), &questions)
	require.NoError(t, err)

	// All questions should be from system_design category
	for _, q := range questions {
		assert.Equal(t, "system_design", q.Category)
	}
}

func TestQuestionBankTool_AddQuestion(t *testing.T) {
	tool := NewQuestionBankTool()

	newQuestion := QuestionTemplate{
		ID:         "custom_001",
		Content:    "这是一个自定义问题",
		Type:       "technical",
		Difficulty: "advanced",
		Category:   "custom",
		Skills:     []string{"custom_skill"},
	}

	tool.AddQuestion(newQuestion)

	// Verify the question was added
	input := QuestionBankInput{
		Category: "custom",
		Count:    1,
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var questions []QuestionTemplate
	err = json.Unmarshal([]byte(result), &questions)
	require.NoError(t, err)
	assert.Len(t, questions, 1)
	assert.Equal(t, "custom_001", questions[0].ID)
}

func TestQuestionBankTool_InvalidInput(t *testing.T) {
	tool := NewQuestionBankTool()

	_, err := tool.InvokableRun(context.Background(), "invalid json")
	assert.Error(t, err)
}

func TestQuestionBankTool_EmptyResult(t *testing.T) {
	tool := NewQuestionBankTool()

	// Search for non-existent category
	input := QuestionBankInput{
		Category: "nonexistent_category",
		Count:    5,
	}

	inputJSON, _ := json.Marshal(input)
	result, err := tool.InvokableRun(context.Background(), string(inputJSON))

	require.NoError(t, err)

	var questions []QuestionTemplate
	err = json.Unmarshal([]byte(result), &questions)
	require.NoError(t, err)
	assert.Empty(t, questions)
}
