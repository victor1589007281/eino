import type { GoBridge } from "../go-bridge.js";
import type { CommandContext } from "../types.js";

export function createStatusCommand(bridge: GoBridge) {
  return {
    name: "status",
    description: "Show project and task status overview. Usage: /status [project_id]",
    async handler(args: string[], context: CommandContext): Promise<string> {
      const projectId = args[0];
      if (!projectId) {
        // Show dashboard overview
        try {
          const dashboard = await bridge.getDashboard();
          const lines: string[] = ["📊 **全局状态概览**\n"];

          if (dashboard.projects?.length > 0) {
            lines.push("**项目**");
            for (const p of dashboard.projects) {
              lines.push(
                `- ${p.name} [${p.status}] | 任务: ${p.stats.total} (进行中: ${p.stats.in_progress}, 阻塞: ${p.stats.blocked})`
              );
            }
          }

          if (dashboard.agents?.length > 0) {
            lines.push("\n**Agent**");
            for (const a of dashboard.agents) {
              lines.push(
                `- ${a.emoji} ${a.display_name} [${a.status}] 负载: ${a.current_load}`
              );
            }
          }

          if (dashboard.alerts?.length > 0) {
            lines.push("\n**⚠️ 告警**");
            for (const alert of dashboard.alerts.slice(0, 5)) {
              lines.push(`- [${alert.severity}] ${alert.agent}: ${alert.message}`);
            }
          }

          return lines.join("\n");
        } catch (err: any) {
          return `❌ Failed to get dashboard: ${err.message}`;
        }
      }

      // Project-specific status
      try {
        const ctx = await bridge.getContext(projectId, "");
        if (!ctx) return `❌ Project ${projectId} not found`;

        const lines: string[] = [];
        lines.push(`📋 **${ctx.project.name}** [${ctx.project.status}]\n`);

        if (ctx.current_iteration) {
          lines.push(`**迭代**: ${ctx.current_iteration.name} - ${ctx.current_iteration.goal}`);
        }

        lines.push(`\n**任务统计**: 总计 ${ctx.stats.total} | 进行中 ${ctx.stats.in_progress} | 阻塞 ${ctx.stats.blocked} | 评审 ${ctx.stats.review} | 完成 ${ctx.stats.completed}`);

        if (ctx.my_tasks?.length > 0) {
          lines.push("\n**进行中的任务**");
          for (const t of ctx.my_tasks) {
            lines.push(`- [${t.priority}] ${t.id}: ${t.title} → ${t.assignee} [${t.status}]`);
          }
        }

        return lines.join("\n");
      } catch (err: any) {
        return `❌ Error: ${err.message}`;
      }
    },
  };
}

export function createPauseCommand(bridge: GoBridge) {
  return {
    name: "pause",
    description: "Pause a task or all tasks in a project. Usage: /pause <task_id|project_id>",
    async handler(args: string[], _context: CommandContext): Promise<string> {
      const target = args[0];
      if (!target) return "Usage: /pause <task_id|project_id>";

      try {
        if (target.startsWith("T")) {
          await bridge.updateTask(target, { status: "paused" });
          return `⏸️ Task ${target} paused`;
        } else if (target.startsWith("P")) {
          const tasks = await bridge.queryTasks({ project_id: target, status: "in_progress" });
          for (const t of tasks) {
            await bridge.updateTask(t.id, { status: "paused" });
          }
          return `⏸️ Paused ${tasks.length} tasks in project ${target}`;
        }
        return "Invalid target. Use task ID (Txxxx) or project ID (Pxxxx).";
      } catch (err: any) {
        return `❌ Error: ${err.message}`;
      }
    },
  };
}

export function createTasksCommand(bridge: GoBridge) {
  return {
    name: "tasks",
    description: "List tasks with filters. Usage: /tasks [project_id] [--status=xxx] [--assignee=xxx]",
    async handler(args: string[], _context: CommandContext): Promise<string> {
      const params: Record<string, string> = {};

      for (const arg of args) {
        if (arg.startsWith("--status=")) {
          params.status = arg.replace("--status=", "");
        } else if (arg.startsWith("--assignee=")) {
          params.assignee = arg.replace("--assignee=", "");
        } else if (!arg.startsWith("--")) {
          params.project_id = arg;
        }
      }

      try {
        const tasks = await bridge.queryTasks(params);
        if (tasks.length === 0) return "No tasks found matching criteria.";

        const lines: string[] = [`📋 **Tasks** (${tasks.length})\n`];
        for (const t of tasks.slice(0, 15)) {
          const deps = t.depends_on?.length > 0 ? ` ← ${t.depends_on.join(",")}` : "";
          lines.push(
            `- [${t.status}] ${t.priority} **${t.id}**: ${t.title} → ${t.assignee}${deps}`
          );
        }
        if (tasks.length > 15) {
          lines.push(`\n... and ${tasks.length - 15} more`);
        }
        return lines.join("\n");
      } catch (err: any) {
        return `❌ Error: ${err.message}`;
      }
    },
  };
}
