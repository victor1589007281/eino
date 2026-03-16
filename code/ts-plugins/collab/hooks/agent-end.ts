import type { GoBridge } from "../go-bridge.js";
import type { AgentEndEvent } from "../types.js";

export function createAgentEndHook(bridge: GoBridge) {
  return async (event: AgentEndEvent): Promise<void> => {
    const { agentId, error, tokenUsage } = event;

    try {
      if (error) {
        // Report error to Go service
        await bridge.updateAgentStatus(agentId, "error");

        // Find the agent's in-progress tasks and record the error
        const tasks = await bridge.queryTasks({
          assignee: agentId,
          status: "in_progress",
        });
        for (const t of tasks) {
          await bridge.addTaskActivity(
            t.id,
            "tool_error",
            agentId,
            `Agent ended with error: ${error}`
          );
        }

        // Save as experience for self-evolution
        await bridge.addExperience(agentId, {
          category: "pitfall",
          content: `Agent crashed: ${error}. Token usage: ${JSON.stringify(tokenUsage || {})}`,
          tags: ["agent_crash", "error"],
        });
      } else {
        // Normal completion — reset status
        await bridge.updateAgentStatus(agentId, "idle");
        await bridge.updateAgentLoad(agentId, 0);
      }
    } catch {
      // Final fallback — nothing we can do
    }
  };
}
