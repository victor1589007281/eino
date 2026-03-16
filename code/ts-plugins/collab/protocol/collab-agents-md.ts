/**
 * M14: Collaboration Protocol
 *
 * Generates the standard collaboration protocol section to be appended
 * to each Agent's AGENTS.md file. This ensures all agents follow the
 * same task management, memory, and communication patterns.
 */

export function generateCollabProtocol(): string {
  return `
## 协作协议

### 上下文感知
每次收到消息时，上下文中已包含（由系统自动注入，标记为 \`---COLLAB_CONTEXT---\`）：
- 当前项目信息、迭代目标
- 我的进行中任务列表
- 关键决策（pinned decisions）
- 可调动的团队 Agent 列表
- 任务统计

基于这些上下文理解消息并处理。如果上下文中没有项目信息，说明当前不在项目群中。

### 任务管理
- 收到任务 → 调 update_task(status=in_progress) 表示开始
- 完成任务 → 调 update_task(status=review, result=交付物) + save_artifact
- 遇到阻塞 → 调 update_task(status=blocked, block_reason=原因)
- 需要拆分 → 调 create_task(parent_task_id=当前任务)
- 需要协作 → 调 create_task(assignee=目标Agent) 或 sessions_send

### 反失忆
以下场景**必须**调 save_decision(pin=true)：
- 技术选型、架构决策、用户指示、方向变更、重大约束发现

### 短期记忆
- 日常对话中的信息由 OpenClaw 自动管理（transcript + compaction）
- 压缩前系统会自动让你保存重要信息到 memory/
- 使用 memory_search 搜索历史持久化的记忆

### 消息格式
不需要手动添加角色前缀 — 系统自动添加。直接输出内容。

### 协作通信格式
Agent 间通信使用 sessions_send 时，消息体遵循以下结构：
\`\`\`
[角色] → [目标角色]
任务: {task_id}
类型: request|response|progress|escalation
内容: ...
\`\`\`

### Subagent 使用规范
- 适合并行处理的子任务使用 sessions_spawn 启动 subagent
- subagent 完成后结果通过 save_artifact 持久化
- 最多并发 3 个 subagent
`.trim();
}

/**
 * Generates the OpenClaw configuration snippet for a single agent.
 */
export function generateAgentOpenClawConfig(agent: {
  id: string;
  name: string;
  allowAgents: string[];
}): string {
  return JSON.stringify(
    {
      id: agent.id,
      name: agent.name,
      subagents: {
        allowAgents: agent.allowAgents,
      },
    },
    null,
    2
  );
}
