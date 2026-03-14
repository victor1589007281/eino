import type { GoBridge } from "../go-bridge.js";
import type { BeforeToolCallEvent, BeforeToolCallResult } from "../types.js";

const MONITORED_TOOLS = new Set([
  "sessions_spawn",
  "sessions_send",
  "agents_list",
]);

export function createBeforeToolCallHook(bridge: GoBridge) {
  return async (event: BeforeToolCallEvent): Promise<BeforeToolCallResult> => {
    const { agentId, sessionKey, toolName, params } = event;

    if (!MONITORED_TOOLS.has(toolName)) {
      return {};
    }

    switch (toolName) {
      case "sessions_spawn":
        return handleSpawn(bridge, agentId, sessionKey, params);
      case "sessions_send":
        return handleSend(bridge, agentId, sessionKey, params);
      case "agents_list":
        return handleAgentsList(bridge, agentId);
      default:
        return {};
    }
  };
}

async function handleSpawn(
  bridge: GoBridge,
  agentId: string,
  _sessionKey: string,
  params: Record<string, any>
): Promise<BeforeToolCallResult> {
  const targetAgentId = params.agentId || params.agent_id;
  if (!targetAgentId) return {};

  try {
    // Register sub-agent activity
    await bridge.addTaskActivity(
      params.taskId || "unknown",
      "subagent_spawn",
      agentId,
      `Spawning subagent: ${targetAgentId}`
    );

    // Update parent agent load
    const agents = await bridge.listAgents();
    const parent = agents.find((a) => a.id === agentId);
    if (parent) {
      await bridge.updateAgentLoad(agentId, (parent.current_load || 0) + 1);
    }
  } catch {
    // Non-blocking
  }

  return {}; // Allow the spawn to proceed
}

async function handleSend(
  bridge: GoBridge,
  agentId: string,
  _sessionKey: string,
  params: Record<string, any>
): Promise<BeforeToolCallResult> {
  const targetSessionKey = params.sessionKey || params.session_key;
  if (!targetSessionKey) return {};

  try {
    await bridge.addTaskActivity(
      params.taskId || "unknown",
      "agent_message",
      agentId,
      `A2A message to: ${targetSessionKey}`
    );
  } catch {
    // Non-blocking
  }

  return {};
}

async function handleAgentsList(
  bridge: GoBridge,
  agentId: string
): Promise<BeforeToolCallResult> {
  try {
    await bridge.updateAgentStatus(agentId, "busy");
  } catch {
    // Non-blocking
  }
  return {};
}
