package task

import (
	"fmt"
	"testing"
	"time"

	"github.com/openclaw-collab/go-collab-service/internal/common"
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
	db.AutoMigrate(&common.Task{}, &common.TaskActivity{}, &common.Artifact{})
	db.Exec("DELETE FROM task_activities WHERE task_id LIKE 'test-%' OR task_id LIKE 'T%'")
	db.Exec("DELETE FROM artifacts WHERE task_id LIKE 'test-%' OR task_id LIKE 'T%'")
	db.Exec("DELETE FROM tasks WHERE id LIKE 'test-%' OR project_id = 'test-proj'")
	return db
}

func TestCreateTask(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	task := &common.Task{
		ProjectID:   "test-proj",
		Title:       "Implement auth module",
		Description: "JWT-based authentication",
		Assignee:    "dev-1",
		AssignedBy:  "rd-mgr",
		Priority:    "P1",
	}

	err := svc.Create(task)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, task.ID, task.Path)
	assert.Equal(t, 0, task.Level)

	// Create subtask
	parentID := task.ID
	sub := &common.Task{
		ProjectID:  "test-proj",
		ParentID:   &parentID,
		Title:      "Add JWT middleware",
		Assignee:   "dev-1",
		AssignedBy: "dev-1",
	}
	err = svc.Create(sub)
	require.NoError(t, err)
	assert.Equal(t, parentID+"/"+sub.ID, sub.Path)
	assert.Equal(t, 1, sub.Level)

	// Cleanup
	db.Exec("DELETE FROM tasks WHERE project_id = 'test-proj'")
}

func TestTaskStatusTransition(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	task := &common.Task{
		ProjectID: "test-proj",
		Title:     "Test transition",
		Assignee:  "dev-1",
		Status:    "created",
	}
	svc.Create(task)
	defer db.Exec("DELETE FROM tasks WHERE project_id = 'test-proj'")

	// Valid: created -> assigned
	err := svc.UpdateStatus(task.ID, "assigned", nil)
	require.NoError(t, err)

	// Valid: assigned -> in_progress
	err = svc.UpdateStatus(task.ID, "in_progress", nil)
	require.NoError(t, err)

	// Valid: in_progress -> review
	err = svc.UpdateStatus(task.ID, "review", nil)
	require.NoError(t, err)

	// Valid: review -> completed
	err = svc.UpdateStatus(task.ID, "completed", nil)
	require.NoError(t, err)

	got, _ := svc.Get(task.ID)
	assert.NotNil(t, got.CompletedAt)

	// Invalid transition
	err = svc.UpdateStatus(task.ID, "in_progress", nil)
	assert.Error(t, err)
}

func TestInvalidTransitionRejected(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	task := &common.Task{
		ProjectID: "test-proj",
		Title:     "Invalid transition",
		Assignee:  "dev-1",
		Status:    "created",
	}
	svc.Create(task)
	defer db.Exec("DELETE FROM tasks WHERE project_id = 'test-proj'")

	// created -> completed is invalid
	err := svc.UpdateStatus(task.ID, "completed", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transition")
}

func TestDAGDependency(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	defer db.Exec("DELETE FROM tasks WHERE project_id = 'test-proj'")

	// Create task A
	a := &common.Task{ProjectID: "test-proj", Title: "Task A", Assignee: "dev-1"}
	svc.Create(a)

	// Create task B that depends on A
	b := &common.Task{
		ProjectID: "test-proj",
		Title:     "Task B",
		Assignee:  "dev-2",
		DependsOn: common.StringSlice{a.ID},
	}
	svc.Create(b)

	// Assign B
	svc.UpdateStatus(b.ID, "assigned", nil)

	// Try to start B (should fail — A not completed)
	err := svc.UpdateStatus(b.ID, "in_progress", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not completed")

	// Complete A, then B should be startable
	svc.UpdateStatus(a.ID, "assigned", nil)
	svc.UpdateStatus(a.ID, "in_progress", nil)
	svc.UpdateStatus(a.ID, "review", nil)
	svc.UpdateStatus(a.ID, "completed", nil)

	err = svc.UpdateStatus(b.ID, "in_progress", nil)
	assert.NoError(t, err)
}

func TestQueryTasks(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	defer db.Exec("DELETE FROM tasks WHERE project_id = 'test-proj'")

	for i := 0; i < 5; i++ {
		svc.Create(&common.Task{
			ProjectID: "test-proj",
			Title:     "Task " + string(rune('A'+i)),
			Assignee:  "dev-1",
			Status:    "created",
		})
	}

	tasks, err := svc.Query(TaskQueryParams{ProjectID: "test-proj"})
	require.NoError(t, err)
	assert.Len(t, tasks, 5)
}

func TestFindStuckTasks(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	defer db.Exec("DELETE FROM tasks WHERE project_id = 'test-proj'")

	stuck := &common.Task{
		ProjectID: "test-proj",
		Title:     "Stuck task",
		Assignee:  "dev-1",
		Status:    "in_progress",
	}
	svc.Create(stuck)
	// Manually set updated_at to 2 hours ago
	db.Model(&common.Task{}).Where("id = ?", stuck.ID).Update("updated_at", time.Now().Add(-2*time.Hour))

	found, err := svc.FindStuckTasks(30 * time.Minute)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(found), 1)
}

func TestArtifacts(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	defer db.Exec("DELETE FROM tasks WHERE project_id = 'test-proj'")
	defer db.Exec("DELETE FROM artifacts WHERE project_id = 'test-proj'")

	task := &common.Task{ProjectID: "test-proj", Title: "With artifact", Assignee: "dev-1"}
	svc.Create(task)

	art := &common.Artifact{
		TaskID:    task.ID,
		ProjectID: "test-proj",
		Name:      "design.md",
		Type:      "document",
		Content:   "# Architecture\nMicroservices...",
		CreatedBy: "arch-1",
	}
	err := svc.SaveArtifact(art)
	require.NoError(t, err)
	assert.NotEmpty(t, art.ID)

	artifacts, err := svc.GetArtifacts(task.ID)
	require.NoError(t, err)
	assert.Len(t, artifacts, 1)
	assert.Equal(t, "design.md", artifacts[0].Name)
}

func TestGetStats(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	defer db.Exec("DELETE FROM tasks WHERE project_id = 'test-proj-stats'")

	statuses := []string{"created", "in_progress", "in_progress", "blocked", "completed"}
	for i, s := range statuses {
		svc.Create(&common.Task{
			ProjectID: "test-proj-stats",
			Title:     "S" + fmt.Sprintf("%d", i),
			Assignee:  "dev-1",
			Status:    s,
		})
	}

	stats := svc.GetStats("test-proj-stats")
	assert.Equal(t, int64(5), stats.Total)
	assert.Equal(t, int64(2), stats.InProgress)
	assert.Equal(t, int64(1), stats.Blocked)
	assert.Equal(t, int64(1), stats.Completed)
}
