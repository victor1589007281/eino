// Package capsule 实体提取器
package capsule

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// EntityExtractor 实体提取器
type EntityExtractor struct {
	patterns map[string]*regexp.Regexp
}

// NewEntityExtractor 创建实体提取器
func NewEntityExtractor() *EntityExtractor {
	e := &EntityExtractor{
		patterns: make(map[string]*regexp.Regexp),
	}

	// 编译正则模式
	e.patterns["url"] = regexp.MustCompile(`https?://[^\s]+`)
	e.patterns["email"] = regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`)
	e.patterns["code_lang"] = regexp.MustCompile("```(\\w+)")
	e.patterns["file_path"] = regexp.MustCompile(`[/\\][\w./\\-]+\.\w+`)
	e.patterns["version"] = regexp.MustCompile(`v?\d+\.\d+(\.\d+)?`)
	e.patterns["command"] = regexp.MustCompile("`[^`]+`")

	return e
}

// Extract 提取实体和关系
func (e *EntityExtractor) Extract(ctx context.Context, messages []*storage.Message) ([]*storage.Entity, []*storage.Relation) {
	var entities []*storage.Entity
	var relations []*storage.Relation

	entityMap := make(map[string]*storage.Entity)

	for _, msg := range messages {
		// 提取实体
		msgEntities := e.extractEntities(msg.Content)
		for _, entity := range msgEntities {
			key := entity.Type + ":" + entity.Name
			if _, exists := entityMap[key]; !exists {
				entityMap[key] = entity
				entities = append(entities, entity)
			}
		}

		// 从消息元数据提取
		if msg.Metadata != nil {
			metaEntities := e.extractFromMetadata(msg.Metadata)
			for _, entity := range metaEntities {
				key := entity.Type + ":" + entity.Name
				if _, exists := entityMap[key]; !exists {
					entityMap[key] = entity
					entities = append(entities, entity)
				}
			}
		}
	}

	// 推断实体间的关系
	relations = e.inferRelations(entities, messages)

	return entities, relations
}

// extractEntities 从文本提取实体
func (e *EntityExtractor) extractEntities(text string) []*storage.Entity {
	var entities []*storage.Entity

	// 提取技术/框架名称
	techEntities := e.extractTechnology(text)
	entities = append(entities, techEntities...)

	// 提取编程语言
	langEntities := e.extractProgrammingLanguage(text)
	entities = append(entities, langEntities...)

	// 提取 URL
	urlMatches := e.patterns["url"].FindAllString(text, -1)
	for _, url := range urlMatches {
		entities = append(entities, &storage.Entity{
			ID:        uuid.New().String(),
			Name:      url,
			Type:      "url",
			CreatedAt: time.Now(),
		})
	}

	// 提取文件路径
	pathMatches := e.patterns["file_path"].FindAllString(text, -1)
	for _, path := range pathMatches {
		entities = append(entities, &storage.Entity{
			ID:        uuid.New().String(),
			Name:      path,
			Type:      "file",
			CreatedAt: time.Now(),
		})
	}

	// 提取命令
	cmdMatches := e.patterns["command"].FindAllString(text, -1)
	for _, cmd := range cmdMatches {
		// 去掉反引号
		cmd = strings.Trim(cmd, "`")
		if len(cmd) > 3 && len(cmd) < 100 {
			entities = append(entities, &storage.Entity{
				ID:        uuid.New().String(),
				Name:      cmd,
				Type:      "command",
				CreatedAt: time.Now(),
			})
		}
	}

	return entities
}

// extractTechnology 提取技术名称
func (e *EntityExtractor) extractTechnology(text string) []*storage.Entity {
	var entities []*storage.Entity

	// 常见技术/框架
	technologies := []string{
		"React", "Vue", "Angular", "Next.js", "Nuxt",
		"Node.js", "Express", "Koa", "Fastify",
		"Django", "Flask", "FastAPI",
		"Spring", "SpringBoot", "Hibernate",
		"Docker", "Kubernetes", "K8s",
		"Redis", "MongoDB", "MySQL", "PostgreSQL", "Neo4j", "Milvus",
		"AWS", "Azure", "GCP", "Alibaba Cloud",
		"Kafka", "RabbitMQ", "RocketMQ",
		"Nginx", "Apache",
		"Git", "GitHub", "GitLab",
		"LangChain", "Eino", "OpenAI", "ChatGPT",
	}

	textLower := strings.ToLower(text)
	seen := make(map[string]bool)

	for _, tech := range technologies {
		if strings.Contains(textLower, strings.ToLower(tech)) && !seen[tech] {
			seen[tech] = true
			entities = append(entities, &storage.Entity{
				ID:        uuid.New().String(),
				Name:      tech,
				Type:      "technology",
				CreatedAt: time.Now(),
			})
		}
	}

	return entities
}

// extractProgrammingLanguage 提取编程语言
func (e *EntityExtractor) extractProgrammingLanguage(text string) []*storage.Entity {
	var entities []*storage.Entity

	// 从代码块提取语言
	langMatches := e.patterns["code_lang"].FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)

	for _, match := range langMatches {
		if len(match) > 1 {
			lang := match[1]
			if !seen[lang] {
				seen[lang] = true
				entities = append(entities, &storage.Entity{
					ID:        uuid.New().String(),
					Name:      lang,
					Type:      "language",
					CreatedAt: time.Now(),
				})
			}
		}
	}

	// 关键词匹配
	languages := map[string][]string{
		"Go":         {"golang", "go语言"},
		"Python":     {"python", "py"},
		"JavaScript": {"javascript", "js"},
		"TypeScript": {"typescript", "ts"},
		"Java":       {"java"},
		"C++":        {"c++", "cpp"},
		"Rust":       {"rust"},
		"Ruby":       {"ruby"},
		"PHP":        {"php"},
		"Swift":      {"swift"},
		"Kotlin":     {"kotlin"},
	}

	textLower := strings.ToLower(text)
	for lang, keywords := range languages {
		if seen[lang] || seen[strings.ToLower(lang)] {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(textLower, kw) {
				seen[lang] = true
				entities = append(entities, &storage.Entity{
					ID:        uuid.New().String(),
					Name:      lang,
					Type:      "language",
					CreatedAt: time.Now(),
				})
				break
			}
		}
	}

	return entities
}

// extractFromMetadata 从元数据提取实体
func (e *EntityExtractor) extractFromMetadata(metadata map[string]interface{}) []*storage.Entity {
	var entities []*storage.Entity

	// 提取工具调用
	if toolName, ok := metadata["tool_name"].(string); ok {
		entities = append(entities, &storage.Entity{
			ID:        uuid.New().String(),
			Name:      toolName,
			Type:      "tool",
			CreatedAt: time.Now(),
		})
	}

	// 提取文件
	if fileName, ok := metadata["file_name"].(string); ok {
		entities = append(entities, &storage.Entity{
			ID:        uuid.New().String(),
			Name:      fileName,
			Type:      "file",
			CreatedAt: time.Now(),
		})
	}

	return entities
}

// inferRelations 推断实体间的关系
func (e *EntityExtractor) inferRelations(entities []*storage.Entity, messages []*storage.Message) []*storage.Relation {
	var relations []*storage.Relation

	// 按类型分组实体
	entityByType := make(map[string][]*storage.Entity)
	for _, entity := range entities {
		entityByType[entity.Type] = append(entityByType[entity.Type], entity)
	}

	// 推断关系
	// 1. 语言 -> 技术框架
	if langs := entityByType["language"]; len(langs) > 0 {
		if techs := entityByType["technology"]; len(techs) > 0 {
			for _, lang := range langs {
				for _, tech := range techs {
					if isRelated(lang.Name, tech.Name) {
						relations = append(relations, &storage.Relation{
							ID:           uuid.New().String(),
							FromEntity:   lang.Name,
							ToEntity:     tech.Name,
							RelationType: "uses",
							Weight:       0.7,
							CreatedAt:    time.Now(),
						})
					}
				}
			}
		}
	}

	// 2. 同一消息中的实体更可能相关
	for _, msg := range messages {
		msgEntities := e.extractEntities(msg.Content)
		for i := 0; i < len(msgEntities)-1; i++ {
			for j := i + 1; j < len(msgEntities); j++ {
				if msgEntities[i].Type != msgEntities[j].Type {
					relations = append(relations, &storage.Relation{
						ID:           uuid.New().String(),
						FromEntity:   msgEntities[i].Name,
						ToEntity:     msgEntities[j].Name,
						RelationType: "mentioned_with",
						Weight:       0.5,
						CreatedAt:    time.Now(),
					})
				}
			}
		}
	}

	return relations
}

// isRelated 检查语言和技术是否相关
func isRelated(lang, tech string) bool {
	relations := map[string][]string{
		"Go":         {"Eino", "Docker", "Kubernetes", "K8s", "Redis", "Milvus"},
		"Python":     {"Django", "Flask", "FastAPI", "LangChain", "TensorFlow", "PyTorch"},
		"JavaScript": {"React", "Vue", "Angular", "Next.js", "Node.js", "Express"},
		"TypeScript": {"React", "Vue", "Angular", "Next.js", "Node.js", "Nest.js"},
		"Java":       {"Spring", "SpringBoot", "Hibernate", "Kafka"},
		"Rust":       {"Docker", "Kubernetes"},
	}

	if techs, ok := relations[lang]; ok {
		techLower := strings.ToLower(tech)
		for _, t := range techs {
			if strings.ToLower(t) == techLower {
				return true
			}
		}
	}

	return false
}

// EntityGraph 实体图谱
type EntityGraph struct {
	Entities  map[string]*storage.Entity
	Relations map[string][]*storage.Relation // key: fromEntity
}

// NewEntityGraph 创建实体图谱
func NewEntityGraph() *EntityGraph {
	return &EntityGraph{
		Entities:  make(map[string]*storage.Entity),
		Relations: make(map[string][]*storage.Relation),
	}
}

// AddEntity 添加实体
func (g *EntityGraph) AddEntity(entity *storage.Entity) {
	g.Entities[entity.Name] = entity
}

// AddRelation 添加关系
func (g *EntityGraph) AddRelation(relation *storage.Relation) {
	g.Relations[relation.FromEntity] = append(g.Relations[relation.FromEntity], relation)
}

// GetRelatedEntities 获取相关实体
func (g *EntityGraph) GetRelatedEntities(entityName string, depth int) []*storage.Entity {
	var result []*storage.Entity
	visited := make(map[string]bool)

	g.traverseRelations(entityName, depth, visited, &result)

	return result
}

func (g *EntityGraph) traverseRelations(entityName string, depth int, visited map[string]bool, result *[]*storage.Entity) {
	if depth <= 0 || visited[entityName] {
		return
	}

	visited[entityName] = true

	if entity, ok := g.Entities[entityName]; ok {
		*result = append(*result, entity)
	}

	for _, relation := range g.Relations[entityName] {
		g.traverseRelations(relation.ToEntity, depth-1, visited, result)
	}
}
