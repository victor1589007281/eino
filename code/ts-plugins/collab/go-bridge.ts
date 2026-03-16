import type { CollabContext, Task, AgentRecord, ProjectMemory, AgentExperience } from "./types.js";

export class GoBridge {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
  }

  // --- Health ---

  async health(): Promise<boolean> {
    try {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), 2000);
      const res = await fetch(`${this.baseUrl}/health`, { signal: controller.signal });
      clearTimeout(timer);
      if (!res.ok) return false;
      const data = await res.json();
      return data.status === "ok";
    } catch {
      return false;
    }
  }

  // --- Context ---

  async getContext(projectId: string, agentId: string): Promise<CollabContext | null> {
    try {
      return await this.get(`/api/v1/projects/${projectId}/context?agent_id=${agentId}`);
    } catch {
      return null;
    }
  }

  // --- Tasks ---

  async createTask(task: Partial<Task>): Promise<Task> {
    return this.post("/api/v1/tasks", task);
  }

  async updateTask(taskId: string, fields: Record<string, any>): Promise<void> {
    await this.patch(`/api/v1/tasks/${taskId}`, fields);
  }

  async queryTasks(params: Record<string, string>): Promise<Task[]> {
    const qs = new URLSearchParams(params).toString();
    const res = await this.get(`/api/v1/tasks?${qs}`);
    return res.tasks || [];
  }

  async getTaskTree(taskId: string): Promise<Task[]> {
    const res = await this.get(`/api/v1/tasks/${taskId}/tree`);
    return res.tasks || [];
  }

  async addTaskActivity(taskId: string, type: string, actorId: string, content: string): Promise<void> {
    await this.post(`/api/v1/tasks/${taskId}/activities`, { type, actor_id: actorId, content });
  }

  async saveArtifact(taskId: string, artifact: Record<string, any>): Promise<void> {
    await this.post(`/api/v1/tasks/${taskId}/artifacts`, artifact);
  }

  // --- Projects ---

  async getProject(projectId: string): Promise<any> {
    return this.get(`/api/v1/projects/${projectId}`);
  }

  async getProjectByGroup(groupId: string): Promise<any | null> {
    try {
      return await this.get(`/api/v1/projects/by-group/${groupId}`);
    } catch {
      return null;
    }
  }

  async createProject(project: Record<string, any>): Promise<any> {
    return this.post("/api/v1/projects", project);
  }

  // --- Memory ---

  async saveDecision(projectId: string, memory: Partial<ProjectMemory>): Promise<void> {
    await this.post(`/api/v1/projects/${projectId}/memory`, {
      ...memory,
      category: memory.category || "decision",
    });
  }

  async queryMemory(projectId: string, query: string, category?: string): Promise<ProjectMemory[]> {
    const params = new URLSearchParams({ query });
    if (category) params.set("category", category);
    const res = await this.get(`/api/v1/projects/${projectId}/memory?${params}`);
    return res.memories || [];
  }

  async syncMemories(projectId: string, agentId: string, memories: Partial<ProjectMemory>[]): Promise<void> {
    await this.post(`/api/v1/projects/${projectId}/memory/sync`, {
      agent_id: agentId,
      memories,
    });
  }

  // --- Agents ---

  async listAgents(projectId?: string): Promise<AgentRecord[]> {
    const qs = projectId ? `?project_id=${projectId}` : "";
    const res = await this.get(`/api/v1/agents${qs}`);
    return res.agents || [];
  }

  async registerAgent(agent: Partial<AgentRecord>): Promise<AgentRecord> {
    return this.post("/api/v1/agents/register", agent);
  }

  async updateAgentStatus(agentId: string, status: string): Promise<void> {
    await this.patch(`/api/v1/agents/${agentId}/status`, { status });
  }

  async updateAgentLoad(agentId: string, load: number): Promise<void> {
    await this.patch(`/api/v1/agents/${agentId}/load`, { load });
  }

  // --- Experiences ---

  async addExperience(agentId: string, exp: Partial<AgentExperience>): Promise<void> {
    await this.post(`/api/v1/agents/${agentId}/experiences`, exp);
  }

  async queryExperiences(agentId: string, keywords?: string[]): Promise<AgentExperience[]> {
    const params = new URLSearchParams();
    keywords?.forEach((kw) => params.append("keywords", kw));
    const res = await this.get(`/api/v1/agents/${agentId}/experiences?${params}`);
    return res.experiences || [];
  }

  // --- Feishu ---

  async feishuSend(req: {
    agent_id: string;
    channel_id: string;
    content: string;
    is_subagent?: boolean;
    parent_agent_id?: string;
    subagent_name?: string;
  }): Promise<void> {
    await this.post("/api/v1/feishu/send", req);
  }

  async feishuSelectBot(req: {
    agent_id: string;
    group_id: string;
    is_subagent?: boolean;
    parent_agent_id?: string;
  }): Promise<string> {
    const res = await this.post("/api/v1/feishu/select-bot", req);
    return res.bot_app_id || "";
  }

  // --- Dashboard ---

  async getDashboard(): Promise<any> {
    return this.get("/api/v1/dashboard/overview");
  }

  // --- HTTP helpers ---

  private async get(path: string): Promise<any> {
    const res = await fetch(`${this.baseUrl}${path}`);
    if (!res.ok) throw new Error(`GET ${path}: ${res.status}`);
    return res.json();
  }

  private async post(path: string, body: any): Promise<any> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(`POST ${path}: ${res.status}`);
    return res.json();
  }

  private async patch(path: string, body: any): Promise<any> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(`PATCH ${path}: ${res.status}`);
    return res.json();
  }
}
