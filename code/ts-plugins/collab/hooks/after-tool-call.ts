import type { GoBridge } from "../go-bridge.js";
import type { AfterToolCallEvent } from "../types.js";

const MONITORED_TOOLS = new Set([
  "sessions_spawn",
  "sessions_send",
  "create_task",
  "update_task",
]);

export function createAfterToolCallHook(bridge: GoBridge) {
  return async (event: AfterToolCallEvent): Promise<void> => {
    const { agentId, toolName, params, result, error } = event;

    if (!MONITORED_TOOLS.has(toolName)) return;

    if (error) {
      try {
        await bridge.addTaskActivity(
          params.taskId || params.task_id || "unknown",
          "tool_error",
          agentId,
          `Tool ${toolName} failed: ${error}`
        );

        // Record as experience for self-evolution
        await bridge.addExperience(agentId, {
          category: "pitfall",
          content: `Tool "${toolName}" with params ${JSON.stringify(params).slice(0, 200)} failed: ${error}`,
          tags: ["tool_error", toolName],
        });
      } catch {
        // Non-blocking
      }
      return;
    }

    switch (toolName) {
      case "sessions_spawn":
        await handleSpawnResult(bridge, agentId, params, result);
        break;
      case "sessions_send":
        await handleSendResult(bridge, agentId, params, result);
        break;
      case "create_task":
      case "update_task":
        await handleTaskToolResult(bridge, agentId, toolName, params, result);
        break;
    }
  };
}

async function handleSpawnResult(
  bridge: GoBridge,
  agentId: string,
  params: Record<string, any>,
  result: string
): Promise<void> {
  try {
    const targetAgentId = params.agentId || params.agent_id;
    // Register spawned subagent in registry
    await bridge.registerAgent({
      id: `${targetAgentId}:sub:${Date.now()}`,
      display_name: `${targetAgentId} (sub)`,
      role: "subagent",
      is_subagent: true,
      parent_agent: agentId,
      status: "busy",
      skills: [],
    });
  } catch {
    // Non-blocking
  }
}

async function handleSendResult(
  bridge: GoBridge,
  agentId: string,
  params: Record<string, any>,
  result: string
): Promise<void> {
  try {
    await bridge.addTaskActivity(
      params.taskId || "unknown",
      "agent_message",
      agentId,
      `A2A reply received: ${result.slice(0, 200)}`
    );
  } catch {
    // Non-blocking
  }
}

async function handleTaskToolResult(
  bridge: GoBridge,
  agentId: string,
  toolName: string,
  params: Record<string, any>,
  result: string
): Promise<void> {
  try {
    let parsed: any;
    try {
      parsed = JSON.parse(result);
    } catch {
      return;
    }
    const taskId = parsed.id || params.task_id || params.id;
    if (taskId) {
      await bridge.addTaskActivity(taskId, "comment", agentId, `${toolName} completed`);
    }
  } catch {
    // Non-blocking
  }
}
