-- Agent Registry
CREATE TABLE IF NOT EXISTS agent_records (
    id              TEXT PRIMARY KEY,
    display_name    TEXT NOT NULL DEFAULT '',
    emoji           TEXT NOT NULL DEFAULT '',
    role            TEXT NOT NULL DEFAULT '',
    skills          JSONB NOT NULL DEFAULT '[]',
    status          TEXT NOT NULL DEFAULT 'idle',
    is_subagent     BOOLEAN NOT NULL DEFAULT FALSE,
    parent_agent    TEXT NOT NULL DEFAULT '',
    current_project_id TEXT NOT NULL DEFAULT '',
    current_load    INTEGER NOT NULL DEFAULT 0,
    registered_at   BIGINT NOT NULL DEFAULT 0,
    last_active_at  BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_agent_status ON agent_records(status);
CREATE INDEX IF NOT EXISTS idx_agent_project ON agent_records(current_project_id);

-- Projects
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    group_id    TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'active',
    team_agents JSONB NOT NULL DEFAULT '[]',
    tech_stack  JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Iterations
CREATE TABLE IF NOT EXISTS iterations (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    name        TEXT NOT NULL,
    goal        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'planning',
    summary     TEXT,
    start_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_iteration_project ON iterations(project_id);

-- Project Memory
CREATE TABLE IF NOT EXISTS project_memories (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    category    TEXT NOT NULL,
    content     TEXT NOT NULL,
    created_by  TEXT NOT NULL DEFAULT '',
    pinned      BOOLEAN NOT NULL DEFAULT FALSE,
    used_count  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_memory_project ON project_memories(project_id);
CREATE INDEX IF NOT EXISTS idx_memory_pinned ON project_memories(pinned);

-- Tasks
CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    parent_id   TEXT,
    path        TEXT NOT NULL DEFAULT '',
    level       INTEGER NOT NULL DEFAULT 0,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    assignee    TEXT NOT NULL DEFAULT '',
    assigned_by TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'created',
    priority    TEXT NOT NULL DEFAULT 'P2',
    deliverable TEXT NOT NULL DEFAULT '',
    acceptance  TEXT NOT NULL DEFAULT '',
    result      TEXT,
    block_reason TEXT,
    error_info  TEXT,
    depends_on  JSONB NOT NULL DEFAULT '[]',
    artifacts   JSONB NOT NULL DEFAULT '[]',
    topology    TEXT NOT NULL DEFAULT 'free',
    deadline    TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_task_assignee ON tasks(assignee);
CREATE INDEX IF NOT EXISTS idx_task_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_task_parent ON tasks(parent_id);
CREATE INDEX IF NOT EXISTS idx_task_path ON tasks(path);

-- Task Activities
CREATE TABLE IF NOT EXISTS task_activities (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL,
    type        TEXT NOT NULL,
    actor_id    TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activity_task ON task_activities(task_id);

-- Artifacts
CREATE TABLE IF NOT EXISTS artifacts (
    id          TEXT PRIMARY KEY,
    task_id     TEXT NOT NULL,
    project_id  TEXT NOT NULL DEFAULT '',
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_artifact_task ON artifacts(task_id);
CREATE INDEX IF NOT EXISTS idx_artifact_project ON artifacts(project_id);

-- Agent Experiences
CREATE TABLE IF NOT EXISTS agent_experiences (
    id          TEXT PRIMARY KEY,
    agent_id    TEXT NOT NULL,
    category    TEXT NOT NULL,
    content     TEXT NOT NULL,
    tags        JSONB NOT NULL DEFAULT '[]',
    used_count  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_experience_agent ON agent_experiences(agent_id);

-- Bot Mappings (Feishu)
CREATE TABLE IF NOT EXISTS bot_mappings (
    id          TEXT PRIMARY KEY,
    agent_id    TEXT NOT NULL,
    bot_app_id  TEXT NOT NULL DEFAULT '',
    group_id    TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_bot_agent ON bot_mappings(agent_id);
