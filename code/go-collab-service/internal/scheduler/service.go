package scheduler

import (
	"log"
	"time"

	"github.com/openclaw-collab/go-collab-service/internal/common"
	"github.com/openclaw-collab/go-collab-service/internal/feishu"
	"github.com/openclaw-collab/go-collab-service/internal/registry"
	"github.com/openclaw-collab/go-collab-service/internal/task"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type Service struct {
	db          *gorm.DB
	taskSvc     *task.Service
	registrySvc *registry.Service
	feishuSvc   *feishu.Service
	cron        *cron.Cron
}

func NewService(db *gorm.DB, taskSvc *task.Service, registrySvc *registry.Service, feishuSvc *feishu.Service) *Service {
	return &Service{
		db:          db,
		taskSvc:     taskSvc,
		registrySvc: registrySvc,
		feishuSvc:   feishuSvc,
		cron:        cron.New(),
	}
}

func (s *Service) Start() {
	s.cron.AddFunc("@every 5m", s.checkStuckTasks)
	s.cron.AddFunc("@every 10m", s.checkAgentHealth)
	s.cron.AddFunc("@every 30m", s.cleanupStaleSubagents)
	s.cron.Start()
	log.Println("[scheduler] Started cron jobs")
}

func (s *Service) Stop() {
	s.cron.Stop()
	log.Println("[scheduler] Stopped cron jobs")
}

func (s *Service) checkStuckTasks() {
	stuck, err := s.taskSvc.FindStuckTasks(30 * time.Minute)
	if err != nil {
		log.Printf("[scheduler] Error finding stuck tasks: %v", err)
		return
	}

	for _, t := range stuck {
		log.Printf("[scheduler] Stuck task: %s (assignee: %s, since: %v)", t.ID, t.Assignee, t.UpdatedAt)
		s.taskSvc.AddActivity(t.ID, "comment", "scheduler",
			"⚠️ Task stuck for 30+ minutes without update")

		// Notify via Feishu
		s.feishuSvc.Send(feishu.SendRequest{
			AgentID:   "system",
			ChannelID: s.findProjectGroup(t.ProjectID),
			Content:   "⚠️ 任务超时预警\n任务: " + t.Title + "\n负责人: " + t.Assignee + "\n已停滞 30+ 分钟",
		})
	}
}

func (s *Service) checkAgentHealth() {
	agents, err := s.registrySvc.List("", "")
	if err != nil {
		log.Printf("[scheduler] Error listing agents: %v", err)
		return
	}

	threshold := time.Now().Add(-15 * time.Minute).Unix()
	for _, a := range agents {
		if a.Status != "offline" && a.LastActiveAt < threshold {
			log.Printf("[scheduler] Agent %s inactive since %v", a.ID, time.Unix(a.LastActiveAt, 0))
			s.registrySvc.UpdateStatus(a.ID, "offline")
		}
	}
}

func (s *Service) cleanupStaleSubagents() {
	var stale []common.AgentRecord
	threshold := time.Now().Add(-2 * time.Hour).Unix()
	s.db.Where("is_subagent = ? AND last_active_at < ?", true, threshold).Find(&stale)

	for _, sub := range stale {
		log.Printf("[scheduler] Cleaning stale subagent: %s (parent: %s)", sub.ID, sub.ParentAgent)
		s.db.Delete(&sub)
	}
}

func (s *Service) findProjectGroup(projectID string) string {
	var p common.Project
	if err := s.db.First(&p, "id = ?", projectID).Error; err != nil {
		return ""
	}
	return p.GroupID
}
