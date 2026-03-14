import { GoBridge } from "../src/go-bridge";
import { createBeforeAgentStartHook } from "../src/hooks/before-agent-start";
import { createMessageSendingHook } from "../src/hooks/message-sending";
import { createBeforeToolCallHook } from "../src/hooks/before-tool-call";
import { createAfterToolCallHook } from "../src/hooks/after-tool-call";
import { createAgentEndHook } from "../src/hooks/agent-end";
import { createBeforeCompactionHook } from "../src/hooks/before-compaction";

// Mock bridge that records calls
class MockBridge {
  calls: { method: string; args: any[] }[] = [];

  async getContext(projectId: string, agentId: string) {
    this.calls.push({ method: "getContext", args: [projectId, agentId] });
    return {
      project: { id: "P1", name: "Test", description: "", group_id: "oc_test", status: "active", team_agents: ["dev-1"], tech_stack: ["Go"] },
      current_iteration: { id: "I1", project_id: "P1", name: "Sprint 1", goal: "Core", status: "active" },
      pinned_memories: [{ id: "M1", project_id: "P1", category: "decision", content: "Use Go", pinned: true }],
      my_tasks: [{ id: "T1", project_id: "P1", title: "Build API", assignee: "dev-1", status: "in_progress", priority: "P1", depends_on: [], artifacts: [], topology: "free", deliverable: "", acceptance: "", description: "" }],
      team_agents: [{ id: "dev-1", display_name: "开发者", emoji: "👨‍💻", role: "developer", skills: [], status: "busy", is_subagent: false, parent_agent: "" }],
      stats: { total: 5, in_progress: 2, blocked: 1, completed: 1, review: 1 },
      experiences: [],
    };
  }

  async updateAgentStatus(agentId: string, status: string) {
    this.calls.push({ method: "updateAgentStatus", args: [agentId, status] });
  }

  async updateAgentLoad(agentId: string, load: number) {
    this.calls.push({ method: "updateAgentLoad", args: [agentId, load] });
  }

  async listAgents() {
    this.calls.push({ method: "listAgents", args: [] });
    return [
      { id: "dev-1", display_name: "开发者", emoji: "👨‍💻", role: "developer", skills: [], status: "busy", is_subagent: false, parent_agent: "", current_load: 0 },
      { id: "arch-1", display_name: "架构师", emoji: "🏗️", role: "architect", skills: [], status: "idle", is_subagent: false, parent_agent: "", current_load: 0 },
    ];
  }

  async addTaskActivity(...args: any[]) {
    this.calls.push({ method: "addTaskActivity", args });
  }

  async registerAgent(...args: any[]) {
    this.calls.push({ method: "registerAgent", args });
  }

  async addExperience(...args: any[]) {
    this.calls.push({ method: "addExperience", args });
  }

  async queryTasks(params: any) {
    this.calls.push({ method: "queryTasks", args: [params] });
    return [{ id: "T1", title: "Test", assignee: "dev-1", status: "in_progress" }];
  }

  async syncMemories(...args: any[]) {
    this.calls.push({ method: "syncMemories", args });
  }

  async feishuSend(...args: any[]) {
    this.calls.push({ method: "feishuSend", args });
  }
}

describe("before_agent_start hook", () => {
  it("should inject context for group sessions", async () => {
    const mock = new MockBridge();
    const hook = createBeforeAgentStartHook(mock as unknown as GoBridge);

    const result = await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1:oc_test",
      messages: [],
    });

    expect(result.prependContext).toBeTruthy();
    expect(result.prependContext).toContain("协作上下文");
    expect(result.prependContext).toContain("Test");
    expect(mock.calls.some((c) => c.method === "updateAgentStatus")).toBe(true);
  });

  it("should return empty for DM sessions without project", async () => {
    const mock = new MockBridge();
    const hook = createBeforeAgentStartHook(mock as unknown as GoBridge);

    const result = await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1:dm:user123",
      messages: [],
    });

    expect(result.prependContext).toBeUndefined();
  });
});

describe("message_sending hook", () => {
  it("should prepend agent header", async () => {
    const mock = new MockBridge();
    const hook = createMessageSendingHook(mock as unknown as GoBridge);

    const result = await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1",
      content: "Task completed!",
    });

    expect(result.content).toContain("👨‍💻");
    expect(result.content).toContain("开发者");
    expect(result.content).toContain("Task completed!");
  });

  it("should not double-format already-formatted messages", async () => {
    const mock = new MockBridge();
    const hook = createMessageSendingHook(mock as unknown as GoBridge);

    const result = await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1",
      content: "【👨‍💻 开发者】Already formatted",
    });

    expect(result.content).toBeUndefined();
  });

  it("should handle subagent messages", async () => {
    const mock = new MockBridge();
    const hook = createMessageSendingHook(mock as unknown as GoBridge);

    const result = await hook({
      agentId: "dev-1",
      sessionKey: "agent:arch-1:subagent:abc123",
      content: "Sub result",
    });

    expect(result.content).toContain("Sub:");
  });
});

describe("before_tool_call hook", () => {
  it("should allow non-monitored tools to pass through", async () => {
    const mock = new MockBridge();
    const hook = createBeforeToolCallHook(mock as unknown as GoBridge);

    const result = await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1",
      toolName: "readFile",
      params: { path: "/tmp/test" },
    });

    expect(result.block).toBeUndefined();
    expect(mock.calls.length).toBe(0);
  });

  it("should track sessions_spawn calls", async () => {
    const mock = new MockBridge();
    const hook = createBeforeToolCallHook(mock as unknown as GoBridge);

    const result = await hook({
      agentId: "arch-1",
      sessionKey: "agent:arch-1",
      toolName: "sessions_spawn",
      params: { agentId: "dev-1", taskId: "T1" },
    });

    expect(result.block).toBeUndefined(); // Should allow
    expect(mock.calls.some((c) => c.method === "addTaskActivity")).toBe(true);
  });
});

describe("after_tool_call hook", () => {
  it("should record errors as experiences", async () => {
    const mock = new MockBridge();
    const hook = createAfterToolCallHook(mock as unknown as GoBridge);

    await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1",
      toolName: "sessions_spawn",
      params: { agentId: "qa-1" },
      result: "",
      error: "Agent not found",
    });

    expect(mock.calls.some((c) => c.method === "addExperience")).toBe(true);
  });
});

describe("agent_end hook", () => {
  it("should reset status on normal end", async () => {
    const mock = new MockBridge();
    const hook = createAgentEndHook(mock as unknown as GoBridge);

    await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1",
    });

    const statusCall = mock.calls.find(
      (c) => c.method === "updateAgentStatus" && c.args[1] === "idle"
    );
    expect(statusCall).toBeTruthy();
  });

  it("should set error status on error", async () => {
    const mock = new MockBridge();
    const hook = createAgentEndHook(mock as unknown as GoBridge);

    await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1",
      error: "Rate limit exceeded",
    });

    const statusCall = mock.calls.find(
      (c) => c.method === "updateAgentStatus" && c.args[1] === "error"
    );
    expect(statusCall).toBeTruthy();
  });
});

describe("before_compaction hook", () => {
  it("should sync task progress before compaction", async () => {
    const mock = new MockBridge();
    const hook = createBeforeCompactionHook(mock as unknown as GoBridge);

    await hook({
      agentId: "dev-1",
      sessionKey: "agent:dev-1:oc_test",
      messageCount: 100,
      totalTokens: 50000,
    });

    expect(mock.calls.some((c) => c.method === "queryTasks")).toBe(true);
    expect(mock.calls.some((c) => c.method === "addTaskActivity")).toBe(true);
  });
});
