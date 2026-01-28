// Package skills 提供技能加载和管理功能
package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// SkillLoader 技能加载器
type SkillLoader struct {
	skillsDir string
	skills    map[string]*Skill
	mu        sync.RWMutex
}

// Skill 技能定义
type Skill struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Keywords    []string          `json:"keywords"`
	Content     string            `json:"content"`
	Examples    []Example         `json:"examples"`
	Metadata    map[string]string `json:"metadata"`
}

// Example 示例
type Example struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// NewSkillLoader 创建技能加载器
func NewSkillLoader(skillsDir string) *SkillLoader {
	return &SkillLoader{
		skillsDir: skillsDir,
		skills:    make(map[string]*Skill),
	}
}

// LoadAll 加载所有技能
func (l *SkillLoader) LoadAll(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// 遍历技能目录
	err := filepath.Walk(l.skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// 只处理markdown文件
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			skill, err := l.loadSkillFromFile(path)
			if err != nil {
				return fmt.Errorf("failed to load skill from %s: %w", path, err)
			}
			l.skills[skill.ID] = skill
		}
		
		return nil
	})
	
	return err
}

// loadSkillFromFile 从文件加载技能
func (l *SkillLoader) loadSkillFromFile(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	// 解析markdown文件
	skill := l.parseSkillMarkdown(string(content), path)
	
	return skill, nil
}

// parseSkillMarkdown 解析技能markdown
func (l *SkillLoader) parseSkillMarkdown(content, path string) *Skill {
	skill := &Skill{
		ID:       l.generateSkillID(path),
		Metadata: make(map[string]string),
	}
	
	lines := strings.Split(content, "\n")
	
	// 解析YAML前置元数据（如果有）
	if len(lines) > 0 && lines[0] == "---" {
		endIndex := -1
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" {
				endIndex = i
				break
			}
		}
		
		if endIndex > 0 {
			// 解析元数据
			for i := 1; i < endIndex; i++ {
				parts := strings.SplitN(lines[i], ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					
					switch key {
					case "name":
						skill.Name = value
					case "category":
						skill.Category = value
					case "description":
						skill.Description = value
					case "keywords":
						skill.Keywords = strings.Split(value, ",")
						for i := range skill.Keywords {
							skill.Keywords[i] = strings.TrimSpace(skill.Keywords[i])
						}
					default:
						skill.Metadata[key] = value
					}
				}
			}
			
			// 移除元数据部分
			lines = lines[endIndex+1:]
		}
	}
	
	// 解析正文
	var contentBuilder strings.Builder
	var currentSection string
	var examples []Example
	var currentExample *Example
	
	headingPattern := regexp.MustCompile(`^#+\s+(.+)$`)
	
	for _, line := range lines {
		if match := headingPattern.FindStringSubmatch(line); match != nil {
			// 标题
			title := strings.ToLower(match[1])
			
			if skill.Name == "" {
				skill.Name = match[1]
			}
			
			if strings.Contains(title, "example") || strings.Contains(title, "示例") {
				currentSection = "example"
				if currentExample != nil {
					examples = append(examples, *currentExample)
				}
				currentExample = &Example{}
			} else if strings.Contains(title, "input") || strings.Contains(title, "输入") {
				currentSection = "input"
			} else if strings.Contains(title, "output") || strings.Contains(title, "输出") {
				currentSection = "output"
			} else {
				currentSection = ""
			}
		} else {
			// 正文内容
			switch currentSection {
			case "input":
				if currentExample != nil {
					currentExample.Input += line + "\n"
				}
			case "output":
				if currentExample != nil {
					currentExample.Output += line + "\n"
				}
			default:
				contentBuilder.WriteString(line)
				contentBuilder.WriteString("\n")
			}
		}
	}
	
	// 保存最后一个示例
	if currentExample != nil && (currentExample.Input != "" || currentExample.Output != "") {
		examples = append(examples, *currentExample)
	}
	
	skill.Content = strings.TrimSpace(contentBuilder.String())
	skill.Examples = examples
	
	return skill
}

// generateSkillID 生成技能ID
func (l *SkillLoader) generateSkillID(path string) string {
	// 使用相对路径作为ID
	relPath, err := filepath.Rel(l.skillsDir, path)
	if err != nil {
		relPath = path
	}
	
	// 移除扩展名
	id := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	// 替换路径分隔符
	id = strings.ReplaceAll(id, string(filepath.Separator), ".")
	
	return id
}

// GetSkill 获取技能
func (l *SkillLoader) GetSkill(id string) (*Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	skill, ok := l.skills[id]
	return skill, ok
}

// GetSkillsByCategory 按类别获取技能
func (l *SkillLoader) GetSkillsByCategory(category string) []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	var result []*Skill
	for _, skill := range l.skills {
		if skill.Category == category {
			result = append(result, skill)
		}
	}
	return result
}

// SearchSkills 搜索技能
func (l *SkillLoader) SearchSkills(query string) []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	query = strings.ToLower(query)
	var result []*Skill
	
	for _, skill := range l.skills {
		// 匹配名称
		if strings.Contains(strings.ToLower(skill.Name), query) {
			result = append(result, skill)
			continue
		}
		
		// 匹配描述
		if strings.Contains(strings.ToLower(skill.Description), query) {
			result = append(result, skill)
			continue
		}
		
		// 匹配关键词
		for _, keyword := range skill.Keywords {
			if strings.Contains(strings.ToLower(keyword), query) {
				result = append(result, skill)
				break
			}
		}
	}
	
	return result
}

// GetAllSkills 获取所有技能
func (l *SkillLoader) GetAllSkills() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	result := make([]*Skill, 0, len(l.skills))
	for _, skill := range l.skills {
		result = append(result, skill)
	}
	return result
}

// ReloadSkill 重新加载单个技能
func (l *SkillLoader) ReloadSkill(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// 找到对应的文件路径
	for _, skill := range l.skills {
		if skill.ID == id {
			path := filepath.Join(l.skillsDir, strings.ReplaceAll(id, ".", string(filepath.Separator))+".md")
			
			newSkill, err := l.loadSkillFromFile(path)
			if err != nil {
				return err
			}
			
			l.skills[id] = newSkill
			return nil
		}
	}
	
	return fmt.Errorf("skill %s not found", id)
}
