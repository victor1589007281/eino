/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agent

import (
	"regexp"
	"strings"
)

// IntentClassifier classifies user intent.
type IntentClassifier struct {
	patterns map[IntentType][]*regexp.Regexp
	keywords map[IntentType][]string
}

// NewIntentClassifier creates a new intent classifier.
func NewIntentClassifier() *IntentClassifier {
	c := &IntentClassifier{
		patterns: make(map[IntentType][]*regexp.Regexp),
		keywords: make(map[IntentType][]string),
	}

	// Define keywords for each intent type
	c.keywords[IntentConcept] = []string{
		"什么是", "解释", "介绍", "概念", "是什么", "含义",
		"what is", "explain", "describe", "concept", "meaning",
	}

	c.keywords[IntentFunction] = []string{
		"函数", "实现", "怎么", "如何", "源码", "代码",
		"function", "implement", "how", "source", "code",
	}

	c.keywords[IntentCallChain] = []string{
		"调用", "流程", "过程", "路径", "链", "顺序",
		"call", "flow", "process", "path", "chain", "trace",
	}

	c.keywords[IntentArchitecture] = []string{
		"架构", "模块", "子系统", "组成", "结构", "设计",
		"architecture", "module", "subsystem", "structure", "design",
	}

	c.keywords[IntentComparison] = []string{
		"区别", "比较", "差异", "异同", "对比", "不同",
		"difference", "compare", "versus", "vs", "different",
	}

	c.keywords[IntentDebug] = []string{
		"为什么", "原因", "问题", "错误", "bug", "调试",
		"why", "reason", "problem", "error", "issue", "debug",
	}

	return c
}

// IntentResult represents the result of intent classification.
type IntentResult struct {
	Primary    IntentType   `json:"primary"`
	Secondary  []IntentType `json:"secondary"`
	Confidence float64      `json:"confidence"`
	Keywords   []string     `json:"keywords"`
	Entities   []string     `json:"entities"`
}

// Classify classifies the intent of a query.
func (c *IntentClassifier) Classify(query string) *IntentResult {
	query = strings.ToLower(query)

	// Score each intent type
	scores := make(map[IntentType]float64)
	matchedKeywords := make(map[IntentType][]string)

	for intent, keywords := range c.keywords {
		for _, kw := range keywords {
			if strings.Contains(query, kw) {
				scores[intent]++
				matchedKeywords[intent] = append(matchedKeywords[intent], kw)
			}
		}
	}

	// Find primary intent
	var primary IntentType = IntentUnknown
	var maxScore float64

	for intent, score := range scores {
		if score > maxScore {
			maxScore = score
			primary = intent
		}
	}

	// Find secondary intents
	var secondary []IntentType
	for intent, score := range scores {
		if intent != primary && score > 0 {
			secondary = append(secondary, intent)
		}
	}

	// Extract entities (function names, subsystems, etc.)
	entities := c.extractEntities(query)

	// Calculate confidence
	confidence := 0.0
	if maxScore > 0 {
		totalScore := 0.0
		for _, score := range scores {
			totalScore += score
		}
		confidence = maxScore / totalScore
	}

	return &IntentResult{
		Primary:    primary,
		Secondary:  secondary,
		Confidence: confidence,
		Keywords:   matchedKeywords[primary],
		Entities:   entities,
	}
}

// extractEntities extracts entities from the query.
func (c *IntentClassifier) extractEntities(query string) []string {
	entities := make([]string, 0)

	// Extract potential function names (snake_case or camelCase)
	funcRe := regexp.MustCompile(`[a-z_][a-z0-9_]{2,}`)
	matches := funcRe.FindAllString(query, -1)
	for _, m := range matches {
		// Filter out common words
		if !isCommonWord(m) {
			entities = append(entities, m)
		}
	}

	// Extract subsystem names
	subsystems := []string{
		"scheduler", "memory", "filesystem", "network", "driver",
		"vfs", "ext4", "btrfs", "tcp", "udp", "socket",
		"mm", "vm", "page", "slab", "kmalloc",
	}
	queryLower := strings.ToLower(query)
	for _, sub := range subsystems {
		if strings.Contains(queryLower, sub) {
			entities = append(entities, sub)
		}
	}

	return entities
}

// isCommonWord checks if a word is a common word.
func isCommonWord(word string) bool {
	common := map[string]bool{
		"the": true, "and": true, "for": true, "with": true,
		"this": true, "that": true, "from": true, "have": true,
		"what": true, "how": true, "why": true, "when": true,
		"which": true, "where": true, "who": true,
	}
	return common[strings.ToLower(word)]
}

// TaskPlanner plans tasks based on intent.
type TaskPlanner struct {
	classifier *IntentClassifier
}

// NewTaskPlanner creates a new task planner.
func NewTaskPlanner() *TaskPlanner {
	return &TaskPlanner{
		classifier: NewIntentClassifier(),
	}
}

// Plan creates a task plan for the query.
func (p *TaskPlanner) Plan(query string) *TaskPlan {
	result := p.classifier.Classify(query)

	plan := &TaskPlan{
		Query:    query,
		Intent:   result.Primary,
		Tasks:    make([]*Task, 0),
		Parallel: true,
	}

	// Create tasks based on intent
	switch result.Primary {
	case IntentFunction:
		plan.Tasks = p.planFunctionAnalysis(result)
	case IntentCallChain:
		plan.Tasks = p.planCallChainAnalysis(result)
	case IntentArchitecture:
		plan.Tasks = p.planArchitectureAnalysis(result)
	case IntentConcept:
		plan.Tasks = p.planConceptExplanation(result)
	case IntentComparison:
		plan.Tasks = p.planComparison(result)
	default:
		plan.Tasks = p.planGeneral(result)
	}

	return plan
}

func (p *TaskPlanner) planFunctionAnalysis(result *IntentResult) []*Task {
	tasks := []*Task{
		{
			ID:    "locate",
			Type:  IntentFunction,
			Agent: "CodeSearchAgent",
			Input: map[string]interface{}{
				"entities": result.Entities,
				"action":   "locate",
			},
		},
		{
			ID:    "analyze",
			Type:  IntentFunction,
			Agent: "FunctionAnalyzerAgent",
			Input: map[string]interface{}{
				"entities": result.Entities,
				"action":   "analyze",
			},
			Dependencies: []string{"locate"},
		},
		{
			ID:    "callchain",
			Type:  IntentCallChain,
			Agent: "CallChainAnalyzerAgent",
			Input: map[string]interface{}{
				"entities":  result.Entities,
				"direction": "both",
				"depth":     5,
			},
			Dependencies: []string{"locate"},
		},
	}
	return tasks
}

func (p *TaskPlanner) planCallChainAnalysis(result *IntentResult) []*Task {
	tasks := []*Task{
		{
			ID:    "locate",
			Type:  IntentFunction,
			Agent: "CodeSearchAgent",
			Input: map[string]interface{}{
				"entities": result.Entities,
				"action":   "locate",
			},
		},
		{
			ID:    "forward_chain",
			Type:  IntentCallChain,
			Agent: "CallChainAnalyzerAgent",
			Input: map[string]interface{}{
				"entities":  result.Entities,
				"direction": "callees",
				"depth":     8,
			},
			Dependencies: []string{"locate"},
		},
		{
			ID:    "backward_chain",
			Type:  IntentCallChain,
			Agent: "CallChainAnalyzerAgent",
			Input: map[string]interface{}{
				"entities":  result.Entities,
				"direction": "callers",
				"depth":     5,
			},
			Dependencies: []string{"locate"},
		},
	}
	return tasks
}

func (p *TaskPlanner) planArchitectureAnalysis(result *IntentResult) []*Task {
	tasks := []*Task{
		{
			ID:    "search_files",
			Type:  IntentArchitecture,
			Agent: "CodeSearchAgent",
			Input: map[string]interface{}{
				"entities": result.Entities,
				"action":   "search_subsystem",
			},
		},
		{
			ID:    "analyze_structure",
			Type:  IntentArchitecture,
			Agent: "ArchitectureAnalyzerAgent",
			Input: map[string]interface{}{
				"entities": result.Entities,
				"action":   "analyze",
			},
			Dependencies: []string{"search_files"},
		},
	}
	return tasks
}

func (p *TaskPlanner) planConceptExplanation(result *IntentResult) []*Task {
	tasks := []*Task{
		{
			ID:    "search_docs",
			Type:  IntentConcept,
			Agent: "CodeSearchAgent",
			Input: map[string]interface{}{
				"entities": result.Entities,
				"action":   "search_documentation",
			},
		},
		{
			ID:    "search_code",
			Type:  IntentConcept,
			Agent: "CodeSearchAgent",
			Input: map[string]interface{}{
				"entities": result.Entities,
				"action":   "search_examples",
			},
		},
	}
	return tasks
}

func (p *TaskPlanner) planComparison(result *IntentResult) []*Task {
	// Need at least 2 entities for comparison
	tasks := make([]*Task, 0)
	for i, entity := range result.Entities {
		tasks = append(tasks, &Task{
			ID:    "analyze_" + entity,
			Type:  IntentComparison,
			Agent: "FunctionAnalyzerAgent",
			Input: map[string]interface{}{
				"entity": entity,
				"index":  i,
			},
		})
	}
	return tasks
}

func (p *TaskPlanner) planGeneral(result *IntentResult) []*Task {
	tasks := []*Task{
		{
			ID:    "search",
			Type:  IntentUnknown,
			Agent: "CodeSearchAgent",
			Input: map[string]interface{}{
				"entities": result.Entities,
				"keywords": result.Keywords,
				"action":   "general_search",
			},
		},
	}
	return tasks
}
