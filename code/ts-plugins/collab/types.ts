// OpenClaw Plugin SDK types (peer dependency)
// These mirror the OpenClaw plugin-sdk interfaces for type safety

export interface PluginApi {
  registerTool(tool: ToolDefinition, opts?: { name?: string }): void;
  on(hookName: string, handler: (...args: any[]) => any, opts?: { priority?: number }): void;
  registerCommand(cmd: CommandDefinition): void;
  registerService(svc: ServiceDefinition): void;
  registerHttpRoute(route: HttpRouteDefinition): void;
  getConfig(): PluginConfig;
}

export interface ToolDefinition {
  description: string;
  parameters: Record<string, any>;
  execute(params: Record<string, any>, context: ToolContext): Promise<string>;
}

export interface ToolContext {
  agentId: string;
  sessionKey: string;
  projectId?: string;
}

export interface CommandDefinition {
  name: string;
  description: string;
  handler(args: string[], context: CommandContext): Promise<string>;
}

export interface CommandContext {
  agentId: string;
  channelId: string;
}

export interface ServiceDefinition {
  id: string;
  start(ctx: ServiceContext): Promise<void> | void;
  stop?(): Promise<void> | void;
}

export interface ServiceContext {
  logger: Logger;
}

export interface HttpRouteDefinition {
  method: "GET" | "POST" | "PATCH" | "DELETE";
  path: string;
  handler(req: any, res: any): Promise<void>;
}

export interface PluginConfig {
  goServiceUrl: string;
  feishuEnabled: boolean;
  contextMaxTokens: number;
  heartbeatIntervalMs: number;
}

export interface Logger {
  info(msg: string, ...args: any[]): void;
  warn(msg: string, ...args: any[]): void;
  error(msg: string, ...args: any[]): void;
}

// Hook event types
export interface BeforeAgentStartEvent {
  agentId: string;
  sessionKey: string;
  messages: Message[];
}

export interface BeforeAgentStartResult {
  prependContext?: string;
  systemPrompt?: string;
}

export interface MessageSendingEvent {
  agentId: string;
  sessionKey: string;
  content: string;
  channelId?: string;
}

export interface MessageSendingResult {
  content?: string;
  cancel?: boolean;
}

export interface BeforeToolCallEvent {
  agentId: string;
  sessionKey: string;
  toolName: string;
  params: Record<string, any>;
}

export interface BeforeToolCallResult {
  params?: Record<string, any>;
  block?: boolean;
  blockReason?: string;
}

export interface AfterToolCallEvent {
  agentId: string;
  sessionKey: string;
  toolName: string;
  params: Record<string, any>;
  result: string;
  error?: string;
}

export interface AgentEndEvent {
  agentId: string;
  sessionKey: string;
  error?: string;
  tokenUsage?: { input: number; output: number };
}

export interface BeforeCompactionEvent {
  agentId: string;
  sessionKey: string;
  messageCount: number;
  totalTokens: number;
}

export interface Message {
  role: string;
  content: string;
}

// Go service API types
export interface CollabContext {
  project: Project;
  current_iteration?: Iteration;
  pinned_memories: ProjectMemory[];
  my_tasks: Task[];
  team_agents: AgentRecord[];
  stats: TaskStats;
  experiences: AgentExperience[];
}

export interface Project {
  id: string;
  name: string;
  description: string;
  group_id: string;
  status: string;
  team_agents: string[];
  tech_stack: string[];
}

export interface Iteration {
  id: string;
  project_id: string;
  name: string;
  goal: string;
  status: string;
}

export interface ProjectMemory {
  id: string;
  project_id: string;
  category: string;
  content: string;
  pinned: boolean;
}

export interface Task {
  id: string;
  project_id: string;
  parent_id?: string;
  title: string;
  description: string;
  assignee: string;
  assigned_by?: string;
  status: string;
  priority: string;
  deliverable: string;
  acceptance: string;
  result?: string;
  depends_on: string[];
  artifacts: string[];
  topology: string;
}

export interface TaskStats {
  total: number;
  in_progress: number;
  blocked: number;
  completed: number;
  review: number;
}

export interface AgentRecord {
  id: string;
  display_name: string;
  emoji: string;
  role: string;
  skills: string[];
  status: string;
  is_subagent: boolean;
  parent_agent: string;
  current_load: number;
}

export interface AgentExperience {
  id: string;
  agent_id: string;
  category: string;
  content: string;
  tags: string[];
}
