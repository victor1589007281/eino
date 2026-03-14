package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/openclaw-collab/go-collab-service/internal/common"
	"github.com/openclaw-collab/go-collab-service/internal/dashboard"
	"github.com/openclaw-collab/go-collab-service/internal/feishu"
	"github.com/openclaw-collab/go-collab-service/internal/project"
	"github.com/openclaw-collab/go-collab-service/internal/registry"
	"github.com/openclaw-collab/go-collab-service/internal/scheduler"
	"github.com/openclaw-collab/go-collab-service/internal/task"
)

func main() {
	cfg := common.LoadConfig()

	db := common.InitDB(cfg.DatabaseDSN)
	common.AutoMigrate(db,
		&common.AgentRecord{},
		&common.Project{},
		&common.Iteration{},
		&common.ProjectMemory{},
		&common.Task{},
		&common.TaskActivity{},
		&common.Artifact{},
		&common.AgentExperience{},
		&common.BotMapping{},
	)

	// Services
	registrySvc := registry.NewService(db)
	taskSvc := task.NewService(db)
	projectSvc := project.NewService(db, taskSvc)
	feishuSvc := feishu.NewService(db, registrySvc, cfg.DefaultBotID)
	dashboardSvc := dashboard.NewService(db, taskSvc)
	schedulerSvc := scheduler.NewService(db, taskSvc, registrySvc, feishuSvc)

	// Auto-scan OpenClaw agents directory
	scanner := registry.NewScanner(db, cfg.OpenClawDir)
	if err := scanner.ScanAgents(); err != nil {
		log.Printf("[server] Agent scan warning: %v", err)
	}

	// Handlers
	registryHandler := registry.NewHandler(registrySvc)
	taskHandler := task.NewHandler(taskSvc)
	projectHandler := project.NewHandler(projectSvc)
	feishuHandler := feishu.NewHandler(feishuSvc)
	dashboardHandler := dashboard.NewHandler(dashboardSvc)

	// Router
	r := gin.Default()
	r.Use(common.RequestLogger())
	r.Use(common.CORS())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	registryHandler.RegisterRoutes(api)
	taskHandler.RegisterRoutes(api)
	projectHandler.RegisterRoutes(api)
	feishuHandler.RegisterRoutes(api)
	dashboardHandler.RegisterRoutes(api)

	// Scheduler
	schedulerSvc.Start()
	defer schedulerSvc.Stop()

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("[server] Starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
