package dashboard

import (
	"time"

	"github.com/openclaw-collab/go-collab-service/internal/common"
	"github.com/openclaw-collab/go-collab-service/internal/task"
	"gorm.io/gorm"
)

type Service struct {
	db      *gorm.DB
	taskSvc *task.Service
}

func NewService(db *gorm.DB, taskSvc *task.Service) *Service {
	return &Service{db: db, taskSvc: taskSvc}
}

func (s *Service) GetOverview() (*common.DashboardOverview, error) {
	overview := &common.DashboardOverview{}

	// Projects
	var projects []common.Project
	s.db.Where("status = ?", "active").Find(&projects)
	for _, p := range projects {
		stats := s.taskSvc.GetStats(p.ID)
		overview.Projects = append(overview.Projects, common.ProjectSummary{
			ID:     p.ID,
			Name:   p.Name,
			Status: p.Status,
			Stats:  stats,
		})
	}

	// Agents
	var agents []common.AgentRecord
	s.db.Where("is_subagent = ?", false).Find(&agents)
	for _, a := range agents {
		var activeProjects common.StringSlice
		s.db.Model(&common.Task{}).
			Where("assignee = ? AND status IN ?", a.ID, []string{"in_progress", "assigned"}).
			Distinct("project_id").Pluck("project_id", &activeProjects)
		overview.Agents = append(overview.Agents, common.AgentSummary{
			ID:             a.ID,
			DisplayName:    a.DisplayName,
			Emoji:          a.Emoji,
			Status:         a.Status,
			CurrentLoad:    a.CurrentLoad,
			ActiveProjects: activeProjects,
		})
	}

	// Alerts — blocked/error tasks
	var alertTasks []common.Task
	s.db.Where("status IN ? AND updated_at > ?", []string{"blocked", "error"},
		time.Now().Add(-24*time.Hour)).
		Order("updated_at DESC").Limit(20).Find(&alertTasks)
	for _, t := range alertTasks {
		severity := "warning"
		if t.Status == "error" {
			severity = "critical"
		}
		msg := ""
		if t.BlockReason != nil {
			msg = *t.BlockReason
		}
		if t.ErrorInfo != nil {
			msg = *t.ErrorInfo
		}
		overview.Alerts = append(overview.Alerts, common.Alert{
			Severity:  severity,
			TaskID:    t.ID,
			Agent:     t.Assignee,
			Message:   msg,
			Timestamp: t.UpdatedAt.Unix(),
		})
	}

	return overview, nil
}

func (s *Service) GetProjectDetail(projectID string) (map[string]interface{}, error) {
	var p common.Project
	if err := s.db.First(&p, "id = ?", projectID).Error; err != nil {
		return nil, err
	}

	tasks, _ := s.taskSvc.Query(task.TaskQueryParams{ProjectID: projectID})
	stats := s.taskSvc.GetStats(projectID)

	return map[string]interface{}{
		"project": p,
		"tasks":   tasks,
		"stats":   stats,
	}, nil
}

func (s *Service) GetAgentDetail(agentID string) (map[string]interface{}, error) {
	var agent common.AgentRecord
	if err := s.db.First(&agent, "id = ?", agentID).Error; err != nil {
		return nil, err
	}

	tasks, _ := s.taskSvc.Query(task.TaskQueryParams{Assignee: agentID})

	return map[string]interface{}{
		"agent": agent,
		"tasks": tasks,
	}, nil
}
