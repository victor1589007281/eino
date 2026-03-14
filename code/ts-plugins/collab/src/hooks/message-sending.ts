import type { GoBridge } from "../go-bridge.js";
import type { MessageSendingEvent, MessageSendingResult } from "../types.js";

const AGENT_HEADER_PATTERN = /^【.*?】/;

export function createMessageSendingHook(bridge: GoBridge) {
  return async (event: MessageSendingEvent): Promise<MessageSendingResult> => {
    const { agentId, sessionKey, content, channelId } = event;

    // Skip empty or already-formatted messages
    if (!content || AGENT_HEADER_PATTERN.test(content)) {
      return {};
    }

    const isSubagent = sessionKey.includes(":subagent:");
    const parentAgentId = isSubagent ? extractParentAgentId(sessionKey) : undefined;
    const subagentName = isSubagent ? extractSubagentName(sessionKey) : undefined;

    // Build role header
    let header = "";
    try {
      const agents = await bridge.listAgents();
      const agent = agents.find((a) => a.id === agentId);
      if (agent) {
        if (isSubagent && parentAgentId) {
          const parent = agents.find((a) => a.id === parentAgentId);
          const parentDisplay = parent ? `${parent.emoji} ${parent.display_name}` : parentAgentId;
          header = `【${parentDisplay} → Sub:${subagentName || agentId}】`;
        } else {
          header = `【${agent.emoji} ${agent.display_name}】`;
        }
      }
    } catch {
      header = `【${agentId}】`;
    }

    // Format the message
    const formatted = header ? `${header}\n${content}` : content;

    // If this is a group chat, also send via Feishu relay for richer formatting
    if (channelId?.startsWith("oc_")) {
      try {
        await bridge.feishuSend({
          agent_id: agentId,
          channel_id: channelId,
          content: formatted,
          is_subagent: isSubagent,
          parent_agent_id: parentAgentId,
          subagent_name: subagentName,
        });
      } catch (err) {
        // Non-blocking; OpenClaw will still send the message natively
      }
    }

    return { content: formatted };
  };
}

function extractParentAgentId(sessionKey: string): string | undefined {
  // Format: "agent:{parentId}:subagent:{uuid}"
  const match = sessionKey.match(/agent:([^:]+):subagent:/);
  return match?.[1];
}

function extractSubagentName(sessionKey: string): string | undefined {
  const match = sessionKey.match(/:subagent:(.+)$/);
  return match?.[1]?.slice(0, 8); // Short UUID
}
