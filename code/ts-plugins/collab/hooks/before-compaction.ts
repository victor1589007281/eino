import type { GoBridge } from "../go-bridge.js";
import type { BeforeCompactionEvent } from "../types.js";

export function createBeforeCompactionHook(bridge: GoBridge) {
  return async (event: BeforeCompactionEvent): Promise<void> => {
    const { agentId, sessionKey } = event;

    const projectId = extractProjectId(sessionKey);
    if (!projectId) return;

    try {
      // Sync current task statuses to Go service before memory compaction
      const tasks = await bridge.queryTasks({
        assignee: agentId,
        status: "in_progress",
      });

      for (const t of tasks) {
        await bridge.addTaskActivity(
          t.id,
          "compaction_sync",
          agentId,
          `Pre-compaction checkpoint. Session tokens about to be compacted. Task progress snapshot synced.`
        );
      }

      // Persist structured memories that might be lost in compaction
      const memoriesToSync = extractStructuredMemories(event);
      if (memoriesToSync.length > 0) {
        await bridge.syncMemories(projectId, agentId, memoriesToSync);
      }
    } catch {
      // Non-blocking; compaction must not be delayed
    }
  };
}

function extractProjectId(sessionKey: string): string | null {
  const parts = sessionKey.split(":");
  for (const part of parts) {
    if (part.startsWith("oc_")) return part;
  }
  return null;
}

interface MemoryCandidate {
  category: string;
  content: string;
  pinned: boolean;
}

function extractStructuredMemories(event: BeforeCompactionEvent): MemoryCandidate[] {
  // In a real implementation, this would parse the session transcript
  // to extract decisions, tech choices, and user directives.
  // For now, we return an empty array as a placeholder; the main
  // memory persistence happens through OpenClaw's native memoryFlush.
  return [];
}
