package registry

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/openclaw-collab/go-collab-service/internal/common"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(projectID, status string) ([]common.AgentRecord, error) {
	var agents []common.AgentRecord
	q := s.db.Model(&common.AgentRecord{})
	if projectID != "" {
		q = q.Where("current_project_id = ?", projectID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Order("id ASC").Find(&agents).Error; err != nil {
		return nil, err
	}
	return agents, nil
}

func (s *Service) Get(id string) (*common.AgentRecord, error) {
	var agent common.AgentRecord
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func (s *Service) GetBySessionKey(sessionKey string) (*common.AgentRecord, error) {
	agentID := parseAgentIDFromSessionKey(sessionKey)
	if agentID == "" {
		return nil, fmt.Errorf("invalid session key: %s", sessionKey)
	}
	return s.Get(agentID)
}

func (s *Service) Register(agent *common.AgentRecord) error {
	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}
	agent.RegisteredAt = time.Now().Unix()
	agent.LastActiveAt = time.Now().Unix()
	return s.db.Create(agent).Error
}

func (s *Service) Upsert(agent *common.AgentRecord) error {
	agent.LastActiveAt = time.Now().Unix()
	return s.db.Save(agent).Error
}

func (s *Service) Delete(id string) error {
	return s.db.Delete(&common.AgentRecord{}, "id = ?", id).Error
}

func (s *Service) UpdateStatus(id, status string) error {
	return s.db.Model(&common.AgentRecord{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":         status,
		"last_active_at": time.Now().Unix(),
	}).Error
}

func (s *Service) UpdateLoad(id string, load int) error {
	return s.db.Model(&common.AgentRecord{}).Where("id = ?", id).Updates(map[string]interface{}{
		"current_load":   load,
		"last_active_at": time.Now().Unix(),
	}).Error
}

func parseAgentIDFromSessionKey(key string) string {
	// Format: "agent:{agentId}" or "agent:{agentId}:subagent:{uuid}"
	if len(key) < 7 {
		return ""
	}
	// Simple parsing
	rest := key
	if len(rest) > 6 && rest[:6] == "agent:" {
		rest = rest[6:]
	} else {
		return key // treat the whole thing as an agent ID
	}
	for i, c := range rest {
		if c == ':' {
			return rest[:i]
		}
	}
	return rest
}
