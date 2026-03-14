package feishu

import (
	"fmt"
	"log"

	"github.com/openclaw-collab/go-collab-service/internal/common"
	"github.com/openclaw-collab/go-collab-service/internal/registry"
	"gorm.io/gorm"
)

type Service struct {
	db           *gorm.DB
	registrySvc  *registry.Service
	defaultBotID string
}

func NewService(db *gorm.DB, registrySvc *registry.Service, defaultBotID string) *Service {
	return &Service{db: db, registrySvc: registrySvc, defaultBotID: defaultBotID}
}

type SendRequest struct {
	AgentID       string `json:"agent_id"`
	ChannelID     string `json:"channel_id"`
	Content       string `json:"content"`
	IsSubagent    bool   `json:"is_subagent"`
	ParentAgentID string `json:"parent_agent_id"`
	SubagentName  string `json:"subagent_name"`
	Format        string `json:"format"`
}

type BotSelectRequest struct {
	AgentID       string `json:"agent_id"`
	GroupID       string `json:"group_id"`
	IsSubagent    bool   `json:"is_subagent"`
	ParentAgentID string `json:"parent_agent_id"`
}

type BotSelectResponse struct {
	BotAppID string `json:"bot_app_id"`
}

func (s *Service) SelectBot(req BotSelectRequest) string {
	lookupAgent := req.AgentID
	if req.IsSubagent && req.ParentAgentID != "" {
		lookupAgent = req.ParentAgentID
	}

	var mapping common.BotMapping
	err := s.db.Where("agent_id = ? AND (group_id = ? OR group_id = '')", lookupAgent, req.GroupID).
		Order("group_id DESC"). // prefer group-specific mapping
		First(&mapping).Error
	if err == nil {
		return mapping.BotAppID
	}

	return s.defaultBotID
}

func (s *Service) BuildHeader(req SendRequest) string {
	agent, err := s.registrySvc.Get(req.AgentID)
	if err != nil {
		return ""
	}

	if req.IsSubagent && req.ParentAgentID != "" {
		parent, err := s.registrySvc.Get(req.ParentAgentID)
		if err != nil {
			return fmt.Sprintf("【%s %s → Sub:%s】", agent.Emoji, agent.DisplayName, req.SubagentName)
		}
		return fmt.Sprintf("【%s %s → Sub:%s】", parent.Emoji, parent.DisplayName, req.SubagentName)
	}
	return fmt.Sprintf("【%s %s】", agent.Emoji, agent.DisplayName)
}

func (s *Service) Send(req SendRequest) error {
	header := s.BuildHeader(req)
	botID := s.SelectBot(BotSelectRequest{
		AgentID:       req.AgentID,
		GroupID:       req.ChannelID,
		IsSubagent:    req.IsSubagent,
		ParentAgentID: req.ParentAgentID,
	})

	message := header + "\n" + req.Content
	log.Printf("[feishu] Send via bot=%s to=%s: %s", botID, req.ChannelID, truncate(message, 100))
	// In production, call Feishu API here
	// For now, log the message
	return nil
}

// --- Bot Mapping CRUD ---

func (s *Service) GetBotMapping(agentID, groupID string) (*common.BotMapping, error) {
	var m common.BotMapping
	if err := s.db.Where("agent_id = ? AND group_id = ?", agentID, groupID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Service) SetBotMapping(m *common.BotMapping) error {
	if m.ID == "" {
		m.ID = m.AgentID + ":" + m.GroupID
	}
	return s.db.Save(m).Error
}

func (s *Service) ListBotMappings() ([]common.BotMapping, error) {
	var mappings []common.BotMapping
	if err := s.db.Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
