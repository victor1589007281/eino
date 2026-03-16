package common

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StringSlice is a custom type for JSON-encoded string slices in PostgreSQL.
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
	return json.Unmarshal(bytes, s)
}

// --- Agent Record ---

type AgentRecord struct {
	ID               string      `json:"id" gorm:"primaryKey"`
	DisplayName      string      `json:"display_name"`
	Emoji            string      `json:"emoji"`
	Role             string      `json:"role"`
	Skills           StringSlice `json:"skills" gorm:"type:jsonb;default:'[]'"`
	Status           string      `json:"status" gorm:"default:'idle'"` // idle / busy / error / offline
	IsSubagent       bool        `json:"is_subagent" gorm:"default:false"`
	ParentAgent      string      `json:"parent_agent"`
	CurrentProjectID string      `json:"current_project_id"`
	CurrentLoad      int         `json:"current_load" gorm:"default:0"`
	RegisteredAt     int64       `json:"registered_at"`
	LastActiveAt     int64       `json:"last_active_at"`
}

func (AgentRecord) TableName() string { return "agent_records" }

// --- Project ---

type Project struct {
	ID          string      `json:"id" gorm:"primaryKey"`
	Name        string      `json:"name"`
	Description string      `json:"description" gorm:"type:text"`
	GroupID     string      `json:"group_id" gorm:"uniqueIndex"`
	Status      string      `json:"status" gorm:"default:'active'"` // active / paused / archived
	TeamAgents  StringSlice `json:"team_agents" gorm:"type:jsonb;default:'[]'"`
	TechStack   StringSlice `json:"tech_stack" gorm:"type:jsonb;default:'[]'"`
	// 仓库信息（JSON 存储，支持多个仓库）
	Repositories  RepositoryConfig `json:"repositories" gorm:"type:jsonb"`
	// 文档输出目录（绝对路径）
	DocsBaseDir   string      `json:"docs_base_dir"`  // 文档根目录绝对路径，如：/home/victor/docs
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// 仓库配置（支持多个仓库）
type RepositoryConfig struct {
	CodeRepos    []CodeRepository    `json:"code_repos"`     // 代码仓库列表
	DocRepos     []DocRepository     `json:"doc_repos"`      // 文档仓库（可选）
	ReferenceRepos []ReferenceRepository `json:"reference_repos"` // 参考源码仓库
}

// 验证仓库路径是否存在
func (r *RepositoryConfig) Validate() error {
	// 验证代码仓库
	for i, repo := range r.CodeRepos {
		if repo.Name == "" {
			return fmt.Errorf("code_repos[%d]: name is required", i)
		}
		if repo.LocalPath == "" {
			return fmt.Errorf("code_repos[%d]: local_path is required", i)
		}
		if !filepath.IsAbs(repo.LocalPath) {
			return fmt.Errorf("code_repos[%d]: local_path must be absolute path, got: %s", i, repo.LocalPath)
		}
		// 在 K8s 环境中，路径需要映射到挂载的目录
		// 检查路径是否在允许的 base 目录内
		if !isPathInAllowedBase(repo.LocalPath) {
			return fmt.Errorf("code_repos[%d]: path %s is not in allowed base directories", i, repo.LocalPath)
		}
		// 检查路径是否存在（在 K8s 中，这个检查可能会失败，因为目录可能还没挂载）
		// 所以只警告，不阻止
		if _, err := os.Stat(repo.LocalPath); os.IsNotExist(err) {
			// 在 K8s 环境中，这可能是正常的（目录还未创建）
			// 只记录警告，不返回错误
			fmt.Printf("[WARN] code_repos[%d]: path does not exist yet: %s (may be created later)\n", i, repo.LocalPath)
		}
	}
	
	// 验证文档仓库
	for i, repo := range r.DocRepos {
		if repo.Name == "" {
			return fmt.Errorf("doc_repos[%d]: name is required", i)
		}
		if repo.LocalPath == "" {
			return fmt.Errorf("doc_repos[%d]: local_path is required", i)
		}
		if !filepath.IsAbs(repo.LocalPath) {
			return fmt.Errorf("doc_repos[%d]: local_path must be absolute path, got: %s", i, repo.LocalPath)
		}
		if !isPathInAllowedBase(repo.LocalPath) {
			return fmt.Errorf("doc_repos[%d]: path %s is not in allowed base directories", i, repo.LocalPath)
		}
	}
	
	// 验证参考仓库
	for i, repo := range r.ReferenceRepos {
		if repo.Name == "" {
			return fmt.Errorf("reference_repos[%d]: name is required", i)
		}
		if repo.LocalPath == "" {
			return fmt.Errorf("reference_repos[%d]: local_path is required", i)
		}
		if !filepath.IsAbs(repo.LocalPath) {
			return fmt.Errorf("reference_repos[%d]: local_path must be absolute path, got: %s", i, repo.LocalPath)
		}
		if !isPathInAllowedBase(repo.LocalPath) {
			return fmt.Errorf("reference_repos[%d]: path %s is not in allowed base directories", i, repo.LocalPath)
		}
	}
	
	return nil
}

// 检查路径是否在允许的 base 目录内
// 在 K8s 环境中，只允许访问挂载的目录
func isPathInAllowedBase(path string) bool {
	// 允许的 base 目录列表
	allowedBases := []string{
		"/home/victor/base/git",      // K8s 挂载的 Git 仓库目录
		"/home/victor/.openclaw",     // K8s 挂载的 OpenClaw 目录
		"/tmp",                        // 临时目录
		"/root/.openclaw",            // Pod 内的 OpenClaw 目录
	}
	
	cleanPath := filepath.Clean(path)
	
	for _, base := range allowedBases {
		// 检查路径是否以 base 开头
		if strings.HasPrefix(cleanPath, base) {
			// 确保是完整的目录名，不是前缀匹配
			if cleanPath == base || strings.HasPrefix(cleanPath, base+"/") {
				return true
			}
		}
	}
	
	return false
}

// 获取主代码仓库
func (r *RepositoryConfig) GetMainCodeRepo() *CodeRepository {
	for _, repo := range r.CodeRepos {
		if repo.Type == "main" {
			return &repo
		}
	}
	// 如果没有 main 类型，返回第一个
	if len(r.CodeRepos) > 0 {
		return &r.CodeRepos[0]
	}
	return nil
}

// 获取项目文档目录结构（返回绝对路径）
func (p *Project) GetDocPaths() map[string]string {
	baseDir := p.DocsBaseDir
	if baseDir == "" {
		baseDir = "/tmp/docs" // 默认值
	}
	
	return map[string]string{
		"designs":  filepath.Join(baseDir, p.ID, "designs"),   // 功能设计文档
		"research": filepath.Join(baseDir, p.ID, "research"),  // 调研分析报告
		"system":   filepath.Join(baseDir, p.ID, "system"),    // 模块实现文档
		"reports":  filepath.Join(baseDir, p.ID, "reports"),   // AI 任务汇总报告
	}
}

// 验证文档目录是否存在，不存在则创建
func (p *Project) EnsureDocDirs() error {
	docPaths := p.GetDocPaths()
	for dirType, path := range docPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("failed to create %s directory %s: %v", dirType, path, err)
			}
		}
	}
	return nil
}

// 代码仓库
type CodeRepository struct {
	Name       string `json:"name"`        // 仓库名称（如：backend, frontend）
	Type       string `json:"type"`        // 类型：main（主仓库）, reference（参考仓库）
	GitURL     string `json:"git_url"`     // Git 远程地址
	LocalPath  string `json:"local_path"`  // 本地绝对路径
	Branch     string `json:"branch"`      // 默认分支
}

// 文档仓库（可选，如果项目有独立文档仓库）
type DocRepository struct {
	Name      string `json:"name"`
	GitURL    string `json:"git_url"`
	LocalPath string `json:"local_path"`
	Branch    string `json:"branch"`
}

// 参考源码仓库（如：参考的开源项目）
type ReferenceRepository struct {
	Name      string `json:"name"`        // 仓库名称
	GitURL    string `json:"git_url"`     // Git 地址
	LocalPath string `json:"local_path"`  // 本地绝对路径
	Purpose   string `json:"purpose"`     // 用途说明
}

func (Project) TableName() string { return "projects" }

// --- Iteration ---

type Iteration struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	ProjectID string    `json:"project_id" gorm:"index"`
	Name      string    `json:"name"`
	Goal      string    `json:"goal" gorm:"type:text"`
	Status    string    `json:"status" gorm:"default:'planning'"` // planning / active / completed
	Summary   *string   `json:"summary" gorm:"type:text"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Iteration) TableName() string { return "iterations" }

// --- Project Memory ---

type ProjectMemory struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	ProjectID string    `json:"project_id" gorm:"index"`
	Category  string    `json:"category"` // decision / tech_choice / lesson / architecture / user_directive
	Content   string    `json:"content" gorm:"type:text"`
	CreatedBy string    `json:"created_by"`
	Pinned    bool      `json:"pinned" gorm:"index;default:false"`
	UsedCount int       `json:"used_count" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ProjectMemory) TableName() string { return "project_memories" }

// --- Task ---

type Task struct {
	ID          string      `json:"id" gorm:"primaryKey"`
	ProjectID   string      `json:"project_id" gorm:"index"`
	ParentID    *string     `json:"parent_id" gorm:"index"`
	Path        string      `json:"path" gorm:"index"`
	Level       int         `json:"level"` // 0=Epic 1=Task 2=SubTask
	Title       string      `json:"title"`
	Description string      `json:"description" gorm:"type:text"`
	Assignee    string      `json:"assignee" gorm:"index"`
	AssignedBy  string      `json:"assigned_by"`
	Status      string      `json:"status" gorm:"index;default:'created'"` // created/assigned/in_progress/blocked/review/rejected/completed/error/paused
	Priority    string      `json:"priority" gorm:"default:'P2'"`
	Deliverable string      `json:"deliverable" gorm:"type:text"`
	Acceptance  string      `json:"acceptance" gorm:"type:text"`
	Result      *string     `json:"result" gorm:"type:text"`
	BlockReason *string     `json:"block_reason"`
	ErrorInfo   *string     `json:"error_info"`
	DependsOn   StringSlice `json:"depends_on" gorm:"type:jsonb;default:'[]'"`
	Artifacts   StringSlice `json:"artifacts" gorm:"type:jsonb;default:'[]'"`
	Topology    string      `json:"topology" gorm:"default:'free'"` // pipeline/fanout/fanin/review/free
	Deadline    *time.Time  `json:"deadline"`
	CompletedAt *time.Time  `json:"completed_at"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

func (Task) TableName() string { return "tasks" }

// UnmarshalJSON implements custom JSON unmarshaling for Task
// to support both camelCase and snake_case field names
func (t *Task) UnmarshalJSON(data []byte) error {
	// Create an alias type to avoid infinite recursion
	type Alias Task
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	
	// First try standard unmarshaling
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	
	// Handle camelCase fallbacks if snake_case fields are empty
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err == nil {
		// Check for camelCase variants and use them if snake_case is empty
		if t.ProjectID == "" {
			if v, ok := raw["projectId"].(string); ok {
				t.ProjectID = v
			}
		}
		if t.ParentID == nil {
			if v, ok := raw["parentId"].(string); ok && v != "" {
				t.ParentID = &v
			}
		}
		if t.AssignedBy == "" {
			if v, ok := raw["assignedBy"].(string); ok {
				t.AssignedBy = v
			}
		}
		if t.Deliverable == "" {
			if v, ok := raw["deliverable"].(string); ok {
				t.Deliverable = v
			}
		}
		if t.Acceptance == "" {
			if v, ok := raw["acceptance"].(string); ok {
				t.Acceptance = v
			}
		}
		if t.BlockReason == nil {
			if v, ok := raw["blockReason"].(string); ok {
				t.BlockReason = &v
			}
		}
		if t.ErrorInfo == nil {
			if v, ok := raw["errorInfo"].(string); ok {
				t.ErrorInfo = &v
			}
		}
		if t.DependsOn == nil || len(t.DependsOn) == 0 {
			if v, ok := raw["dependsOn"].([]interface{}); ok {
				dependsOn := make([]string, len(v))
				for i, item := range v {
					if s, ok := item.(string); ok {
						dependsOn[i] = s
					}
				}
				t.DependsOn = dependsOn
			}
		}
		if t.Artifacts == nil || len(t.Artifacts) == 0 {
			if v, ok := raw["artifacts"].([]interface{}); ok {
				artifacts := make([]string, len(v))
				for i, item := range v {
					if s, ok := item.(string); ok {
						artifacts[i] = s
					}
				}
				t.Artifacts = artifacts
			}
		}
	}
	
	return nil
}

// --- Task Activity ---

type TaskActivity struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	TaskID    string    `json:"task_id" gorm:"index"`
	Type      string    `json:"type"` // status_change / comment / agent_message / subagent_spawn / tool_error / compaction_sync
	ActorID   string    `json:"actor_id"`
	Content   string    `json:"content" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}

func (TaskActivity) TableName() string { return "task_activities" }

// --- Artifact ---

type Artifact struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	TaskID    string    `json:"task_id" gorm:"index"`
	ProjectID string    `json:"project_id" gorm:"index"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // document / code / config / test_report
	Content   string    `json:"content" gorm:"type:text"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (Artifact) TableName() string { return "artifacts" }

// --- Agent Experience ---

type AgentExperience struct {
	ID        string      `json:"id" gorm:"primaryKey"`
	AgentID   string      `json:"agent_id" gorm:"index"`
	Category  string      `json:"category"` // pitfall / best_practice / tool_tip
	Content   string      `json:"content" gorm:"type:text"`
	Tags      StringSlice `json:"tags" gorm:"type:jsonb;default:'[]'"`
	UsedCount int         `json:"used_count" gorm:"default:0"`
	CreatedAt time.Time   `json:"created_at"`
}

func (AgentExperience) TableName() string { return "agent_experiences" }

// --- Bot Mapping ---

type BotMapping struct {
	ID       string `json:"id" gorm:"primaryKey"`
	AgentID  string `json:"agent_id" gorm:"index"`
	BotAppID string `json:"bot_app_id"`
	GroupID  string `json:"group_id"`
}

func (BotMapping) TableName() string { return "bot_mappings" }

// --- API Types ---

type TaskStats struct {
	Total      int64 `json:"total"`
	InProgress int64 `json:"in_progress"`
	Blocked    int64 `json:"blocked"`
	Completed  int64 `json:"completed"`
	Review     int64 `json:"review"`
}

type CollabContext struct {
	Project          Project          `json:"project"`
	CurrentIteration *Iteration       `json:"current_iteration"`
	PinnedMemories   []ProjectMemory  `json:"pinned_memories"`
	MyTasks          []Task           `json:"my_tasks"`
	TeamAgents       []AgentRecord    `json:"team_agents"`
	Stats            TaskStats        `json:"stats"`
	Experiences      []AgentExperience `json:"experiences"`
}

type DashboardOverview struct {
	Projects []ProjectSummary `json:"projects"`
	Agents   []AgentSummary   `json:"agents"`
	Alerts   []Alert          `json:"alerts"`
}

type ProjectSummary struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Status string    `json:"status"`
	Stats  TaskStats `json:"stats"`
}

type AgentSummary struct {
	ID             string      `json:"id"`
	DisplayName    string      `json:"display_name"`
	Emoji          string      `json:"emoji"`
	Status         string      `json:"status"`
	CurrentLoad    int         `json:"current_load"`
	ActiveProjects StringSlice `json:"active_projects"`
}

type Alert struct {
	Severity  string `json:"severity"` // critical / warning / info
	TaskID    string `json:"task_id"`
	Agent     string `json:"agent"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}
