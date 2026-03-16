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
  return async (event: BeforeAgentStartEvent, context: { channelId?: string }): Promise<BeforeAgentStartResult> => {
    const { agentId, sessionKey } = event || {};
    if (!agentId || !sessionKey) {
      return {};
    }
    
    // 从 context 中获取 channelId（飞书群聊 ID）
    const channelId = context?.channelId;
    if (!channelId || !channelId.startsWith("oc_")) {
      // 不是飞书群聊，不加载项目上下文
      return {};
    }
    
    // 通过群聊 ID 查询绑定的项目
    const projectId = await getProjectIdByGroup(bridge, channelId);
    if (!projectId) {
      // 群聊没有绑定项目
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

async function getProjectIdByGroup(bridge: GoBridge, groupId: string): Promise<string | null> {
  try {
    const result = await bridge.getProjectByGroup(groupId);
    if (result && result.project && result.project.id) {
      return result.project.id;
    }
  } catch (err) {
    // 查询失败或没有绑定项目
  }
  return null;
}

function buildContextBlock(ctx: CollabContext, agentId: string): string {
  const lines: string[] = [];
  lines.push("=== 📋 协作上下文 (自动注入, 勿向用户展示此块) ===\n");

  // Project info
  lines.push(`## 当前项目：${ctx.project.name}`);
  lines.push(`- 状态：${ctx.project.status}`);
  if (ctx.project.tech_stack?.length > 0) {
    lines.push(`- 技术栈：${ctx.project.tech_stack.join(", ")}`);
  }
  
  // 仓库信息和文档路径
  const docsBaseDir = ctx.project.docs_base_dir || "/tmp/docs";
  const docPaths = {
    designs: `${docsBaseDir}/${ctx.project.id}/designs`,
    research: `${docsBaseDir}/${ctx.project.id}/research`,
    system: `${docsBaseDir}/${ctx.project.id}/system`,
    reports: `${docsBaseDir}/${ctx.project.id}/reports`,
  };
  
  const mainRepo = ctx.project.repositories?.code_repos?.find(r => r.type === "main") 
                || ctx.project.repositories?.code_repos?.[0];
  
  lines.push(`\n## 📁 项目仓库与文档路径`);
  if (mainRepo) {
    lines.push(`- 主代码仓库：${mainRepo.name}`);
    lines.push(`  - 本地路径：${mainRepo.local_path}`);
    if (mainRepo.git_url) {
      lines.push(`  - Git 远程：${mainRepo.git_url}`);
    }
  }
  lines.push(`- 文档输出根目录：${docsBaseDir}`);
  lines.push(`\n### 💾 文档目录结构（绝对路径）`);
  lines.push(`- designs:  ${docPaths.designs}/   # 功能设计文档`);
  lines.push(`- research: ${docPaths.research}/  # 调研分析报告`);
  lines.push(`- system:   ${docPaths.system}/    # 模块实现文档`);
  lines.push(`- reports:  ${docPaths.reports}/   # AI 任务汇总报告`);
  
  // 使用指南
  lines.push(`\n### 📝 保存文件指南`);
  lines.push(`1. 调研报告 → ${docPaths.research}/<主题>.md`);
  lines.push(`2. 设计文档 → ${docPaths.designs}/<文档名>.md`);
  lines.push(`3. 模块文档 → ${docPaths.system}/<模块名>.md`);
  lines.push(`4. 汇总报告 → ${docPaths.reports}/<报告名>.md`);
  lines.push(`5. 源代码 → ${mainRepo ? mainRepo.local_path : '<未配置>'}/<模块>/<文件>.ts`);
  lines.push(`\n**重要**:`);
  lines.push(`- ✅ 所有路径都是绝对路径，直接使用`);
  lines.push(`- ✅ 使用 save_artifact 工具保存交付物到数据库`);
  lines.push(`- ✅ 使用 query_project_info 工具查询最新路径信息`);
  lines.push(`- ❌ 如果路径不存在，会收到错误提示，请引导用户创建或修正配置`);

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
