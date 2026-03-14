package registry

import (
	"testing"

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
	db.AutoMigrate(&common.AgentRecord{})
	db.Exec("DELETE FROM agent_records WHERE id LIKE 'test-%'")
	return db
}

func TestRegisterAndGet(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	agent := &common.AgentRecord{
		ID:          "test-arch",
		DisplayName: "架构师",
		Emoji:       "🏗️",
		Role:        "architect",
		Skills:      common.StringSlice{"design", "review"},
		Status:      "idle",
	}

	err := svc.Register(agent)
	require.NoError(t, err)

	got, err := svc.Get("test-arch")
	require.NoError(t, err)
	assert.Equal(t, "架构师", got.DisplayName)
	assert.Equal(t, "🏗️", got.Emoji)
	assert.Equal(t, "idle", got.Status)
	assert.Contains(t, []string(got.Skills), "design")

	// Cleanup
	svc.Delete("test-arch")
}

func TestListWithFilters(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	agents := []*common.AgentRecord{
		{ID: "test-a1", DisplayName: "A1", Role: "dev", Status: "idle", CurrentProjectID: "proj-1"},
		{ID: "test-a2", DisplayName: "A2", Role: "dev", Status: "busy", CurrentProjectID: "proj-1"},
		{ID: "test-a3", DisplayName: "A3", Role: "qa", Status: "idle", CurrentProjectID: "proj-2"},
	}
	for _, a := range agents {
		svc.Register(a)
	}
	defer func() {
		for _, a := range agents {
			svc.Delete(a.ID)
		}
	}()

	// Filter by project
	list, err := svc.List("proj-1", "")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Filter by status
	list, err = svc.List("", "busy")
	require.NoError(t, err)
	found := false
	for _, a := range list {
		if a.ID == "test-a2" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestUpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	svc.Register(&common.AgentRecord{ID: "test-status", Status: "idle"})
	defer svc.Delete("test-status")

	err := svc.UpdateStatus("test-status", "busy")
	require.NoError(t, err)

	got, _ := svc.Get("test-status")
	assert.Equal(t, "busy", got.Status)
}

func TestParseAgentIDFromSessionKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"agent:architect:subagent:abc123", "architect"},
		{"agent:dev-manager", "dev-manager"},
		{"plain-id", "plain-id"},
	}
	for _, tc := range tests {
		got := parseAgentIDFromSessionKey(tc.key)
		assert.Equal(t, tc.expected, got, "key=%s", tc.key)
	}
}
