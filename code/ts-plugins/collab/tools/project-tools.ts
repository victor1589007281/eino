import type { GoBridge } from "../go-bridge.js";
import type { ToolDefinition, ToolContext } from "../types.js";

export function createProjectTool(bridge: GoBridge): ToolDefinition {
  return {
    description: "Create a new project bound to a Feishu group.",
    parameters: {
      type: "object",
      properties: {
        name: { type: "string", description: "Project name" },
        description: { type: "string", description: "Project description" },
        group_id: { type: "string", description: "Feishu group chat ID (oc_xxx)" },
        team_agents: { type: "array", items: { type: "string" }, description: "Agent IDs for the team" },
        tech_stack: { type: "array", items: { type: "string" }, description: "Tech stack tags" },
      },
      required: ["name", "group_id"],
    },
    async execute(params: Record<string, any>): Promise<string> {
      try {
        const project = await bridge.createProject({
          name: params.name,
          description: params.description || "",
          group_id: params.group_id,
          team_agents: params.team_agents || [],
          tech_stack: params.tech_stack || [],
        });
        return JSON.stringify({ success: true, project_id: project.id, message: `Project "${params.name}" created` });
      } catch (err: any) {
        return JSON.stringify({ success: false, error: err.message });
      }
    },
  };
}

export function createSaveDecisionTool(bridge: GoBridge): ToolDefinition {
  return {
    description: "Save an important project decision, technical choice, lesson learned, or user directive to structured memory.",
    parameters: {
      type: "object",
      properties: {
        project_id: { type: "string", description: "Project ID" },
        category: {
          type: "string",
          enum: ["decision", "tech_choice", "lesson", "architecture", "user_directive"],
          description: "Memory category",
        },
        content: { type: "string", description: "Memory content" },
        pinned: { type: "boolean", description: "Pin to always include in context" },
      },
      required: ["project_id", "category", "content"],
    },
    async execute(params: Record<string, any>, context: ToolContext): Promise<string> {
      try {
        await bridge.saveDecision(params.project_id, {
          category: params.category,
          content: params.content,
          pinned: params.pinned || false,
          created_by: context.agentId,
        } as any);
        return JSON.stringify({ success: true, message: `${params.category} memory saved` });
      } catch (err: any) {
        return JSON.stringify({ success: false, error: err.message });
      }
    },
  };
}

export function createQueryProjectMemoryTool(bridge: GoBridge): ToolDefinition {
  return {
    description: "Search project memories by keyword or category.",
    parameters: {
      type: "object",
      properties: {
        project_id: { type: "string", description: "Project ID" },
        query: { type: "string", description: "Search keywords" },
        category: {
          type: "string",
          enum: ["decision", "tech_choice", "lesson", "architecture", "user_directive"],
          description: "Filter by category",
        },
      },
      required: ["project_id"],
    },
    async execute(params: Record<string, any>): Promise<string> {
      try {
        const memories = await bridge.queryMemory(
          params.project_id,
          params.query || "",
          params.category
        );
        return JSON.stringify({ success: true, count: memories.length, memories: memories.slice(0, 10) });
      } catch (err: any) {
        return JSON.stringify({ success: false, error: err.message });
      }
    },
  };
}
