package common

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
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
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
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
