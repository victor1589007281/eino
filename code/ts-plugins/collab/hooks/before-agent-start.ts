import type { GoBridge } from "../go-bridge.js";
import type {
  BeforeAgentStartEvent,
  BeforeAgentStartResult,
  CollabContext,
  Task,
  AgentRecord,
  AgentExperience,
  ProjectMemory,
} from "../types.js";
import { buildBudgetedContext } from "./context-budget.js";

export function createBeforeAgentStartHook(bridge: GoBridge, maxTokens?: number) {
  return async (event: BeforeAgentStartEvent): Promise<BeforeAgentStartResult> => {
    const { agentId, sessionKey } = event;
    const projectId = extractProjectId(sessionKey);
    if (!projectId) {
      return {};
    }

    const ctx = await bridge.getContext(projectId, agentId);
    if (!ctx) {
      return {};
    }

    await bridge.updateAgentStatus(agentId, "busy");

    const contextBlock = maxTokens
      ? buildBudgetedContext(ctx, agentId, { maxTokens })
      : buildContextBlock(ctx, agentId);
    return {
      prependContext: contextBlock,
    };
  };
}

function extractProjectId(sessionKey: string): string | null {
  // Session keys from group chats carry the group ID
  // We look up the project by group. For DMs, return null.
  const parts = sessionKey.split(":");
  for (const part of parts) {
    if (part.startsWith("oc_")) return part;
  }
  return null;
}

function buildContextBlock(ctx: CollabContext, agentId: string): string {
  const lines: string[] = [];
  lines.push("=== 📋 协作上下文 (自动注入, 勿向用户展示此块) ===\n");

  // Project info
  lines.push(`## 当前项目: ${ctx.project.name}`);
  lines.push(`- 状态: ${ctx.project.status}`);
  if (ctx.project.tech_stack?.length > 0) {
    lines.push(`- 技术栈: ${ctx.project.tech_stack.join(", ")}`);
  }

  // Iteration
  if (ctx.current_iteration) {
    lines.push(`\n## 当前迭代: ${ctx.current_iteration.name}`);
    lines.push(`- 目标: ${ctx.current_iteration.goal}`);
  }

  // Stats
  lines.push(`\n## 任务概览`);
  lines.push(`- 总计: ${ctx.stats.total} | 进行中: ${ctx.stats.in_progress} | 阻塞: ${ctx.stats.blocked} | 评审中: ${ctx.stats.review} | 已完成: ${ctx.stats.completed}`);

  // My tasks
  if (ctx.my_tasks?.length > 0) {
    lines.push(`\n## 我的任务 (${agentId})`);
    for (const t of ctx.my_tasks.slice(0, 5)) {
      lines.push(formatTask(t));
    }
  }

  // Pinned memories
  if (ctx.pinned_memories?.length > 0) {
    lines.push(`\n## 📌 置顶决策`);
    for (const m of ctx.pinned_memories.slice(0, 5)) {
      lines.push(formatMemory(m));
    }
  }

  // Team
  if (ctx.team_agents?.length > 0) {
    lines.push(`\n## 团队成员`);
    for (const a of ctx.team_agents) {
      lines.push(formatAgent(a));
    }
  }

  // Experiences
  if (ctx.experiences?.length > 0) {
    lines.push(`\n## 💡 经验提醒`);
    for (const e of ctx.experiences.slice(0, 3)) {
      lines.push(formatExperience(e));
    }
  }

  lines.push("\n=== 协作上下文结束 ===");
  return lines.join("\n");
}

function formatTask(t: Task): string {
  const deps = t.depends_on?.length > 0 ? ` (依赖: ${t.depends_on.join(",")})` : "";
  return `- [${t.status}] ${t.priority} ${t.id}: ${t.title}${deps}`;
}

function formatMemory(m: ProjectMemory): string {
  return `- [${m.category}] ${m.content.slice(0, 200)}`;
}

function formatAgent(a: AgentRecord): string {
  return `- ${a.emoji} ${a.display_name} (${a.role}) [${a.status}]${a.is_subagent ? " [子Agent]" : ""}`;
}

function formatExperience(e: AgentExperience): string {
  return `- [${e.category}] ${e.content.slice(0, 150)}`;
}
