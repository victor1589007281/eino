import { GoBridge } from "../src/go-bridge";
import { createStatusCommand, createPauseCommand, createTasksCommand } from "../src/commands/status-command";

class MockBridge {
  calls: { method: string; args: any[] }[] = [];

  async getDashboard() {
    return {
      projects: [
        { id: "P1", name: "项目A", status: "active", stats: { total: 10, in_progress: 3, blocked: 1, completed: 5, review: 1 } },
      ],
      agents: [
        { id: "dev-1", display_name: "开发者", emoji: "👨‍💻", status: "busy", current_load: 2, active_projects: ["P1"] },
      ],
      alerts: [
        { severity: "warning", task_id: "T5", agent: "dev-1", message: "Stuck for 30min", timestamp: Date.now() / 1000 },
      ],
    };
  }

  async getContext(projectId: string, agentId: string) {
    return {
      project: { id: projectId, name: "项目A", description: "", group_id: "oc_test", status: "active", team_agents: [], tech_stack: [] },
      current_iteration: { id: "I1", project_id: projectId, name: "Sprint 1", goal: "Core", status: "active" },
      pinned_memories: [],
      my_tasks: [
        { id: "T1", project_id: projectId, title: "Build API", assignee: "dev-1", status: "in_progress", priority: "P1", depends_on: [], artifacts: [], topology: "free", deliverable: "", acceptance: "", description: "" },
      ],
      team_agents: [],
      stats: { total: 10, in_progress: 3, blocked: 1, completed: 5, review: 1 },
      experiences: [],
    };
  }

  async updateTask(taskId: string, fields: any) {
    this.calls.push({ method: "updateTask", args: [taskId, fields] });
  }

  async queryTasks(params: any) {
    this.calls.push({ method: "queryTasks", args: [params] });
    return [
      { id: "T1", title: "Build API", assignee: "dev-1", status: "in_progress", priority: "P1", depends_on: [] },
      { id: "T2", title: "Write tests", assignee: "qa-1", status: "assigned", priority: "P2", depends_on: ["T1"] },
    ];
  }
}

const mockCtx = { agentId: "rd-mgr", channelId: "oc_test" };

describe("/status command", () => {
  it("should show global overview without args", async () => {
    const mock = new MockBridge();
    const cmd = createStatusCommand(mock as unknown as GoBridge);

    const result = await cmd.handler([], mockCtx);
    expect(result).toContain("全局状态概览");
    expect(result).toContain("项目A");
    expect(result).toContain("开发者");
    expect(result).toContain("告警");
  });

  it("should show project detail with project ID arg", async () => {
    const mock = new MockBridge();
    const cmd = createStatusCommand(mock as unknown as GoBridge);

    const result = await cmd.handler(["P1"], mockCtx);
    expect(result).toContain("项目A");
    expect(result).toContain("Sprint 1");
    expect(result).toContain("Build API");
  });
});

describe("/pause command", () => {
  it("should pause a single task", async () => {
    const mock = new MockBridge();
    const cmd = createPauseCommand(mock as unknown as GoBridge);

    const result = await cmd.handler(["T1"], mockCtx);
    expect(result).toContain("paused");
    expect(mock.calls.some((c) => c.method === "updateTask" && c.args[0] === "T1")).toBe(true);
  });

  it("should pause all tasks in a project", async () => {
    const mock = new MockBridge();
    const cmd = createPauseCommand(mock as unknown as GoBridge);

    const result = await cmd.handler(["P1"], mockCtx);
    expect(result).toContain("Paused");
  });

  it("should show usage with no args", async () => {
    const mock = new MockBridge();
    const cmd = createPauseCommand(mock as unknown as GoBridge);

    const result = await cmd.handler([], mockCtx);
    expect(result).toContain("Usage");
  });
});

describe("/tasks command", () => {
  it("should list tasks with filters", async () => {
    const mock = new MockBridge();
    const cmd = createTasksCommand(mock as unknown as GoBridge);

    const result = await cmd.handler(["P1", "--status=in_progress"], mockCtx);
    expect(result).toContain("Tasks");
    expect(result).toContain("Build API");
  });

  it("should show all tasks with no filters", async () => {
    const mock = new MockBridge();
    const cmd = createTasksCommand(mock as unknown as GoBridge);

    const result = await cmd.handler([], mockCtx);
    expect(result).toContain("Tasks");
  });
});
