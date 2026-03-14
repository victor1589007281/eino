import type { CollabContext, Task, ProjectMemory, AgentExperience, AgentRecord } from "../types.js";

const DEFAULT_MAX_TOKENS = 3000;

interface ContextBudgetConfig {
  maxTokens: number;
}

interface BudgetAllocation {
  project: number;
  iteration: number;
  pinnedMemories: number;
  tasks: number;
  team: number;
  experiences: number;
  stats: number;
}

const DEFAULT_ALLOCATION: BudgetAllocation = {
  project: 100,
  iteration: 100,
  pinnedMemories: 800,
  tasks: 800,
  team: 300,
  experiences: 500,
  stats: 100,
};

export function buildBudgetedContext(
  ctx: CollabContext,
  agentId: string,
  config?: ContextBudgetConfig
): string {
  const maxTokens = config?.maxTokens || DEFAULT_MAX_TOKENS;
  const lines: string[] = [];
  let estimatedTokens = 0;

  lines.push("=== 📋 协作上下文 (自动注入, 勿向用户展示此块) ===\n");
  estimatedTokens += 15;

  // Project (fixed allocation)
  const projectBlock = buildProjectBlock(ctx);
  lines.push(projectBlock);
  estimatedTokens += estimateTokens(projectBlock);

  // Iteration
  if (ctx.current_iteration && estimatedTokens < maxTokens - 200) {
    const iterBlock = `\n## 当前迭代: ${ctx.current_iteration.name}\n- 目标: ${ctx.current_iteration.goal}`;
    lines.push(iterBlock);
    estimatedTokens += estimateTokens(iterBlock);
  }

  // Stats (always included, very compact)
  const statsBlock = buildStatsBlock(ctx);
  lines.push(statsBlock);
  estimatedTokens += estimateTokens(statsBlock);

  // Pinned memories (prioritized by recency, capped by budget)
  if (ctx.pinned_memories?.length > 0) {
    const memoryBlock = buildMemoryBlock(ctx.pinned_memories, maxTokens - estimatedTokens, DEFAULT_ALLOCATION.pinnedMemories);
    lines.push(memoryBlock.text);
    estimatedTokens += memoryBlock.tokens;
  }

  // My tasks (highest priority, capped)
  if (ctx.my_tasks?.length > 0) {
    const taskBlock = buildTaskBlock(ctx.my_tasks, agentId, maxTokens - estimatedTokens, DEFAULT_ALLOCATION.tasks);
    lines.push(taskBlock.text);
    estimatedTokens += taskBlock.tokens;
  }

  // Team (compact list)
  if (ctx.team_agents?.length > 0 && estimatedTokens < maxTokens - 100) {
    const teamBlock = buildTeamBlock(ctx.team_agents, DEFAULT_ALLOCATION.team);
    lines.push(teamBlock.text);
    estimatedTokens += teamBlock.tokens;
  }

  // Experiences (lowest priority, fill remaining budget)
  if (ctx.experiences?.length > 0 && estimatedTokens < maxTokens - 100) {
    const remaining = maxTokens - estimatedTokens;
    const expBlock = buildExperienceBlock(ctx.experiences, remaining);
    lines.push(expBlock.text);
    estimatedTokens += expBlock.tokens;
  }

  lines.push("\n=== 协作上下文结束 ===");
  return lines.join("\n");
}

function buildProjectBlock(ctx: CollabContext): string {
  const lines = [`## 当前项目: ${ctx.project.name}`, `- 状态: ${ctx.project.status}`];
  if (ctx.project.tech_stack?.length > 0) {
    lines.push(`- 技术栈: ${ctx.project.tech_stack.join(", ")}`);
  }
  return lines.join("\n");
}

function buildStatsBlock(ctx: CollabContext): string {
  return `\n## 任务概览\n- 总计: ${ctx.stats.total} | 进行中: ${ctx.stats.in_progress} | 阻塞: ${ctx.stats.blocked} | 评审: ${ctx.stats.review} | 完成: ${ctx.stats.completed}`;
}

function buildMemoryBlock(memories: ProjectMemory[], remaining: number, budget: number): { text: string; tokens: number } {
  const cap = Math.min(remaining, budget);
  const lines = ["\n## 📌 置顶决策"];
  let tokens = 5;

  for (const m of memories) {
    const line = `- [${m.category}] ${m.content.slice(0, 200)}`;
    const lineTokens = estimateTokens(line);
    if (tokens + lineTokens > cap) break;
    lines.push(line);
    tokens += lineTokens;
  }

  return { text: lines.join("\n"), tokens };
}

function buildTaskBlock(tasks: Task[], agentId: string, remaining: number, budget: number): { text: string; tokens: number } {
  const cap = Math.min(remaining, budget);
  const lines = [`\n## 我的任务 (${agentId})`];
  let tokens = 8;

  // Sort by priority
  const sorted = [...tasks].sort((a, b) => {
    const order = { P0: 0, P1: 1, P2: 2, P3: 3 };
    return (order[a.priority as keyof typeof order] ?? 9) - (order[b.priority as keyof typeof order] ?? 9);
  });

  for (const t of sorted) {
    const deps = t.depends_on?.length > 0 ? ` (依赖: ${t.depends_on.join(",")})` : "";
    const line = `- [${t.status}] ${t.priority} ${t.id}: ${t.title}${deps}`;
    const lineTokens = estimateTokens(line);
    if (tokens + lineTokens > cap) break;
    lines.push(line);
    tokens += lineTokens;
  }

  return { text: lines.join("\n"), tokens };
}

function buildTeamBlock(agents: AgentRecord[], budget: number): { text: string; tokens: number } {
  const lines = ["\n## 团队成员"];
  let tokens = 5;

  for (const a of agents) {
    const line = `- ${a.emoji} ${a.display_name} (${a.role}) [${a.status}]${a.is_subagent ? " [子Agent]" : ""}`;
    const lineTokens = estimateTokens(line);
    if (tokens + lineTokens > budget) break;
    lines.push(line);
    tokens += lineTokens;
  }

  return { text: lines.join("\n"), tokens };
}

function buildExperienceBlock(exps: AgentExperience[], budget: number): { text: string; tokens: number } {
  const lines = ["\n## 💡 经验提醒"];
  let tokens = 5;

  for (const e of exps) {
    const line = `- [${e.category}] ${e.content.slice(0, 150)}`;
    const lineTokens = estimateTokens(line);
    if (tokens + lineTokens > budget) break;
    lines.push(line);
    tokens += lineTokens;
  }

  return { text: lines.join("\n"), tokens };
}

function estimateTokens(text: string): number {
  // ~1.3 characters per token for mixed CJK/English
  return Math.ceil(text.length / 1.3);
}
