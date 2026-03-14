package feishu

import (
	"testing"

	"github.com/openclaw-collab/go-collab-service/internal/common"
	"github.com/openclaw-collab/go-collab-service/internal/registry"
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
	db.AutoMigrate(&common.AgentRecord{}, &common.BotMapping{})
	return db
}

func TestSelectBot(t *testing.T) {
	db := setupTestDB(t)
	registrySvc := registry.NewService(db)
	svc := NewService(db, registrySvc, "default-bot")

	defer db.Exec("DELETE FROM bot_mappings WHERE id LIKE 'test-%'")

	// No mapping — should return default
	bot := svc.SelectBot(BotSelectRequest{
		AgentID: "agent-1",
		GroupID: "oc_group_1",
	})
	assert.Equal(t, "default-bot", bot)

	// Add mapping
	svc.SetBotMapping(&common.BotMapping{
		ID:       "test-agent-1:oc_group_1",
		AgentID:  "agent-1",
		BotAppID: "bot-for-agent-1",
		GroupID:  "oc_group_1",
	})

	bot = svc.SelectBot(BotSelectRequest{
		AgentID: "agent-1",
		GroupID: "oc_group_1",
	})
	assert.Equal(t, "bot-for-agent-1", bot)
}

func TestBuildHeader(t *testing.T) {
	db := setupTestDB(t)
	registrySvc := registry.NewService(db)
	svc := NewService(db, registrySvc, "default-bot")

	defer db.Exec("DELETE FROM agent_records WHERE id LIKE 'test-%'")

	registrySvc.Register(&common.AgentRecord{
		ID:          "test-arch",
		DisplayName: "架构师",
		Emoji:       "🏗️",
		Role:        "architect",
	})

	// Normal agent header
	header := svc.BuildHeader(SendRequest{
		AgentID: "test-arch",
	})
	assert.Contains(t, header, "🏗️")
	assert.Contains(t, header, "架构师")

	// Subagent header — subagent uses parent's identity
	registrySvc.Register(&common.AgentRecord{
		ID:          "test-sub-agent",
		DisplayName: "子Agent",
		Emoji:       "🔧",
		Role:        "subagent",
		IsSubagent:  true,
		ParentAgent: "test-arch",
	})
	header = svc.BuildHeader(SendRequest{
		AgentID:       "test-sub-agent",
		IsSubagent:    true,
		ParentAgentID: "test-arch",
		SubagentName:  "code-review",
	})
	assert.Contains(t, header, "架构师")
	assert.Contains(t, header, "Sub:code-review")
}

func TestSend(t *testing.T) {
	db := setupTestDB(t)
	registrySvc := registry.NewService(db)
	svc := NewService(db, registrySvc, "default-bot")

	defer db.Exec("DELETE FROM agent_records WHERE id LIKE 'test-%'")

	registrySvc.Register(&common.AgentRecord{
		ID:          "test-sender",
		DisplayName: "开发者",
		Emoji:       "👨‍💻",
	})

	err := svc.Send(SendRequest{
		AgentID:   "test-sender",
		ChannelID: "oc_test_group",
		Content:   "Task completed!",
	})
	require.NoError(t, err)
}

func TestBotMappingCRUD(t *testing.T) {
	db := setupTestDB(t)
	registrySvc := registry.NewService(db)
	svc := NewService(db, registrySvc, "default-bot")

	defer db.Exec("DELETE FROM bot_mappings WHERE id LIKE 'test-%'")

	err := svc.SetBotMapping(&common.BotMapping{
		ID:       "test-crud-agent:group",
		AgentID:  "test-crud-agent",
		BotAppID: "bot-x",
		GroupID:  "group",
	})
	require.NoError(t, err)

	mapping, err := svc.GetBotMapping("test-crud-agent", "group")
	require.NoError(t, err)
	assert.Equal(t, "bot-x", mapping.BotAppID)

	mappings, err := svc.ListBotMappings()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(mappings), 1)
}
