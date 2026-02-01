package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// QuestionBankTool provides access to interview question bank.
type QuestionBankTool struct {
	questions map[string][]QuestionTemplate
	random    *rand.Rand
}

// QuestionTemplate represents a question template.
type QuestionTemplate struct {
	ID              string   `json:"id"`
	Content         string   `json:"content"`
	Type            string   `json:"type"`
	Difficulty      string   `json:"difficulty"`
	Category        string   `json:"category"`
	Skills          []string `json:"skills"`
	ExpectedAnswer  string   `json:"expected_answer"`
	FollowUps       []string `json:"follow_ups"`
	Tags            []string `json:"tags"`
	TimeLimit       int      `json:"time_limit_minutes"`
}

// QuestionBankInput represents the input for question bank query.
type QuestionBankInput struct {
	Skills     []string `json:"skills"`
	Difficulty string   `json:"difficulty"`
	Type       string   `json:"type"`
	Count      int      `json:"count"`
	Category   string   `json:"category"`
}

// NewQuestionBankTool creates a new question bank tool.
func NewQuestionBankTool() *QuestionBankTool {
	t := &QuestionBankTool{
		questions: make(map[string][]QuestionTemplate),
		random:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	t.loadDefaultQuestions()
	return t
}

// Info returns the tool information.
func (t *QuestionBankTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "question_bank",
		Description: "从题库中获取面试题目，支持按技能、难度、类型筛选",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skills": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "需要覆盖的技能列表",
				},
				"difficulty": map[string]any{
					"type":        "string",
					"enum":        []string{"basic", "intermediate", "advanced", "expert"},
					"description": "难度级别",
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"technical", "behavioral", "scenario", "coding", "system_design"},
					"description": "题目类型",
				},
				"count": map[string]any{
					"type":        "integer",
					"description": "需要的题目数量",
					"default":     5,
				},
				"category": map[string]any{
					"type":        "string",
					"description": "题目分类",
				},
			},
		},
	}, nil
}

// InvokableRun executes the tool.
func (t *QuestionBankTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params QuestionBankInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if params.Count <= 0 {
		params.Count = 5
	}

	questions := t.selectQuestions(params)
	result, err := json.Marshal(questions)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
}

func (t *QuestionBankTool) selectQuestions(params QuestionBankInput) []QuestionTemplate {
	var candidates []QuestionTemplate

	// Filter questions based on criteria
	for category, questions := range t.questions {
		if params.Category != "" && category != params.Category {
			continue
		}

		for _, q := range questions {
			// Check difficulty
			if params.Difficulty != "" && q.Difficulty != params.Difficulty {
				continue
			}

			// Check type
			if params.Type != "" && q.Type != params.Type {
				continue
			}

			// Check skills
			if len(params.Skills) > 0 {
				hasSkill := false
				for _, skill := range params.Skills {
					for _, qSkill := range q.Skills {
						if strings.EqualFold(skill, qSkill) {
							hasSkill = true
							break
						}
					}
					if hasSkill {
						break
					}
				}
				if !hasSkill {
					continue
				}
			}

			candidates = append(candidates, q)
		}
	}

	// Randomly select required number of questions
	if len(candidates) <= params.Count {
		return candidates
	}

	// Shuffle and select
	t.random.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	return candidates[:params.Count]
}

// AddQuestion adds a question to the bank.
func (t *QuestionBankTool) AddQuestion(q QuestionTemplate) {
	t.questions[q.Category] = append(t.questions[q.Category], q)
}

func (t *QuestionBankTool) loadDefaultQuestions() {
	// Programming fundamentals
	t.questions["programming"] = []QuestionTemplate{
		{
			ID:         "prog_001",
			Content:    "请解释什么是面向对象编程的三大特性，并举例说明。",
			Type:       "technical",
			Difficulty: "basic",
			Category:   "programming",
			Skills:     []string{"OOP", "编程基础"},
			ExpectedAnswer: "封装、继承、多态。封装是将数据和操作封装在类中；继承是子类继承父类的属性和方法；多态是同一个方法在不同对象上有不同的表现形式。",
			FollowUps:  []string{"你能说说多态的实际应用场景吗？", "继承和组合各有什么优缺点？"},
			Tags:       []string{"OOP", "基础"},
			TimeLimit:  5,
		},
		{
			ID:         "prog_002",
			Content:    "请解释并比较进程和线程的区别，以及什么时候应该使用多进程vs多线程。",
			Type:       "technical",
			Difficulty: "intermediate",
			Category:   "programming",
			Skills:     []string{"操作系统", "并发编程"},
			ExpectedAnswer: "进程是资源分配的基本单位，线程是CPU调度的基本单位。进程间隔离性好但开销大，线程共享进程资源开销小但需要处理同步问题。CPU密集型用多进程，IO密集型用多线程。",
			FollowUps:  []string{"如何解决多线程的竞态条件？", "什么是协程？"},
			Tags:       []string{"并发", "操作系统"},
			TimeLimit:  8,
		},
		{
			ID:         "prog_003",
			Content:    "请解释什么是死锁，产生的四个必要条件，以及如何预防死锁。",
			Type:       "technical",
			Difficulty: "advanced",
			Category:   "programming",
			Skills:     []string{"并发编程", "操作系统"},
			ExpectedAnswer: "死锁是两个或多个进程互相等待对方持有的资源。四个条件：互斥、持有并等待、不可抢占、循环等待。预防方法：破坏任一条件，如资源一次性分配、允许抢占、按序申请资源等。",
			FollowUps:  []string{"实际项目中你遇到过死锁吗？如何排查的？"},
			Tags:       []string{"死锁", "并发"},
			TimeLimit:  10,
		},
	}

	// System design
	t.questions["system_design"] = []QuestionTemplate{
		{
			ID:         "sd_001",
			Content:    "请设计一个短链服务，需要支持生成短链、跳转、统计访问量等功能。",
			Type:       "system_design",
			Difficulty: "intermediate",
			Category:   "system_design",
			Skills:     []string{"系统设计", "分布式系统"},
			ExpectedAnswer: "核心组件：短链生成器、存储层、重定向服务、统计服务。短链算法可用hash+62进制、发号器等。存储考虑分库分表、缓存。需考虑高可用、防刷等。",
			FollowUps:  []string{"如何保证短链的唯一性？", "如何处理热点短链？"},
			Tags:       []string{"URL shortener", "高并发"},
			TimeLimit:  20,
		},
		{
			ID:         "sd_002",
			Content:    "请设计一个分布式限流系统，需要支持多种限流策略。",
			Type:       "system_design",
			Difficulty: "advanced",
			Category:   "system_design",
			Skills:     []string{"系统设计", "分布式系统", "限流"},
			ExpectedAnswer: "限流算法：令牌桶、漏桶、滑动窗口。分布式实现：Redis + Lua脚本、本地缓存+定期同步。需要考虑：限流粒度、降级策略、熔断机制。",
			FollowUps:  []string{"令牌桶和漏桶有什么区别？", "如何处理分布式环境下的时钟偏移？"},
			Tags:       []string{"限流", "高可用"},
			TimeLimit:  20,
		},
	}

	// Database
	t.questions["database"] = []QuestionTemplate{
		{
			ID:         "db_001",
			Content:    "请解释MySQL中的索引原理，以及如何优化慢查询。",
			Type:       "technical",
			Difficulty: "intermediate",
			Category:   "database",
			Skills:     []string{"MySQL", "数据库", "性能优化"},
			ExpectedAnswer: "MySQL主要使用B+树索引。优化慢查询：1)分析执行计划 2)添加合适索引 3)避免全表扫描 4)优化SQL写法 5)考虑分库分表。",
			FollowUps:  []string{"B+树和B树有什么区别？", "什么是覆盖索引？"},
			Tags:       []string{"MySQL", "索引", "优化"},
			TimeLimit:  10,
		},
		{
			ID:         "db_002",
			Content:    "请解释数据库事务的ACID特性，以及MySQL中的事务隔离级别。",
			Type:       "technical",
			Difficulty: "intermediate",
			Category:   "database",
			Skills:     []string{"MySQL", "数据库", "事务"},
			ExpectedAnswer: "ACID：原子性、一致性、隔离性、持久性。MySQL隔离级别：读未提交、读已提交、可重复读、串行化。InnoDB默认可重复读，通过MVCC实现。",
			FollowUps:  []string{"可重复读能解决幻读问题吗？", "什么是间隙锁？"},
			Tags:       []string{"事务", "ACID"},
			TimeLimit:  10,
		},
	}

	// Behavioral
	t.questions["behavioral"] = []QuestionTemplate{
		{
			ID:         "beh_001",
			Content:    "请描述一个你曾经遇到的最具挑战性的技术问题，以及你是如何解决的。",
			Type:       "behavioral",
			Difficulty: "intermediate",
			Category:   "behavioral",
			Skills:     []string{"问题解决", "技术深度"},
			ExpectedAnswer: "STAR方法：描述情景、任务、行动、结果。重点考察：问题分析能力、解决方案选择、团队协作、技术判断。",
			FollowUps:  []string{"如果重新做一次，你会有什么不同的做法？", "这个经历给你带来了什么成长？"},
			Tags:       []string{"STAR", "问题解决"},
			TimeLimit:  10,
		},
		{
			ID:         "beh_002",
			Content:    "请描述一次你与团队成员发生意见分歧的经历，你是如何处理的？",
			Type:       "behavioral",
			Difficulty: "intermediate",
			Category:   "behavioral",
			Skills:     []string{"沟通能力", "团队协作"},
			ExpectedAnswer: "考察沟通方式、冲突解决能力、换位思考、最终结果。好的回答应该展示：尊重不同意见、数据驱动决策、寻求共识、维护团队关系。",
			FollowUps:  []string{"如果对方坚持己见，你会怎么做？"},
			Tags:       []string{"沟通", "冲突解决"},
			TimeLimit:  8,
		},
	}

	// Go specific
	t.questions["golang"] = []QuestionTemplate{
		{
			ID:         "go_001",
			Content:    "请解释Go语言中的goroutine和channel，以及它们是如何实现并发的。",
			Type:       "technical",
			Difficulty: "intermediate",
			Category:   "golang",
			Skills:     []string{"Go", "并发编程"},
			ExpectedAnswer: "goroutine是轻量级线程，由Go运行时管理。channel是goroutine间通信的管道，支持同步和异步。CSP并发模型：通过通信共享内存，而非通过共享内存通信。",
			FollowUps:  []string{"goroutine和线程有什么区别？", "如何避免goroutine泄漏？"},
			Tags:       []string{"Go", "goroutine", "channel"},
			TimeLimit:  10,
		},
		{
			ID:         "go_002",
			Content:    "请解释Go语言中的GC机制，以及如何优化GC性能。",
			Type:       "technical",
			Difficulty: "advanced",
			Category:   "golang",
			Skills:     []string{"Go", "GC", "性能优化"},
			ExpectedAnswer: "Go使用三色标记清除GC，并发标记减少STW时间。优化方法：减少堆分配、使用sync.Pool、避免短生命周期大对象、调整GOGC参数。",
			FollowUps:  []string{"什么是写屏障？", "如何分析GC性能问题？"},
			Tags:       []string{"Go", "GC", "优化"},
			TimeLimit:  12,
		},
	}
}
