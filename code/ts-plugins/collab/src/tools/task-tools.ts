import type { GoBridge } from "../go-bridge.js";
import type { ToolDefinition, ToolContext } from "../types.js";

export function createTaskTool(bridge: GoBridge): ToolDefinition {
  return {
    description: "Create a new task with title, description, assignee, priority, and dependencies.",
    parameters: {
      type: "object",
      properties: {
        project_id: { type: "string", description: "Project ID" },
        title: { type: "string", description: "Task title" },
        description: { type: "string", description: "Detailed description" },
        assignee: { type: "string", description: "Agent ID to assign" },
        priority: { type: "string", enum: ["P0", "P1", "P2", "P3"], description: "Priority level" },
        parent_id: { type: "string", description: "Parent task ID for subtasks" },
        depends_on: { type: "array", items: { type: "string" }, description: "IDs of dependent tasks" },
        topology: { type: "string", enum: ["pipeline", "fanout", "fanin", "review", "free"], description: "Execution pattern" },
        deliverable: { type: "string", description: "Expected deliverable" },
        acceptance: { type: "string", description: "Acceptance criteria" },
      },
      required: ["project_id", "title", "assignee"],
    },
    async execute(params: Record<string, any>, context: ToolContext): Promise<string> {
      try {
        const task = await bridge.createTask({
          project_id: params.project_id,
          title: params.title,
          description: params.description || "",
          assignee: params.assignee,
          assigned_by: context.agentId,
          priority: params.priority || "P2",
          parent_id: params.parent_id,
          depends_on: params.depends_on || [],
          topology: params.topology || "free",
          deliverable: params.deliverable || "",
          acceptance: params.acceptance || "",
          status: "assigned",
        });
        return JSON.stringify({ success: true, task_id: task.id, message: `Task ${task.id} created and assigned to ${params.assignee}` });
      } catch (err: any) {
        return JSON.stringify({ success: false, error: err.message });
      }
    },
  };
}

export function createUpdateTaskTool(bridge: GoBridge): ToolDefinition {
  return {
    description: "Update an existing task status, result, or other fields.",
    parameters: {
      type: "object",
      properties: {
        task_id: { type: "string", description: "Task ID to update" },
        status: { type: "string", enum: ["in_progress", "blocked", "review", "completed", "error", "paused"], description: "New status" },
        result: { type: "string", description: "Task result or output" },
        block_reason: { type: "string", description: "Reason for blocking" },
        error_info: { type: "string", description: "Error information" },
      },
      required: ["task_id"],
    },
    async execute(params: Record<string, any>, context: ToolContext): Promise<string> {
      try {
        const fields: Record<string, any> = {};
        if (params.result) fields.result = params.result;
        if (params.block_reason) fields.block_reason = params.block_reason;
        if (params.error_info) fields.error_info = params.error_info;
        if (params.status) fields.status = params.status;

        await bridge.updateTask(params.task_id, fields);
        return JSON.stringify({ success: true, message: `Task ${params.task_id} updated` });
      } catch (err: any) {
        return JSON.stringify({ success: false, error: err.message });
      }
    },
  };
}

export function createQueryTasksTool(bridge: GoBridge): ToolDefinition {
  return {
    description: "Query tasks by project, assignee, status, or parent task.",
    parameters: {
      type: "object",
      properties: {
        project_id: { type: "string", description: "Filter by project" },
        assignee: { type: "string", description: "Filter by assignee agent ID" },
        status: { type: "string", description: "Filter by status" },
        parent_task_id: { type: "string", description: "Filter by parent task" },
        include_subtree: { type: "boolean", description: "Include full subtree" },
      },
    },
    async execute(params: Record<string, any>): Promise<string> {
      try {
        const queryParams: Record<string, string> = {};
        if (params.project_id) queryParams.project_id = params.project_id;
        if (params.assignee) queryParams.assignee = params.assignee;
        if (params.status) queryParams.status = params.status;
        if (params.parent_task_id) queryParams.parent_task_id = params.parent_task_id;
        if (params.include_subtree) queryParams.include_subtree = "true";

        const tasks = await bridge.queryTasks(queryParams);
        return JSON.stringify({ success: true, count: tasks.length, tasks: tasks.slice(0, 20) });
      } catch (err: any) {
        return JSON.stringify({ success: false, error: err.message });
      }
    },
  };
}

export function createSaveArtifactTool(bridge: GoBridge): ToolDefinition {
  return {
    description: "Save a task artifact (document, code, config, test report).",
    parameters: {
      type: "object",
      properties: {
        task_id: { type: "string", description: "Associated task ID" },
        project_id: { type: "string", description: "Project ID" },
        name: { type: "string", description: "Artifact name" },
        type: { type: "string", enum: ["document", "code", "config", "test_report"], description: "Artifact type" },
        content: { type: "string", description: "Artifact content" },
      },
      required: ["task_id", "name", "type", "content"],
    },
    async execute(params: Record<string, any>, context: ToolContext): Promise<string> {
      try {
        await bridge.saveArtifact(params.task_id, {
          project_id: params.project_id || "",
          name: params.name,
          type: params.type,
          content: params.content,
          created_by: context.agentId,
        });
        return JSON.stringify({ success: true, message: `Artifact "${params.name}" saved` });
      } catch (err: any) {
        return JSON.stringify({ success: false, error: err.message });
      }
    },
  };
}
