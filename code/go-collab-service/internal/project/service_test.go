package project

import (
	"testing"

	"github.com/openclaw-collab/go-collab-service/internal/common"
	"github.com/openclaw-collab/go-collab-service/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := "host=localhost user=collab password=collab dbname=collab port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	db.AutoMigrate(
		&common.Project{},
		&common.Iteration{},
		&common.ProjectMemory{},
		&common.Task{},
		&common.TaskActivity{},
		&common.Artifact{},
		&common.AgentRecord{},
		&common.AgentExperience{},
	)
	return db
}

func cleanupProject(db *gorm.DB, id string) {
	db.Exec("DELETE FROM project_memories WHERE project_id = ?", id)
	db.Exec("DELETE FROM iterations WHERE project_id = ?", id)
	db.Exec("DELETE FROM task_activities WHERE task_id IN (SELECT id FROM tasks WHERE project_id = ?)", id)
	db.Exec("DELETE FROM artifacts WHERE project_id = ?", id)
	db.Exec("DELETE FROM tasks WHERE project_id = ?", id)
	db.Exec("DELETE FROM projects WHERE id = ?", id)
}

func TestCreateAndGetProject(t *testing.T) {
	db := setupTestDB(t)
	taskSvc := task.NewService(db)
	svc := NewService(db, taskSvc)

	p := &common.Project{
		Name:       "Test Project",
		GroupID:    "oc_test_grp_" + t.Name(),
		TeamAgents: common.StringSlice{"arch", "dev"},
		TechStack:  common.StringSlice{"Go", "React"},
	}

	err := svc.Create(p)
	require.NoError(t, err)
	defer cleanupProject(db, p.ID)

	got, err := svc.Get(p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Project", got.Name)
	assert.Contains(t, []string(got.TeamAgents), "arch")

	// Get by group
	byGroup, err := svc.GetByGroup(p.GroupID)
	require.NoError(t, err)
	assert.Equal(t, p.ID, byGroup.ID)
}

func TestProjectMemory(t *testing.T) {
	db := setupTestDB(t)
	taskSvc := task.NewService(db)
	svc := NewService(db, taskSvc)

	p := &common.Project{Name: "Mem Project", GroupID: "oc_mem_" + t.Name()}
	svc.Create(p)
	defer cleanupProject(db, p.ID)

	// Add memories
	svc.AddMemory(&common.ProjectMemory{
		ProjectID: p.ID,
		Category:  "decision",
		Content:   "Use PostgreSQL for persistence",
		Pinned:    true,
		CreatedBy: "arch",
	})
	svc.AddMemory(&common.ProjectMemory{
		ProjectID: p.ID,
		Category:  "tech_choice",
		Content:   "Gin for HTTP framework",
		CreatedBy: "arch",
	})

	// Query
	mems, err := svc.QueryMemory(p.ID, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, mems, 2)

	// Query by category
	mems, err = svc.QueryMemory(p.ID, "decision", "", 10)
	require.NoError(t, err)
	assert.Len(t, mems, 1)

	// Query by keyword
	mems, err = svc.QueryMemory(p.ID, "", "PostgreSQL", 10)
	require.NoError(t, err)
	assert.Len(t, mems, 1)
}

func TestIterations(t *testing.T) {
	db := setupTestDB(t)
	taskSvc := task.NewService(db)
	svc := NewService(db, taskSvc)

	p := &common.Project{Name: "Iter Project", GroupID: "oc_iter_" + t.Name()}
	svc.Create(p)
	defer cleanupProject(db, p.ID)

	iter := &common.Iteration{
		ProjectID: p.ID,
		Name:      "Sprint 1",
		Goal:      "Core modules",
		Status:    "active",
	}
	err := svc.CreateIteration(iter)
	require.NoError(t, err)

	iters, err := svc.ListIterations(p.ID)
	require.NoError(t, err)
	assert.Len(t, iters, 1)
	assert.Equal(t, "Sprint 1", iters[0].Name)
}

func TestGetContext(t *testing.T) {
	db := setupTestDB(t)
	taskSvc := task.NewService(db)
	svc := NewService(db, taskSvc)

	p := &common.Project{
		Name:       "Context Project",
		GroupID:    "oc_ctx_" + t.Name(),
		TeamAgents: common.StringSlice{"arch-1", "dev-1"},
	}
	svc.Create(p)
	defer cleanupProject(db, p.ID)

	// Add some data
	svc.CreateIteration(&common.Iteration{
		ProjectID: p.ID,
		Name:      "Sprint 1",
		Goal:      "Core",
		Status:    "active",
	})
	svc.AddMemory(&common.ProjectMemory{
		ProjectID: p.ID,
		Category:  "decision",
		Content:   "Use microservices",
		Pinned:    true,
	})

	// Create a task
	taskSvc.Create(&common.Task{
		ProjectID: p.ID,
		Title:     "Build API",
		Assignee:  "dev-1",
		Status:    "in_progress",
	})

	// Get context
	ctx, err := svc.GetContext(p.ID, "dev-1")
	require.NoError(t, err)
	assert.Equal(t, p.Name, ctx.Project.Name)
	assert.NotNil(t, ctx.CurrentIteration)
	assert.Len(t, ctx.PinnedMemories, 1)
	assert.GreaterOrEqual(t, len(ctx.MyTasks), 1)
}

func TestExperiences(t *testing.T) {
	db := setupTestDB(t)
	taskSvc := task.NewService(db)
	svc := NewService(db, taskSvc)

	agentID := "test-exp-agent"
	defer db.Exec("DELETE FROM agent_experiences WHERE agent_id = ?", agentID)

	err := svc.AddExperience(&common.AgentExperience{
		AgentID:  agentID,
		Category: "pitfall",
		Content:  "Always validate JWT token expiry",
		Tags:     common.StringSlice{"auth", "jwt"},
	})
	require.NoError(t, err)

	exps, err := svc.QueryExperiences(agentID, []string{"jwt"}, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(exps), 1)
}
