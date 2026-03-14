package dashboard

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
		&common.Task{},
		&common.TaskActivity{},
		&common.AgentRecord{},
		&common.Artifact{},
	)
	return db
}

func TestDashboardOverview(t *testing.T) {
	db := setupTestDB(t)
	taskSvc := task.NewService(db)
	svc := NewService(db, taskSvc)

	projID := "test-dash-proj"
	defer func() {
		db.Exec("DELETE FROM task_activities WHERE task_id IN (SELECT id FROM tasks WHERE project_id = ?)", projID)
		db.Exec("DELETE FROM tasks WHERE project_id = ?", projID)
		db.Exec("DELETE FROM projects WHERE id = ?", projID)
		db.Exec("DELETE FROM agent_records WHERE id LIKE 'test-dash-%'")
	}()

	db.Create(&common.Project{
		ID:      projID,
		Name:    "Dashboard Test",
		GroupID: "oc_dash_test",
		Status:  "active",
	})
	db.Create(&common.AgentRecord{
		ID:          "test-dash-dev",
		DisplayName: "Dev",
		Emoji:       "👨‍💻",
		Status:      "idle",
	})

	taskSvc.Create(&common.Task{
		ProjectID: projID,
		Title:     "Test task",
		Assignee:  "test-dash-dev",
		Status:    "in_progress",
	})

	overview, err := svc.GetOverview()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(overview.Projects), 1)
}

func TestProjectDetail(t *testing.T) {
	db := setupTestDB(t)
	taskSvc := task.NewService(db)
	svc := NewService(db, taskSvc)

	projID := "test-dash-detail"
	defer func() {
		db.Exec("DELETE FROM tasks WHERE project_id = ?", projID)
		db.Exec("DELETE FROM projects WHERE id = ?", projID)
	}()

	db.Create(&common.Project{
		ID:      projID,
		Name:    "Detail Test",
		GroupID: "oc_detail_test",
		Status:  "active",
	})

	detail, err := svc.GetProjectDetail(projID)
	require.NoError(t, err)
	assert.Equal(t, "Detail Test", detail["project"].(common.Project).Name)
}
