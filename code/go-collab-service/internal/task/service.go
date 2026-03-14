package task

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/openclaw-collab/go-collab-service/internal/common"
	"gorm.io/gorm"
)

var validTransitions = map[string][]string{
	"created":     {"assigned"},
	"assigned":    {"in_progress", "paused"},
	"in_progress": {"blocked", "review", "error", "paused", "completed"},
	"blocked":     {"in_progress", "paused", "error"},
	"review":      {"rejected", "completed"},
	"rejected":    {"in_progress"},
	"error":       {"in_progress", "paused"},
	"paused":      {"in_progress", "assigned"},
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(t *common.Task) error {
	if t.ID == "" {
		t.ID = "T" + uuid.New().String()[:8]
	}
	if t.Status == "" {
		t.Status = "created"
	}
	if t.Topology == "" {
		t.Topology = "free"
	}

	// Build materialized path
	if t.ParentID != nil && *t.ParentID != "" {
		parent, err := s.Get(*t.ParentID)
		if err != nil {
			return fmt.Errorf("parent task not found: %s", *t.ParentID)
		}
		t.Path = parent.Path + "/" + t.ID
		t.Level = parent.Level + 1
	} else {
		t.Path = t.ID
		t.Level = 0
	}

	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	return s.db.Create(t).Error
}

func (s *Service) Get(id string) (*common.Task, error) {
	var t common.Task
	if err := s.db.First(&t, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) UpdateStatus(id, newStatus string, fields map[string]interface{}) error {
	t, err := s.Get(id)
	if err != nil {
		return fmt.Errorf("task not found: %s", id)
	}

	if !isValidTransition(t.Status, newStatus) {
		return fmt.Errorf("invalid transition: %s -> %s", t.Status, newStatus)
	}

	// DAG dependency check
	if newStatus == "in_progress" && len(t.DependsOn) > 0 {
		for _, depID := range t.DependsOn {
			dep, err := s.Get(depID)
			if err != nil {
				return fmt.Errorf("dependency not found: %s", depID)
			}
			if dep.Status != "completed" {
				return fmt.Errorf("dependency %s not completed (status: %s)", depID, dep.Status)
			}
		}
	}

	updates := map[string]interface{}{
		"status":     newStatus,
		"updated_at": time.Now(),
	}
	for k, v := range fields {
		updates[k] = v
	}
	if newStatus == "completed" {
		now := time.Now()
		updates["completed_at"] = &now
	}

	if err := s.db.Model(&common.Task{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}

	// Record activity
	s.addActivity(id, "status_change", t.Assignee, fmt.Sprintf("%s -> %s", t.Status, newStatus))

	// Status propagation
	s.propagateStatus(t, newStatus)

	return nil
}

func (s *Service) Update(id string, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	return s.db.Model(&common.Task{}).Where("id = ?", id).Updates(fields).Error
}

func (s *Service) Query(params TaskQueryParams) ([]common.Task, error) {
	var tasks []common.Task
	q := s.db.Model(&common.Task{})

	if params.ProjectID != "" {
		q = q.Where("project_id = ?", params.ProjectID)
	}
	if params.Assignee != "" {
		q = q.Where("assignee = ?", params.Assignee)
	}
	if params.Status != "" {
		q = q.Where("status = ?", params.Status)
	}
	if params.ParentTaskID != "" {
		if params.IncludeSubtree {
			q = q.Where("path LIKE ?", "%"+params.ParentTaskID+"%")
		} else {
			q = q.Where("parent_id = ?", params.ParentTaskID)
		}
	}

	if err := q.Order("created_at ASC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Service) GetTree(id string) ([]common.Task, error) {
	var tasks []common.Task
	t, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if err := s.db.Where("path LIKE ?", t.Path+"%").Order("path ASC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Service) GetAncestors(id string) ([]common.Task, error) {
	t, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	var ancestors []common.Task
	current := t
	for current.ParentID != nil && *current.ParentID != "" {
		parent, err := s.Get(*current.ParentID)
		if err != nil {
			break
		}
		ancestors = append([]common.Task{*parent}, ancestors...)
		current = parent
	}
	return ancestors, nil
}

func (s *Service) GetStats(projectID string) common.TaskStats {
	var stats common.TaskStats
	q := s.db.Model(&common.Task{})
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	q.Count(&stats.Total)
	s.db.Model(&common.Task{}).Where("project_id = ? AND status = ?", projectID, "in_progress").Count(&stats.InProgress)
	s.db.Model(&common.Task{}).Where("project_id = ? AND status = ?", projectID, "blocked").Count(&stats.Blocked)
	s.db.Model(&common.Task{}).Where("project_id = ? AND status = ?", projectID, "completed").Count(&stats.Completed)
	s.db.Model(&common.Task{}).Where("project_id = ? AND status = ?", projectID, "review").Count(&stats.Review)
	return stats
}

func (s *Service) AddActivity(taskID, actType, actorID, content string) error {
	return s.addActivity(taskID, actType, actorID, content)
}

func (s *Service) GetActivities(taskID string) ([]common.TaskActivity, error) {
	var activities []common.TaskActivity
	if err := s.db.Where("task_id = ?", taskID).Order("created_at DESC").Limit(50).Find(&activities).Error; err != nil {
		return nil, err
	}
	return activities, nil
}

func (s *Service) FindStuckTasks(timeout time.Duration) ([]common.Task, error) {
	threshold := time.Now().Add(-timeout)
	var tasks []common.Task
	if err := s.db.Where("status = ? AND updated_at < ?", "in_progress", threshold).Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// --- Artifacts ---

func (s *Service) SaveArtifact(a *common.Artifact) error {
	if a.ID == "" {
		a.ID = "A" + uuid.New().String()[:8]
	}
	a.CreatedAt = time.Now()

	if err := s.db.Create(a).Error; err != nil {
		return err
	}

	// Link to task
	var t common.Task
	if err := s.db.First(&t, "id = ?", a.TaskID).Error; err == nil {
		artifacts := append(t.Artifacts, a.ID)
		s.db.Model(&t).Update("artifacts", artifacts)
	}
	return nil
}

func (s *Service) GetArtifacts(taskID string) ([]common.Artifact, error) {
	var artifacts []common.Artifact
	if err := s.db.Where("task_id = ?", taskID).Order("created_at DESC").Find(&artifacts).Error; err != nil {
		return nil, err
	}
	return artifacts, nil
}

// --- Internal ---

func (s *Service) addActivity(taskID, actType, actorID, content string) error {
	act := common.TaskActivity{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		Type:      actType,
		ActorID:   actorID,
		Content:   content,
		CreatedAt: time.Now(),
	}
	return s.db.Create(&act).Error
}

func (s *Service) propagateStatus(t *common.Task, newStatus string) {
	switch newStatus {
	case "completed":
		s.checkSiblingsCompleted(t)
		s.notifyDependents(t.ID)
	case "paused":
		s.cascadePause(t.ID)
	}
}

func (s *Service) checkSiblingsCompleted(t *common.Task) {
	if t.ParentID == nil || *t.ParentID == "" {
		return
	}
	var count int64
	s.db.Model(&common.Task{}).Where("parent_id = ? AND status != ?", *t.ParentID, "completed").Count(&count)
	if count == 0 {
		_ = s.UpdateStatus(*t.ParentID, "review", nil)
	}
}

func (s *Service) notifyDependents(completedID string) {
	var dependents []common.Task
	s.db.Where("depends_on::text LIKE ?", "%"+completedID+"%").Find(&dependents)
	for _, dep := range dependents {
		allReady := true
		for _, reqID := range dep.DependsOn {
			req, err := s.Get(reqID)
			if err != nil || req.Status != "completed" {
				allReady = false
				break
			}
		}
		if allReady && dep.Status == "assigned" {
			s.addActivity(dep.ID, "dependencies_ready", "system",
				fmt.Sprintf("All dependencies completed, task can start"))
		}
	}
}

func (s *Service) cascadePause(parentID string) {
	var children []common.Task
	s.db.Where("parent_id = ? AND status IN ?", parentID, []string{"in_progress", "assigned"}).Find(&children)
	for _, child := range children {
		s.db.Model(&child).Updates(map[string]interface{}{
			"status":     "paused",
			"updated_at": time.Now(),
		})
		s.addActivity(child.ID, "status_change", "system", "Cascade paused from parent")
		s.cascadePause(child.ID)
	}
}

func isValidTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}

type TaskQueryParams struct {
	ProjectID      string
	Assignee       string
	Status         string
	ParentTaskID   string
	IncludeSubtree bool
}
