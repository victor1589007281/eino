package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// SkillMapperTool maps and categorizes skills.
type SkillMapperTool struct {
	skillGraph     map[string]*SkillNode
	categoryMap    map[string]string
	synonymMap     map[string]string
	hierarchyMap   map[string][]string // parent -> children
}

// SkillNode represents a skill in the skill graph.
type SkillNode struct {
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Level      int      `json:"level"` // 1=fundamental, 2=intermediate, 3=advanced
	Related    []string `json:"related"`
	Prerequisites []string `json:"prerequisites"`
	Keywords   []string `json:"keywords"`
}

// SkillMapInput represents the input for skill mapping.
type SkillMapInput struct {
	Skills      []string `json:"skills"`
	Context     string   `json:"context,omitempty"`
	MapToLevel  bool     `json:"map_to_level,omitempty"`
	IncludeRelated bool  `json:"include_related,omitempty"`
}

// SkillMapOutput represents the mapped skills.
type SkillMapOutput struct {
	MappedSkills   []MappedSkill         `json:"mapped_skills"`
	Categories     map[string][]string   `json:"categories"`
	RelatedSkills  []string              `json:"related_skills,omitempty"`
	SkillTree      map[string][]string   `json:"skill_tree,omitempty"`
}

// MappedSkill represents a mapped skill.
type MappedSkill struct {
	Original      string   `json:"original"`
	Normalized    string   `json:"normalized"`
	Category      string   `json:"category"`
	Level         int      `json:"level"`
	Prerequisites []string `json:"prerequisites,omitempty"`
	Related       []string `json:"related,omitempty"`
}

// NewSkillMapperTool creates a new skill mapper tool.
func NewSkillMapperTool() *SkillMapperTool {
	t := &SkillMapperTool{
		skillGraph:   make(map[string]*SkillNode),
		categoryMap:  make(map[string]string),
		synonymMap:   make(map[string]string),
		hierarchyMap: make(map[string][]string),
	}
	t.loadDefaultSkills()
	return t
}

// Info returns the tool information.
func (t *SkillMapperTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "skill_mapper",
		Description: "映射和分类技能，识别技能之间的关系和层次",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skills": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "需要映射的技能列表",
				},
				"context": map[string]any{
					"type":        "string",
					"description": "上下文信息，帮助更准确地映射",
				},
				"map_to_level": map[string]any{
					"type":        "boolean",
					"description": "是否映射技能等级",
				},
				"include_related": map[string]any{
					"type":        "boolean",
					"description": "是否包含相关技能",
				},
			},
			"required": []string{"skills"},
		},
	}, nil
}

// InvokableRun executes the tool.
func (t *SkillMapperTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params SkillMapInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	output := t.mapSkills(params)
	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
}

func (t *SkillMapperTool) mapSkills(input SkillMapInput) *SkillMapOutput {
	output := &SkillMapOutput{
		MappedSkills: make([]MappedSkill, 0),
		Categories:   make(map[string][]string),
	}

	relatedSet := make(map[string]bool)

	for _, skill := range input.Skills {
		mapped := t.mapSingleSkill(skill)
		output.MappedSkills = append(output.MappedSkills, mapped)

		// Add to category
		output.Categories[mapped.Category] = append(output.Categories[mapped.Category], mapped.Normalized)

		// Collect related skills
		if input.IncludeRelated {
			for _, related := range mapped.Related {
				if !relatedSet[related] {
					relatedSet[related] = true
					output.RelatedSkills = append(output.RelatedSkills, related)
				}
			}
		}
	}

	// Build skill tree if requested
	if input.MapToLevel {
		output.SkillTree = t.buildSkillTree(output.MappedSkills)
	}

	return output
}

func (t *SkillMapperTool) mapSingleSkill(skill string) MappedSkill {
	normalizedSkill := strings.ToLower(strings.TrimSpace(skill))

	// Check for synonyms
	if canonical, ok := t.synonymMap[normalizedSkill]; ok {
		normalizedSkill = canonical
	}

	// Look up in skill graph
	if node, ok := t.skillGraph[normalizedSkill]; ok {
		return MappedSkill{
			Original:      skill,
			Normalized:    node.Name,
			Category:      node.Category,
			Level:         node.Level,
			Prerequisites: node.Prerequisites,
			Related:       node.Related,
		}
	}

	// Try to infer category
	category := t.inferCategory(normalizedSkill)

	return MappedSkill{
		Original:   skill,
		Normalized: skill,
		Category:   category,
		Level:      2, // Default to intermediate
	}
}

func (t *SkillMapperTool) inferCategory(skill string) string {
	// Check keywords
	skillLower := strings.ToLower(skill)

	categoryPatterns := map[string][]string{
		"programming": {"java", "python", "go", "javascript", "typescript", "c++", "rust", "scala", "kotlin", "swift"},
		"framework":   {"spring", "django", "flask", "react", "vue", "angular", "gin", "express", "rails"},
		"database":    {"mysql", "postgresql", "mongodb", "redis", "elasticsearch", "cassandra", "oracle", "sql"},
		"cloud":       {"aws", "azure", "gcp", "docker", "kubernetes", "terraform", "cloud"},
		"devops":      {"ci/cd", "jenkins", "gitlab", "github actions", "ansible", "prometheus", "grafana"},
		"ai_ml":       {"机器学习", "深度学习", "nlp", "tensorflow", "pytorch", "scikit", "大模型", "llm"},
		"soft_skills": {"沟通", "领导", "团队", "管理", "协调", "规划"},
	}

	for category, patterns := range categoryPatterns {
		for _, pattern := range patterns {
			if strings.Contains(skillLower, pattern) {
				return category
			}
		}
	}

	return "other"
}

func (t *SkillMapperTool) buildSkillTree(skills []MappedSkill) map[string][]string {
	tree := make(map[string][]string)

	// Group by level
	levelNames := map[int]string{
		1: "fundamental",
		2: "intermediate",
		3: "advanced",
	}

	for _, skill := range skills {
		levelName := levelNames[skill.Level]
		if levelName == "" {
			levelName = "other"
		}
		tree[levelName] = append(tree[levelName], skill.Normalized)
	}

	return tree
}

func (t *SkillMapperTool) loadDefaultSkills() {
	// Programming languages
	t.skillGraph["java"] = &SkillNode{
		Name:     "Java",
		Category: "programming",
		Level:    2,
		Related:  []string{"Spring", "Maven", "JVM"},
		Keywords: []string{"java", "jdk", "jvm"},
	}
	t.skillGraph["python"] = &SkillNode{
		Name:     "Python",
		Category: "programming",
		Level:    1,
		Related:  []string{"Django", "Flask", "FastAPI"},
		Keywords: []string{"python", "pip"},
	}
	t.skillGraph["go"] = &SkillNode{
		Name:     "Go",
		Category: "programming",
		Level:    2,
		Related:  []string{"Gin", "Echo", "goroutine"},
		Keywords: []string{"go", "golang"},
	}
	t.synonymMap["golang"] = "go"

	// Frameworks
	t.skillGraph["spring"] = &SkillNode{
		Name:          "Spring",
		Category:      "framework",
		Level:         2,
		Prerequisites: []string{"Java"},
		Related:       []string{"Spring Boot", "Spring Cloud"},
		Keywords:      []string{"spring", "springboot"},
	}
	t.skillGraph["react"] = &SkillNode{
		Name:          "React",
		Category:      "framework",
		Level:         2,
		Prerequisites: []string{"JavaScript"},
		Related:       []string{"Redux", "Next.js"},
		Keywords:      []string{"react", "reactjs"},
	}

	// Databases
	t.skillGraph["mysql"] = &SkillNode{
		Name:     "MySQL",
		Category: "database",
		Level:    2,
		Related:  []string{"SQL", "索引优化", "事务"},
		Keywords: []string{"mysql", "sql"},
	}
	t.skillGraph["redis"] = &SkillNode{
		Name:     "Redis",
		Category: "database",
		Level:    2,
		Related:  []string{"缓存", "分布式锁"},
		Keywords: []string{"redis"},
	}

	// Cloud & DevOps
	t.skillGraph["kubernetes"] = &SkillNode{
		Name:          "Kubernetes",
		Category:      "cloud",
		Level:         3,
		Prerequisites: []string{"Docker"},
		Related:       []string{"Helm", "Istio"},
		Keywords:      []string{"kubernetes", "k8s"},
	}
	t.synonymMap["k8s"] = "kubernetes"

	// System design concepts
	t.skillGraph["分布式系统"] = &SkillNode{
		Name:          "分布式系统",
		Category:      "system_design",
		Level:         3,
		Prerequisites: []string{"网络基础", "操作系统"},
		Related:       []string{"微服务", "CAP理论", "分布式事务"},
		Keywords:      []string{"分布式", "distributed"},
	}

	// Build hierarchy
	t.hierarchyMap["programming"] = []string{"Java", "Python", "Go", "JavaScript"}
	t.hierarchyMap["framework"] = []string{"Spring", "React", "Vue", "Django"}
	t.hierarchyMap["database"] = []string{"MySQL", "PostgreSQL", "MongoDB", "Redis"}
}

// GetRelatedSkills returns skills related to the given skill.
func (t *SkillMapperTool) GetRelatedSkills(skill string) []string {
	normalizedSkill := strings.ToLower(skill)
	if canonical, ok := t.synonymMap[normalizedSkill]; ok {
		normalizedSkill = canonical
	}

	if node, ok := t.skillGraph[normalizedSkill]; ok {
		return node.Related
	}
	return nil
}

// GetPrerequisites returns prerequisites for the given skill.
func (t *SkillMapperTool) GetPrerequisites(skill string) []string {
	normalizedSkill := strings.ToLower(skill)
	if canonical, ok := t.synonymMap[normalizedSkill]; ok {
		normalizedSkill = canonical
	}

	if node, ok := t.skillGraph[normalizedSkill]; ok {
		return node.Prerequisites
	}
	return nil
}
