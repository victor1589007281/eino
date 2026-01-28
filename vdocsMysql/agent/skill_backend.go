package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk/middlewares/skill"
	"gopkg.in/yaml.v3"
)

// MySQLSkillBackend implements skill.Backend for MySQL-specific skills.
type MySQLSkillBackend struct {
	sourcePath string
	skills     map[string]*skill.Skill
}

// NewMySQLSkillBackend creates a new MySQL skill backend.
func NewMySQLSkillBackend(sourcePath string) (*MySQLSkillBackend, error) {
	backend := &MySQLSkillBackend{
		sourcePath: sourcePath,
		skills:     make(map[string]*skill.Skill),
	}

	// Register built-in skills
	backend.registerBuiltinSkills()

	return backend, nil
}

// List returns all available skills.
func (b *MySQLSkillBackend) List(ctx context.Context) ([]skill.FrontMatter, error) {
	matters := make([]skill.FrontMatter, 0, len(b.skills))
	for _, s := range b.skills {
		matters = append(matters, s.FrontMatter)
	}
	return matters, nil
}

// Get returns a specific skill by name.
func (b *MySQLSkillBackend) Get(ctx context.Context, name string) (skill.Skill, error) {
	s, ok := b.skills[name]
	if !ok {
		return skill.Skill{}, fmt.Errorf("skill not found: %s", name)
	}
	return *s, nil
}

// registerBuiltinSkills registers built-in MySQL analysis skills.
func (b *MySQLSkillBackend) registerBuiltinSkills() {
	// InnoDB Transaction skill
	b.skills["innodb_transaction"] = &skill.Skill{
		FrontMatter: skill.FrontMatter{
			Name:        "innodb_transaction",
			Description: "InnoDB事务处理相关的代码搜索技能",
		},
		Content: `## InnoDB事务处理

### 核心文件位置
- storage/innobase/trx/trx0trx.cc - 事务主要实现
- storage/innobase/trx/trx0undo.cc - Undo日志
- storage/innobase/trx/trx0rec.cc - Undo记录
- storage/innobase/trx/trx0purge.cc - Purge线程

### 关键函数
- trx_start_low - 事务启动入口
- trx_commit - 事务提交
- trx_commit_low - 事务提交底层实现
- trx_rollback - 事务回滚
- trx_rollback_to_savepoint - 回滚到保存点

### 搜索模式
- 事务开始: "trx_start|trx_begin"
- 事务提交: "trx_commit"
- 事务回滚: "trx_rollback"
- Undo日志: "trx_undo"

### 关键数据结构
- trx_t - 事务结构体
- trx_sys_t - 事务系统
- trx_undo_t - Undo段`,
		BaseDirectory: "storage/innobase/trx/",
	}

	// InnoDB Lock skill
	b.skills["innodb_lock"] = &skill.Skill{
		FrontMatter: skill.FrontMatter{
			Name:        "innodb_lock",
			Description: "InnoDB锁机制相关的代码搜索技能",
		},
		Content: `## InnoDB锁机制

### 核心文件位置
- storage/innobase/lock/lock0lock.cc - 锁主要实现
- storage/innobase/lock/lock0wait.cc - 锁等待
- storage/innobase/lock/lock0prdt.cc - 谓词锁

### 关键函数
- lock_rec_lock - 行锁加锁
- lock_table - 表锁加锁
- lock_rec_unlock - 行锁解锁
- lock_deadlock_check - 死锁检测
- lock_wait_suspend_thread - 锁等待

### 搜索模式
- 行锁: "lock_rec_"
- 表锁: "lock_table"
- 死锁: "deadlock"
- 锁等待: "lock_wait"

### 关键数据结构
- lock_t - 锁结构体
- lock_sys_t - 锁系统
- lock_rec_t - 行锁结构`,
		BaseDirectory: "storage/innobase/lock/",
	}

	// Buffer Pool skill
	b.skills["innodb_buffer_pool"] = &skill.Skill{
		FrontMatter: skill.FrontMatter{
			Name:        "innodb_buffer_pool",
			Description: "InnoDB Buffer Pool相关的代码搜索技能",
		},
		Content: `## InnoDB Buffer Pool

### 核心文件位置
- storage/innobase/buf/buf0buf.cc - Buffer Pool主要实现
- storage/innobase/buf/buf0lru.cc - LRU管理
- storage/innobase/buf/buf0flu.cc - 刷脏页
- storage/innobase/buf/buf0rea.cc - 预读

### 关键函数
- buf_page_get_gen - 获取页面
- buf_page_create - 创建页面
- buf_flush_list - 刷脏页列表
- buf_LRU_get_free_block - 获取空闲块
- buf_read_page - 读取页面

### 搜索模式
- 页面获取: "buf_page_get"
- 刷脏: "buf_flush"
- LRU: "buf_LRU"
- 预读: "buf_read"

### 关键数据结构
- buf_pool_t - Buffer Pool结构
- buf_page_t - 页面控制块
- buf_block_t - 数据块`,
		BaseDirectory: "storage/innobase/buf/",
	}

	// Redo Log skill
	b.skills["innodb_redo_log"] = &skill.Skill{
		FrontMatter: skill.FrontMatter{
			Name:        "innodb_redo_log",
			Description: "InnoDB Redo Log相关的代码搜索技能",
		},
		Content: `## InnoDB Redo Log

### 核心文件位置
- storage/innobase/log/log0log.cc - Redo Log主要实现
- storage/innobase/log/log0write.cc - 日志写入
- storage/innobase/log/log0recv.cc - 崩溃恢复
- storage/innobase/log/log0chkp.cc - Checkpoint

### 关键函数
- log_write_up_to - 写日志到指定LSN
- log_buffer_flush_to_disk - 刷日志到磁盘
- log_checkpoint - 执行Checkpoint
- recv_recovery_from_checkpoint_start - 开始恢复

### 搜索模式
- 日志写入: "log_write"
- 日志刷盘: "log_flush|log_buffer_flush"
- Checkpoint: "log_checkpoint|checkpoint"
- 恢复: "recv_recovery"

### 关键数据结构
- log_t - 日志系统结构
- log_buffer_t - 日志缓冲
- lsn_t - 日志序列号`,
		BaseDirectory: "storage/innobase/log/",
	}

	// SQL Parser skill
	b.skills["sql_parser"] = &skill.Skill{
		FrontMatter: skill.FrontMatter{
			Name:        "sql_parser",
			Description: "SQL解析器相关的代码搜索技能",
		},
		Content: `## SQL解析器

### 核心文件位置
- sql/sql_parse.cc - SQL解析入口
- sql/sql_yacc.yy - 语法文件
- sql/sql_lex.cc - 词法分析
- sql/parse_tree_nodes.cc - 解析树节点

### 关键函数
- mysql_parse - SQL解析入口
- mysql_execute_command - 命令执行
- MYSQLparse - Bison解析函数
- dispatch_command - 命令分发

### 搜索模式
- 解析入口: "mysql_parse"
- 命令执行: "mysql_execute_command"
- 命令分发: "dispatch_command"
- 词法分析: "MYSQLlex"

### 关键数据结构
- THD - 线程句柄
- LEX - 词法结构
- Parse_tree_root - 解析树根`,
		BaseDirectory: "sql/",
	}

	// Query Optimizer skill
	b.skills["sql_optimizer"] = &skill.Skill{
		FrontMatter: skill.FrontMatter{
			Name:        "sql_optimizer",
			Description: "查询优化器相关的代码搜索技能",
		},
		Content: `## 查询优化器

### 核心文件位置
- sql/sql_optimizer.cc - 优化器主要实现
- sql/opt_range.cc - 范围优化
- sql/opt_sum.cc - 聚合优化
- sql/join_optimizer/ - 连接优化器

### 关键函数
- JOIN::optimize - 优化入口
- make_join_plan - 制定连接计划
- optimize_cond - 条件优化
- get_best_combination - 获取最优组合

### 搜索模式
- 优化入口: "JOIN::optimize"
- 代价计算: "cost_|calculate_cost"
- 访问路径: "access_path|best_access_path"
- 连接顺序: "join_order|best_order"

### 关键数据结构
- JOIN - 连接优化结构
- POSITION - 表位置
- AccessPath - 访问路径`,
		BaseDirectory: "sql/",
	}
}

// LoadSkillsFromDirectory loads skills from a directory.
func (b *MySQLSkillBackend) LoadSkillsFromDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		skill, err := b.loadSkillFromFile(path)
		if err != nil {
			return fmt.Errorf("failed to load skill from %s: %w", path, err)
		}

		b.skills[skill.Name] = skill
		return nil
	})
}

// loadSkillFromFile loads a skill from a markdown file with YAML front matter.
func (b *MySQLSkillBackend) loadSkillFromFile(path string) (*skill.Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Parse YAML front matter
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("skill file must start with YAML front matter")
	}

	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid front matter format")
	}

	var frontMatter skill.FrontMatter
	if err := yaml.Unmarshal([]byte(parts[0]), &frontMatter); err != nil {
		return nil, fmt.Errorf("failed to parse front matter: %w", err)
	}

	return &skill.Skill{
		FrontMatter:   frontMatter,
		Content:       strings.TrimSpace(parts[1]),
		BaseDirectory: filepath.Dir(path),
	}, nil
}
