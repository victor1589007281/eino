// Package intent 意图识别引擎类型定义
// 此文件重新导出 types 包的意图相关类型以保持向后兼容性
package intent

import "github.com/cloudwego/eino/vdocstool/algorithm/types"

// 类型别名 - 重新导出 types 包的意图相关类型
type (
	Recognizer = types.Recognizer
	Input      = types.IntentInput
	Message    = types.IntentMessage
	Result     = types.IntentResult
	Intent     = types.Intent
	Entity     = types.Entity
)

// 意图常量重新导出
const (
	// 邮件领域意图
	IntentEmailSearch     = types.IntentEmailSearch
	IntentEmailRead       = types.IntentEmailRead
	IntentEmailDownload   = types.IntentEmailDownload
	IntentEmailCategorize = types.IntentEmailCategorize
	IntentEmailSummarize  = types.IntentEmailSummarize
	IntentEmailSend       = types.IntentEmailSend
	IntentEmailReply      = types.IntentEmailReply

	// 记忆领域意图
	IntentMemoryRecall      = types.IntentMemoryRecall
	IntentMemoryReference   = types.IntentMemoryReference
	IntentMemoryTopicSwitch = types.IntentMemoryTopicSwitch
	IntentMemoryNewTopic    = types.IntentMemoryNewTopic
	IntentMemorySearch      = types.IntentMemorySearch

	// 搜索领域意图
	IntentWebSearch      = types.IntentWebSearch
	IntentWebSearchNews  = types.IntentWebSearchNews
	IntentWebSearchImage = types.IntentWebSearchImage
	IntentWebSearchLocal = types.IntentWebSearchLocal

	// 通用意图
	IntentUnknown = types.IntentUnknown
	IntentGreet   = types.IntentGreet
	IntentConfirm = types.IntentConfirm
	IntentCancel  = types.IntentCancel
	IntentHelp    = types.IntentHelp
)

// 实体类型常量重新导出
const (
	EntityTypePerson     = types.EntityTypePerson
	EntityTypeTime       = types.EntityTypeTime
	EntityTypeDate       = types.EntityTypeDate
	EntityTypeDuration   = types.EntityTypeDuration
	EntityTypeLocation   = types.EntityTypeLocation
	EntityTypeOrg        = types.EntityTypeOrg
	EntityTypeEmail      = types.EntityTypeEmail
	EntityTypeURL        = types.EntityTypeURL
	EntityTypeNumber     = types.EntityTypeNumber
	EntityTypeMoney      = types.EntityTypeMoney
	EntityTypeKeyword    = types.EntityTypeKeyword
	EntityTypeFileType   = types.EntityTypeFileType
	EntityTypeTechnology = types.EntityTypeTechnology
)

// 领域常量重新导出
const (
	DomainEmail   = types.DomainEmail
	DomainMemory  = types.DomainMemory
	DomainSearch  = types.DomainSearch
	DomainGeneral = types.DomainGeneral
)
