package registry

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/openclaw-collab/go-collab-service/internal/common"
	"gorm.io/gorm"
)

type Scanner struct {
	db          *gorm.DB
	openClawDir string
}

func NewScanner(db *gorm.DB, openClawDir string) *Scanner {
	return &Scanner{db: db, openClawDir: openClawDir}
}

// ScanAgents scans the OpenClaw agents directory and upserts records
// into the agent_records table. It reads each agent's SYSTEM_PROMPT.md
// or SOUL.md to extract metadata (display_name, emoji, role).
func (s *Scanner) ScanAgents() error {
	agentsDir := filepath.Join(s.openClawDir, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return fmt.Errorf("failed to read agents dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentID := entry.Name()
		agentPath := filepath.Join(agentsDir, agentID, "agent")

		if _, err := os.Stat(agentPath); os.IsNotExist(err) {
			continue
		}

		record := s.parseAgentRecord(agentID, agentPath)
		svc := NewService(s.db)
		if err := svc.Upsert(record); err != nil {
			log.Printf("[scanner] Failed to upsert agent %s: %v", agentID, err)
		} else {
			log.Printf("[scanner] Registered agent: %s (%s)", agentID, record.DisplayName)
		}
	}

	return nil
}

func (s *Scanner) parseAgentRecord(agentID, agentPath string) *common.AgentRecord {
	record := &common.AgentRecord{
		ID:          agentID,
		DisplayName: agentID,
		Role:        "agent",
		Status:      "idle",
		Skills:      common.StringSlice{},
	}

	// Try SYSTEM_PROMPT.md first, then SOUL.md
	for _, filename := range []string{"SYSTEM_PROMPT.md", "SOUL.md"} {
		content, err := os.ReadFile(filepath.Join(agentPath, filename))
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)

			if strings.HasPrefix(line, "# ") {
				name := strings.TrimPrefix(line, "# ")
				// Extract emoji if present
				if len(name) > 0 {
					runes := []rune(name)
					if len(runes) > 0 && !isASCII(runes[0]) {
						record.Emoji = string(runes[0])
						record.DisplayName = strings.TrimSpace(string(runes[1:]))
					} else {
						record.DisplayName = name
					}
				}
			}

			if strings.HasPrefix(line, "- 角色:") || strings.HasPrefix(line, "- Role:") {
				record.Role = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}

			if strings.HasPrefix(line, "- 别名:") || strings.HasPrefix(line, "- Alias:") {
				// Could extract aliases for Feishu mention patterns
			}
		}
		break
	}

	return record
}

func isASCII(r rune) bool {
	return r < 128
}
