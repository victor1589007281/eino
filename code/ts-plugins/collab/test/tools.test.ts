import { GoBridge } from "../src/go-bridge";
import { createTaskTool, createUpdateTaskTool, createQueryTasksTool, createSaveArtifactTool } from "../src/tools/task-tools";
import { createProjectTool, createSaveDecisionTool, createQueryProjectMemoryTool } from "../src/tools/project-tools";

class MockBridge {
  calls: { method: string; args: any[] }[] = [];

  async createTask(task: any) {
    this.calls.push({ method: "createTask", args: [task] });
    return { id: "T-mock-1", ...task };
  }

  async updateTask(taskId: string, fields: any) {
    this.calls.push({ method: "updateTask", args: [taskId, fields] });
  }

  async queryTasks(params: any) {
    this.calls.push({ method: "queryTasks", args: [params] });
    return [
      { id: "T1", title: "Mock Task", assignee: "dev-1", status: "in_progress", priority: "P1", depends_on: [], artifacts: [] },
    ];
  }

  async saveArtifact(taskId: string, artifact: any) {
    this.calls.push({ method: "saveArtifact", args: [taskId, artifact] });
  }

  async createProject(project: any) {
    this.calls.push({ method: "createProject", args: [project] });
    return { id: "P-mock-1", ...project };
  }

  async saveDecision(projectId: string, memory: any) {
    this.calls.push({ method: "saveDecision", args: [projectId, memory] });
  }

  async queryMemory(projectId: string, query: string, category?: string) {
    this.calls.push({ method: "queryMemory", args: [projectId, query, category] });
    return [{ id: "M1", content: "Mock memory", category: "decision" }];
  }
}

const mockContext = { agentId: "dev-1", sessionKey: "agent:dev-1" };

describe("Task Tools", () => {
  describe("create_task", () => {
    it("should create a task with required fields", async () => {
      const mock = new MockBridge();
      const tool = createTaskTool(mock as unknown as GoBridge);

      const result = await tool.execute(
        { project_id: "P1", title: "Implement auth", assignee: "dev-1", priority: "P1" },
        mockContext
      );
      const parsed = JSON.parse(result);

      expect(parsed.success).toBe(true);
      expect(parsed.task_id).toBe("T-mock-1");
      expect(mock.calls[0].args[0].title).toBe("Implement auth");
    });

    it("should handle creation errors", async () => {
      const mock = new MockBridge();
      mock.createTask = async () => { throw new Error("DB error"); };
      const tool = createTaskTool(mock as unknown as GoBridge);

      const result = await tool.execute(
        { project_id: "P1", title: "Fail", assignee: "dev-1" },
        mockContext
      );
      const parsed = JSON.parse(result);
      expect(parsed.success).toBe(false);
      expect(parsed.error).toContain("DB error");
    });
  });

  describe("update_task", () => {
    it("should update task status", async () => {
      const mock = new MockBridge();
      const tool = createUpdateTaskTool(mock as unknown as GoBridge);

      const result = await tool.execute(
        { task_id: "T1", status: "completed", result: "Done" },
        mockContext
      );
      const parsed = JSON.parse(result);
      expect(parsed.success).toBe(true);
      expect(mock.calls[0].args[1].status).toBe("completed");
    });
  });

  describe("query_tasks", () => {
    it("should query tasks with filters", async () => {
      const mock = new MockBridge();
      const tool = createQueryTasksTool(mock as unknown as GoBridge);

      const result = await tool.execute(
        { project_id: "P1", status: "in_progress" },
        mockContext
      );
      const parsed = JSON.parse(result);
      expect(parsed.success).toBe(true);
      expect(parsed.count).toBe(1);
    });
  });

  describe("save_artifact", () => {
    it("should save an artifact", async () => {
      const mock = new MockBridge();
      const tool = createSaveArtifactTool(mock as unknown as GoBridge);

      const result = await tool.execute(
        { task_id: "T1", name: "design.md", type: "document", content: "# Design" },
        mockContext
      );
      const parsed = JSON.parse(result);
      expect(parsed.success).toBe(true);
    });
  });
});

describe("Project Tools", () => {
  describe("create_project", () => {
    it("should create a project", async () => {
      const mock = new MockBridge();
      const tool = createProjectTool(mock as unknown as GoBridge);

      const result = await tool.execute(
        { name: "New Project", group_id: "oc_new", team_agents: ["dev-1"], tech_stack: ["Go"] },
        mockContext
      );
      const parsed = JSON.parse(result);
      expect(parsed.success).toBe(true);
      expect(parsed.project_id).toBe("P-mock-1");
    });
  });

  describe("save_decision", () => {
    it("should save a project decision", async () => {
      const mock = new MockBridge();
      const tool = createSaveDecisionTool(mock as unknown as GoBridge);

      const result = await tool.execute(
        { project_id: "P1", category: "tech_choice", content: "Use PostgreSQL", pinned: true },
        mockContext
      );
      const parsed = JSON.parse(result);
      expect(parsed.success).toBe(true);
    });
  });

  describe("query_project_memory", () => {
    it("should query project memories", async () => {
      const mock = new MockBridge();
      const tool = createQueryProjectMemoryTool(mock as unknown as GoBridge);

      const result = await tool.execute(
        { project_id: "P1", query: "PostgreSQL" },
        mockContext
      );
      const parsed = JSON.parse(result);
      expect(parsed.success).toBe(true);
      expect(parsed.count).toBe(1);
    });
  });
});
