import { GoBridge } from "../src/go-bridge";

const BRIDGE_URL = "http://localhost:8090";
const SHORT_TIMEOUT = 3000;

describe("GoBridge", () => {
  let bridge: GoBridge;
  let goServiceAvailable = false;

  beforeAll(async () => {
    bridge = new GoBridge(BRIDGE_URL);
    try {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), 2000);
      const res = await fetch(`${BRIDGE_URL}/health`, { signal: controller.signal });
      clearTimeout(timer);
      goServiceAvailable = res.ok;
    } catch {
      goServiceAvailable = false;
    }
  });

  describe("health", () => {
    it("should check health endpoint", async () => {
      const ok = await bridge.health();
      expect(typeof ok).toBe("boolean");
      if (goServiceAvailable) {
        expect(ok).toBe(true);
      }
    }, SHORT_TIMEOUT);
  });

  describe("agents", () => {
    it("should list agents (may be empty)", async () => {
      if (!goServiceAvailable) return;
      const agents = await bridge.listAgents();
      expect(Array.isArray(agents)).toBe(true);
    }, SHORT_TIMEOUT);

    it("should register and retrieve an agent", async () => {
      if (!goServiceAvailable) return;
      const agent = await bridge.registerAgent({
        id: "test-ts-agent",
        display_name: "TS Test Agent",
        emoji: "🧪",
        role: "tester",
        skills: ["testing"],
        status: "idle",
      });
      expect(agent.id).toBeTruthy();

      const agents = await bridge.listAgents();
      const found = agents.find((a) => a.id === "test-ts-agent");
      expect(found).toBeTruthy();
    }, SHORT_TIMEOUT);

    it("should update agent status", async () => {
      if (!goServiceAvailable) return;
      await bridge.updateAgentStatus("test-ts-agent", "busy");
    }, SHORT_TIMEOUT);
  });

  describe("tasks", () => {
    let taskId: string;

    it("should create a task", async () => {
      if (!goServiceAvailable) return;
      const task = await bridge.createTask({
        project_id: "test-proj",
        title: "TS Test Task",
        assignee: "test-ts-agent",
        priority: "P2",
      });
      expect(task.id).toBeTruthy();
      taskId = task.id;
    }, SHORT_TIMEOUT);

    it("should query tasks", async () => {
      if (!goServiceAvailable) return;
      const tasks = await bridge.queryTasks({ project_id: "test-proj" });
      expect(Array.isArray(tasks)).toBe(true);
    }, SHORT_TIMEOUT);

    it("should update task status", async () => {
      if (!goServiceAvailable || !taskId) return;
      await bridge.updateTask(taskId, { status: "assigned" });
    }, SHORT_TIMEOUT);
  });

  describe("projects", () => {
    it("should create a project", async () => {
      if (!goServiceAvailable) return;
      const project = await bridge.createProject({
        name: "TS Test Project",
        group_id: "oc_ts_test_" + Date.now(),
        team_agents: ["agent-1"],
        tech_stack: ["TypeScript"],
      });
      expect(project.id).toBeTruthy();
    }, SHORT_TIMEOUT);
  });

  describe("memory", () => {
    it("should save and query memory", async () => {
      if (!goServiceAvailable) return;
      await bridge.saveDecision("test-proj", {
        category: "decision",
        content: "Use Jest for testing",
        pinned: true,
      });
      const memories = await bridge.queryMemory("test-proj", "Jest");
      expect(Array.isArray(memories)).toBe(true);
    }, SHORT_TIMEOUT);
  });

  describe("experiences", () => {
    it("should save and query experiences", async () => {
      if (!goServiceAvailable) return;
      await bridge.addExperience("test-ts-agent", {
        category: "best_practice",
        content: "Always mock external APIs in tests",
        tags: ["testing", "mocking"],
      });
      const exps = await bridge.queryExperiences("test-ts-agent", ["testing"]);
      expect(Array.isArray(exps)).toBe(true);
    }, SHORT_TIMEOUT);
  });

  describe("dashboard", () => {
    it("should get dashboard overview", async () => {
      if (!goServiceAvailable) return;
      const dashboard = await bridge.getDashboard();
      expect(dashboard).toBeTruthy();
    }, SHORT_TIMEOUT);
  });
});
