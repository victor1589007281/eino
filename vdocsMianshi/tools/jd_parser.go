// Package tools provides interview-related tools for the agent.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// JDParserTool parses job descriptions.
type JDParserTool struct {
	skillPatterns    map[string]*regexp.Regexp
	levelPatterns    map[string]*regexp.Regexp
	experiencePattern *regexp.Regexp
}

// JDParseInput represents the input for JD parsing.
type JDParseInput struct {
	Content string `json:"content"`
}

// JDParseOutput represents the output of JD parsing.
type JDParseOutput struct {
	Title            string   `json:"title"`
	Level            string   `json:"level"`
	RequiredSkills   []string `json:"required_skills"`
	PreferredSkills  []string `json:"preferred_skills"`
	Responsibilities []string `json:"responsibilities"`
	Requirements     []string `json:"requirements"`
	YearsExperience  struct {
		Min int `json:"min"`
		Max int `json:"max"`
	} `json:"years_experience"`
	Keywords []string `json:"keywords"`
}

// NewJDParserTool creates a new JD parser tool.
func NewJDParserTool() *JDParserTool {
	return &JDParserTool{
		skillPatterns: map[string]*regexp.Regexp{
			"programming": regexp.MustCompile(`(?i)(java|python|go|golang|javascript|typescript|c\+\+|rust|scala|kotlin)`),
			"framework":   regexp.MustCompile(`(?i)(spring|django|flask|react|vue|angular|gin|echo|fiber)`),
			"database":    regexp.MustCompile(`(?i)(mysql|postgresql|mongodb|redis|elasticsearch|kafka|cassandra)`),
			"cloud":       regexp.MustCompile(`(?i)(aws|azure|gcp|docker|kubernetes|k8s|terraform)`),
			"ai_ml":       regexp.MustCompile(`(?i)(机器学习|深度学习|nlp|tensorflow|pytorch|大模型|llm)`),
		},
		levelPatterns: map[string]*regexp.Regexp{
			"junior":    regexp.MustCompile(`(?i)(初级|junior|入门|应届)`),
			"mid":       regexp.MustCompile(`(?i)(中级|mid|中高级)`),
			"senior":    regexp.MustCompile(`(?i)(高级|senior|资深)`),
			"lead":      regexp.MustCompile(`(?i)(lead|负责人|tech lead|技术负责人)`),
			"principal": regexp.MustCompile(`(?i)(principal|首席|架构师|expert)`),
		},
		experiencePattern: regexp.MustCompile(`(\d+)[-~到]?(\d*)年`),
	}
}

// Info returns the tool information.
func (t *JDParserTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "jd_parser",
		Description: "解析职位描述(JD)，提取关键信息如技能要求、经验要求等",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "职位描述内容",
				},
			},
			"required": []string{"content"},
		},
	}, nil
}

// InvokableRun executes the tool.
func (t *JDParserTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params JDParseInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		// If JSON parsing fails, treat input as raw content
		params.Content = input
	}

	if params.Content == "" {
		return "", fmt.Errorf("empty JD content")
	}

	output := t.parse(params.Content)
	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
}

func (t *JDParserTool) parse(content string) *JDParseOutput {
	output := &JDParseOutput{}

	// Extract title (first line or before first newline)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			output.Title = line
			break
		}
	}

	// Extract level
	for level, pattern := range t.levelPatterns {
		if pattern.MatchString(content) {
			output.Level = level
			break
		}
	}
	if output.Level == "" {
		output.Level = "mid" // default
	}

	// Extract skills
	skillsFound := make(map[string]bool)
	for category, pattern := range t.skillPatterns {
		matches := pattern.FindAllString(content, -1)
		for _, match := range matches {
			skill := strings.ToLower(match)
			if !skillsFound[skill] {
				skillsFound[skill] = true
				// Categorize as required or preferred based on context
				if t.isRequiredSkill(content, match) {
					output.RequiredSkills = append(output.RequiredSkills, match)
				} else {
					output.PreferredSkills = append(output.PreferredSkills, match)
				}
			}
			_ = category // Used for potential category tracking
		}
	}

	// Extract years of experience
	if matches := t.experiencePattern.FindStringSubmatch(content); len(matches) > 1 {
		var min, max int
		fmt.Sscanf(matches[1], "%d", &min)
		if len(matches) > 2 && matches[2] != "" {
			fmt.Sscanf(matches[2], "%d", &max)
		} else {
			max = min + 2
		}
		output.YearsExperience.Min = min
		output.YearsExperience.Max = max
	}

	// Extract responsibilities and requirements
	output.Responsibilities = t.extractSection(content, []string{"职责", "负责", "工作内容", "responsibilities"})
	output.Requirements = t.extractSection(content, []string{"要求", "任职资格", "qualification", "requirements"})

	// Extract keywords
	output.Keywords = t.extractKeywords(content)

	return output
}

func (t *JDParserTool) isRequiredSkill(content string, skill string) bool {
	// Check if skill appears near required/must-have keywords
	requiredPattern := regexp.MustCompile(fmt.Sprintf(`(?i)(必须|必备|required|must)[^。]*%s|%s[^。]*(必须|必备|required|must)`, skill, skill))
	return requiredPattern.MatchString(content)
}

func (t *JDParserTool) extractSection(content string, markers []string) []string {
	var results []string
	contentLower := strings.ToLower(content)

	for _, marker := range markers {
		idx := strings.Index(contentLower, strings.ToLower(marker))
		if idx != -1 {
			// Extract text after marker until next section or end
			section := content[idx:]
			lines := strings.Split(section, "\n")
			for i, line := range lines {
				if i == 0 {
					continue // Skip the marker line
				}
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				// Check if this is a new section
				isNewSection := false
				for _, m := range markers {
					if strings.Contains(strings.ToLower(line), strings.ToLower(m)) {
						isNewSection = true
						break
					}
				}
				if isNewSection {
					break
				}
				// Clean up bullet points
				line = strings.TrimPrefix(line, "-")
				line = strings.TrimPrefix(line, "•")
				line = strings.TrimPrefix(line, "*")
				line = regexp.MustCompile(`^\d+[\.\、]`).ReplaceAllString(line, "")
				line = strings.TrimSpace(line)
				if line != "" {
					results = append(results, line)
				}
			}
			break
		}
	}

	return results
}

func (t *JDParserTool) extractKeywords(content string) []string {
	// Extract common tech keywords
	keywordPatterns := []string{
		`(?i)分布式`,
		`(?i)微服务`,
		`(?i)高并发`,
		`(?i)大数据`,
		`(?i)云原生`,
		`(?i)DevOps`,
		`(?i)CI/CD`,
		`(?i)敏捷`,
		`(?i)Scrum`,
		`(?i)API`,
		`(?i)RESTful`,
		`(?i)GraphQL`,
		`(?i)消息队列`,
		`(?i)缓存`,
	}

	var keywords []string
	seen := make(map[string]bool)

	for _, pattern := range keywordPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(content, -1)
		for _, match := range matches {
			matchLower := strings.ToLower(match)
			if !seen[matchLower] {
				seen[matchLower] = true
				keywords = append(keywords, match)
			}
		}
	}

	return keywords
}
