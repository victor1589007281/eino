// Package master 实现任务规划器
package master

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/config"
)

// Planner 任务规划器
type Planner struct {
	cfg       *config.Config
	subAgents map[string]agent.SubAgent
}

// NewPlanner 创建规划器
func NewPlanner(cfg *config.Config, subAgents map[string]agent.SubAgent) *Planner {
	return &Planner{
		cfg:       cfg,
		subAgents: subAgents,
	}
}

// CreatePlan 根据意图和输入创建执行计划
func (p *Planner) CreatePlan(ctx context.Context, intent *agent.Intent, input *agent.ArticleInput) *agent.Plan {
	plan := &agent.Plan{
		Steps:    make([]agent.PlanStep, 0),
		Parallel: false,
	}
	
	switch intent.Type {
	case agent.IntentTypePolish:
		// 全文润色：并行执行多个子Agent
		plan = p.createPolishPlan(input)
		
	case agent.IntentTypeGrammar:
		// 仅语法检查
		plan = p.createSingleAgentPlan("grammar", input)
		
	case agent.IntentTypeStyle:
		// 仅风格优化
		plan = p.createSingleAgentPlan("style", input)
		
	case agent.IntentTypeStructure:
		// 仅结构调整
		plan = p.createSingleAgentPlan("structure", input)
		
	case agent.IntentTypeModify:
		// 指定修改：使用内容Agent
		plan = p.createSingleAgentPlan("content", input)
		
	default:
		// 默认全文润色
		plan = p.createPolishPlan(input)
	}
	
	return plan
}

// createPolishPlan 创建全文润色计划
func (p *Planner) createPolishPlan(input *agent.ArticleInput) *agent.Plan {
	plan := &agent.Plan{
		Parallel:  true, // 并行执行
		Estimated: 5000, // 预估5秒
	}
	
	// 根据文章类型选择Agent
	agentOrder := p.getAgentOrder(input.Type)
	
	for i, agentName := range agentOrder {
		if _, ok := p.subAgents[agentName]; ok {
			plan.Steps = append(plan.Steps, agent.PlanStep{
				ID:        fmt.Sprintf("step_%d", i+1),
				AgentName: agentName,
				Action:    "process",
				Input:     input.Content,
				Priority:  p.getAgentPriority(agentName),
			})
		}
	}
	
	return plan
}

// createSingleAgentPlan 创建单Agent执行计划
func (p *Planner) createSingleAgentPlan(agentName string, input *agent.ArticleInput) *agent.Plan {
	return &agent.Plan{
		Parallel:  false,
		Estimated: 2000,
		Steps: []agent.PlanStep{
			{
				ID:        "step_1",
				AgentName: agentName,
				Action:    "process",
				Input:     input.Content,
				Priority:  agent.PriorityHigh,
			},
		},
	}
}

// getAgentOrder 根据文章类型获取Agent执行顺序
func (p *Planner) getAgentOrder(articleType agent.ArticleType) []string {
	switch articleType {
	case agent.ArticleTypeTech:
		// 技术文章：先检查逻辑和结构，再检查语法和风格
		return []string{"logic", "structure", "grammar", "style", "tech"}
		
	case agent.ArticleTypeTutorial:
		// 教程文章：注重结构清晰和步骤正确
		return []string{"structure", "logic", "grammar", "style"}
		
	case agent.ArticleTypeNews:
		// 新闻资讯：注重时效性和准确性
		return []string{"grammar", "style", "content"}
		
	default:
		// 非技术文章：先检查语法，再优化风格和结构
		return []string{"grammar", "style", "structure", "logic"}
	}
}

// getAgentPriority 获取Agent优先级
func (p *Planner) getAgentPriority(agentName string) agent.Priority {
	priorities := map[string]agent.Priority{
		"grammar":   agent.PriorityHigh,
		"logic":     agent.PriorityHigh,
		"structure": agent.PriorityMedium,
		"style":     agent.PriorityMedium,
		"content":   agent.PriorityLow,
		"tech":      agent.PriorityMedium,
	}
	
	if priority, ok := priorities[agentName]; ok {
		return priority
	}
	return agent.PriorityLow
}

// OptimizePlan 优化执行计划（根据资源和时间限制）
func (p *Planner) OptimizePlan(ctx context.Context, plan *agent.Plan, timeLimit int64) *agent.Plan {
	if plan.Estimated <= timeLimit {
		return plan
	}
	
	// 时间不足时，只保留高优先级任务
	optimized := &agent.Plan{
		Parallel:  plan.Parallel,
		Estimated: 0,
	}
	
	for _, step := range plan.Steps {
		if step.Priority >= agent.PriorityMedium {
			optimized.Steps = append(optimized.Steps, step)
			optimized.Estimated += 1000 // 假设每个高优先级任务1秒
		}
	}
	
	return optimized
}

// ReplanOnError 错误时重新规划
func (p *Planner) ReplanOnError(ctx context.Context, originalPlan *agent.Plan, failedStep string, err error) *agent.Plan {
	// 创建新计划，跳过失败的步骤
	newPlan := &agent.Plan{
		Parallel:  originalPlan.Parallel,
		Estimated: originalPlan.Estimated,
	}
	
	for _, step := range originalPlan.Steps {
		if step.AgentName != failedStep {
			newPlan.Steps = append(newPlan.Steps, step)
		}
	}
	
	return newPlan
}
