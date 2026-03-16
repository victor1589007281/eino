import type { GoBridge } from "../go-bridge.js";
import type { ToolDefinition, ToolContext, CodeRepository } from "../types.js";

export function createProjectTool(bridge: GoBridge): ToolDefinition {
  return {
    name: "create_project",
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

// 从上下文中解析项目名称（从协作上下文块中提取）
function extractProjectNameFromContext(context: ToolContext): string | null {
  // 尝试从 prompt 或上下文中提取项目名称
  const prompt = context?.prompt || "";
  
  // 匹配 "当前项目: xxx" 或 "项目名称: xxx" 格式
  const match = prompt.match(/当前项目[:：]\s*(\S+)/i) || 
                prompt.match(/项目名称[:：]\s*(\S+)/i);
  if (match) {
    return match[1].trim();
  }
  
  return null;
}

// 从 sessionKey 中提取群 ID（如 agent:test-manager:feishu:group:oc_xxx）
function extractGroupIdFromSessionKey(sessionKey: string): string | null {
  if (!sessionKey || typeof sessionKey !== "string") return null;
  const parts = sessionKey.split(":");
  for (const part of parts) {
    if (part.startsWith("oc_")) return part;
  }
  return null;
}

export function createSaveDecisionTool(bridge: GoBridge): ToolDefinition {
  return {
    name: "save_decision",
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
    name: "query_project_memory",
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

export function createQueryProjectInfoTool(bridge: GoBridge): ToolDefinition {
  return {
    name: "query_project_info",
    description: "查询项目的基本信息，包括仓库路径、文档目录等。当需要保存文件到项目仓库时使用此工具。",
    parameters: {
      type: "object",
      properties: {
        project_id: { type: "string", description: "Project ID (可选，不提供则自动使用系统中的项目)" },
      },
      required: [],
    },
    async execute(params: Record<string, any>, context: ToolContext): Promise<string> {
      try {
        // 从协作上下文中提取项目ID
        let projectId = params.project_id;
        
        // 如果参数中没有，尝试从上下文中获取
        if (!projectId) {
          // 1. 尝试从 context.projectId 获取
          if (context?.projectId) {
            projectId = context.projectId;
            console.log(`[collab] Using projectId from context: ${projectId}`);
          }
          // 2. 尝试从 sessionKey 中提取群 ID 查询
          else if (context?.sessionKey) {
            const groupId = extractGroupIdFromSessionKey(context.sessionKey);
            if (groupId) {
              try {
                const result = await bridge.getProjectByGroup(groupId);
                if (result && result.id) {
                  projectId = result.id;
                  console.log(`[collab] Auto-filled project_id ${projectId} from group ${groupId}`);
                }
              } catch (err) {
                console.log(`[collab] No project bound to group ${groupId}`);
              }
            }
          }
          // 3. 尝试从 channelId 查询（如果是群 ID）
          else if (context?.channelId?.startsWith("oc_")) {
            try {
              const result = await bridge.getProjectByGroup(context.channelId);
              if (result && result.id) {
                projectId = result.id;
                console.log(`[collab] Auto-filled project_id ${projectId} from channelId ${context.channelId}`);
              }
            } catch (err) {
              console.log(`[collab] No project bound to group ${context.channelId}`);
            }
          }
        }
        
        if (!projectId) {
          return JSON.stringify({ 
            success: false, 
            error: "未提供 project_id。请从协作上下文中获取项目ID（如 db-k8s-deploy 对应的项目ID），然后传入 project_id 参数。",
            hint: "协作上下文中显示了项目名称和项目ID，请确保正确提取项目ID。"
          });
        }
        
        const project = await bridge.getProject(projectId);
        if (!project) {
          return JSON.stringify({ success: false, error: "Project not found" });
        }
        
        // 构建文档目录路径
        const docsBaseDir = project.docs_base_dir || "/tmp/docs";
        const docPaths = {
          designs: `${docsBaseDir}/${project.id}/designs`,   // 功能设计文档
          research: `${docsBaseDir}/${project.id}/research`, // 调研分析报告
          system: `${docsBaseDir}/${project.id}/system`,     // 模块实现文档
          reports: `${docsBaseDir}/${project.id}/reports`,   // AI 任务汇总报告
        };
        
        // 获取主代码仓库
        const mainRepo = project.repositories?.code_repos?.find((r: CodeRepository) => r.type === "main") 
                      || project.repositories?.code_repos?.[0];
        
        return JSON.stringify({
          success: true,
          project: {
            id: project.id,
            name: project.name,
            docs_base_dir: docsBaseDir,
            doc_paths: docPaths,
            main_code_repo: mainRepo || null,
            all_code_repos: project.repositories?.code_repos || [],
            reference_repos: project.repositories?.reference_repos || [],
          },
          usage: {
            save_design: `设计文档 → ${docPaths.designs}/<文档名>.md`,
            save_research: `调研报告 → ${docPaths.research}/<报告名>.md`,
            save_system: `模块文档 → ${docPaths.system}/<模块名>.md`,
            save_report: `汇总报告 → ${docPaths.reports}/<报告名>.md`,
            save_code: mainRepo ? `源代码 → ${mainRepo.local_path}/<模块>/<文件>.ts` : "未配置代码仓库",
          },
          guidance: [
            "所有路径都是绝对路径，直接使用即可",
            "保存文件前，先调用 save_artifact 工具将交付物保存到数据库",
            "如果路径不存在或配置有误，会收到错误提示，请引导用户修正",
          ],
        });
      } catch (err: any) {
        return JSON.stringify({ success: false, error: err.message });
      }
    },
  };
}
