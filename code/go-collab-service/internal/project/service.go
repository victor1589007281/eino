package project

import (
	"fmt"
	"time"

	"github.com/google/uuid"
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

// --- Project CRUD ---

func (s *Service) Create(p *common.Project) error {
	if p.ID == "" {
		p.ID = "P" + uuid.New().String()[:8]
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return s.db.Create(p).Error
}

func (s *Service) Get(id string) (*common.Project, error) {
	var p common.Project
	if err := s.db.First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) GetByGroup(groupID string) (*common.Project, error) {
	var p common.Project
	if err := s.db.First(&p, "group_id = ?", groupID).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) Update(id string, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	return s.db.Model(&common.Project{}).Where("id = ?", id).Updates(fields).Error
}

func (s *Service) List() ([]common.Project, error) {
	var projects []common.Project
	if err := s.db.Order("created_at DESC").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// --- Context Loading ---

func (s *Service) GetContext(projectID, agentID string) (*common.CollabContext, error) {
	p, err := s.Get(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	ctx := &common.CollabContext{
		Project: *p,
	}

	// Current iteration
	var iter common.Iteration
	if err := s.db.Where("project_id = ? AND status = ?", projectID, "active").First(&iter).Error; err == nil {
		ctx.CurrentIteration = &iter
	}

	// Pinned memories (ordered by recency, limited)
	s.db.Where("project_id = ? AND pinned = ?", projectID, true).
		Order("created_at DESC").Limit(10).Find(&ctx.PinnedMemories)

	// Agent's tasks
	if agentID != "" {
		s.db.Where("project_id = ? AND assignee = ? AND status IN ?",
			projectID, agentID, []string{"assigned", "in_progress", "blocked", "review"}).
			Order("priority ASC, created_at ASC").Limit(10).Find(&ctx.MyTasks)
	}

	// Team agents
	var agents []common.AgentRecord
	s.db.Where("id IN ?", []string(p.TeamAgents)).Find(&agents)
	ctx.TeamAgents = agents

	// Stats
	ctx.Stats = s.taskSvc.GetStats(projectID)

	// Experiences
	if agentID != "" {
		s.db.Where("agent_id = ?", agentID).
			Order("used_count DESC, created_at DESC").Limit(5).Find(&ctx.Experiences)
	}

	return ctx, nil
}

// --- Iterations ---

func (s *Service) CreateIteration(iter *common.Iteration) error {
	if iter.ID == "" {
		iter.ID = "I" + uuid.New().String()[:8]
	}
	iter.CreatedAt = time.Now()
	iter.UpdatedAt = time.Now()
	return s.db.Create(iter).Error
}

func (s *Service) UpdateIteration(id string, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	return s.db.Model(&common.Iteration{}).Where("id = ?", id).Updates(fields).Error
}

func (s *Service) ListIterations(projectID string) ([]common.Iteration, error) {
	var iters []common.Iteration
	if err := s.db.Where("project_id = ?", projectID).Order("start_at DESC").Find(&iters).Error; err != nil {
		return nil, err
	}
	return iters, nil
}

// --- Memory ---

func (s *Service) AddMemory(m *common.ProjectMemory) error {
	if m.ID == "" {
		m.ID = "M" + uuid.New().String()[:8]
	}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	return s.db.Create(m).Error
}

func (s *Service) QueryMemory(projectID, category, query string, limit int) ([]common.ProjectMemory, error) {
	var memories []common.ProjectMemory
	q := s.db.Where("project_id = ?", projectID)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if query != "" {
		q = q.Where("content ILIKE ?", "%"+query+"%")
	}
	if limit <= 0 {
		limit = 20
	}
	if err := q.Order("pinned DESC, used_count DESC, created_at DESC").Limit(limit).Find(&memories).Error; err != nil {
		return nil, err
	}
	return memories, nil
}

func (s *Service) UpdateMemory(id string, fields map[string]interface{}) error {
	fields["updated_at"] = time.Now()
	return s.db.Model(&common.ProjectMemory{}).Where("id = ?", id).Updates(fields).Error
}

func (s *Service) SyncMemories(projectID, agentID string, memories []common.ProjectMemory) error {
	for i := range memories {
		memories[i].ProjectID = projectID
		memories[i].CreatedBy = agentID
		if err := s.AddMemory(&memories[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Experiences ---

func (s *Service) AddExperience(exp *common.AgentExperience) error {
	if exp.ID == "" {
		exp.ID = "E" + uuid.New().String()[:8]
	}
	exp.CreatedAt = time.Now()
	return s.db.Create(exp).Error
}

func (s *Service) QueryExperiences(agentID string, keywords []string, limit int) ([]common.AgentExperience, error) {
	var exps []common.AgentExperience
	q := s.db.Where("agent_id = ?", agentID)
	if len(keywords) > 0 {
		for _, kw := range keywords {
			q = q.Or("content ILIKE ? AND agent_id = ?", "%"+kw+"%", agentID)
		}
	}
	if limit <= 0 {
		limit = 10
	}
	if err := q.Order("used_count DESC, created_at DESC").Limit(limit).Find(&exps).Error; err != nil {
		return nil, err
	}
	return exps, nil
}
