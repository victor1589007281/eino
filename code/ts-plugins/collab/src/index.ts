import { GoBridge } from "./go-bridge.js";
import { createBeforeAgentStartHook } from "./hooks/before-agent-start.js";
import { createMessageSendingHook } from "./hooks/message-sending.js";
import { createBeforeToolCallHook } from "./hooks/before-tool-call.js";
import { createAfterToolCallHook } from "./hooks/after-tool-call.js";
import { createAgentEndHook } from "./hooks/agent-end.js";
import { createBeforeCompactionHook } from "./hooks/before-compaction.js";
import { createTaskTool, createUpdateTaskTool, createQueryTasksTool, createSaveArtifactTool } from "./tools/task-tools.js";
import { createProjectTool, createSaveDecisionTool, createQueryProjectMemoryTool } from "./tools/project-tools.js";
import { createStatusCommand, createPauseCommand, createTasksCommand } from "./commands/status-command.js";
import type { PluginApi } from "./types.js";

interface PluginDefinition {
  id: string;
  name: string;
  version: string;
  register(api: PluginApi): void;
}

const DEFAULT_GO_SERVICE_URL = "http://localhost:8090";

const plugin: PluginDefinition = {
  id: "@team/collab",
  name: "Multi-Agent Collaboration",
  version: "1.0.0",

  register(api: PluginApi) {
    const config = api.getConfig?.() || {};
    const goServiceUrl = (config as any).goServiceUrl || DEFAULT_GO_SERVICE_URL;
    const bridge = new GoBridge(goServiceUrl);

    // --- Lifecycle Hooks ---

    const contextMaxTokens = (config as any).contextMaxTokens || 3000;
    api.on("before_agent_start", createBeforeAgentStartHook(bridge, contextMaxTokens), { priority: 10 });
    api.on("message_sending", createMessageSendingHook(bridge), { priority: 10 });
    api.on("before_tool_call", createBeforeToolCallHook(bridge), { priority: 10 });
    api.on("after_tool_call", createAfterToolCallHook(bridge), { priority: 10 });
    api.on("agent_end", createAgentEndHook(bridge), { priority: 10 });
    api.on("before_compaction", createBeforeCompactionHook(bridge), { priority: 10 });

    // --- Tools: Task Management ---

    api.registerTool(createTaskTool(bridge), { name: "create_task" });
    api.registerTool(createUpdateTaskTool(bridge), { name: "update_task" });
    api.registerTool(createQueryTasksTool(bridge), { name: "query_tasks" });
    api.registerTool(createSaveArtifactTool(bridge), { name: "save_artifact" });

    // --- Tools: Project Management ---

    api.registerTool(createProjectTool(bridge), { name: "create_project" });
    api.registerTool(createSaveDecisionTool(bridge), { name: "save_decision" });
    api.registerTool(createQueryProjectMemoryTool(bridge), { name: "query_project_memory" });

    // --- Commands ---

    api.registerCommand(createStatusCommand(bridge));
    api.registerCommand(createPauseCommand(bridge));
    api.registerCommand(createTasksCommand(bridge));

    // --- Background Service: Go Bridge Health Check ---

    api.registerService({
      id: "collab-bridge",
      async start(ctx) {
        ctx.logger.info("[collab] Starting Go bridge health monitor");
        const check = async () => {
          const ok = await bridge.health();
          if (!ok) {
            ctx.logger.warn("[collab] Go service unreachable at " + goServiceUrl);
          }
        };
        await check();
        setInterval(check, 60_000);
      },
    });
  },
};

export default plugin;
