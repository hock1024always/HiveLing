package models

import (
	"time"

	"gorm.io/gorm"
)

// Session 会话模型
type Session struct {
	ID           string         `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserID       uint           `gorm:"index" json:"user_id"`
	Title        string         `gorm:"type:varchar(255)" json:"title"`
	Mode         string         `gorm:"type:varchar(20);default:'auto'" json:"mode"` // local, online, auto
	Messages     []Message      `gorm:"foreignKey:SessionID" json:"messages"`
	Summary      string         `gorm:"type:text" json:"summary"`              // 会话摘要
	Topics       string         `gorm:"type:json" json:"topics"`               // 话题标签 JSON
	KeyEntities  string         `gorm:"type:json" json:"key_entities"`         // 关键实体 JSON
	MessageCount int            `gorm:"default:0" json:"message_count"`        // 消息计数
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Message 消息模型
type Message struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID    string    `gorm:"index;type:varchar(64)" json:"session_id"`
	Role         string    `gorm:"type:varchar(20)" json:"role"`                    // user, assistant, system
	Content      string    `gorm:"type:text" json:"content"`
	Tag          string    `gorm:"type:varchar(50)" json:"tag"`                     // 消息标签
	Importance   int       `gorm:"default:5" json:"importance"`                     // 重要性 1-10
	Topics       string    `gorm:"type:json" json:"topics,omitempty"`               // 涉及的话题 JSON
	Entities     string    `gorm:"type:json" json:"entities,omitempty"`             // 提及的实体 JSON
	ToolCalls    string    `gorm:"type:json" json:"tool_calls,omitempty"`           // JSON 存储工具调用
	ToolResult   string    `gorm:"type:text" json:"tool_result,omitempty"`          // 工具返回结果
	IsSummarized bool      `gorm:"default:false" json:"is_summarized"`              // 是否已被摘要
	CreatedAt    time.Time `json:"created_at"`
}

// MessageTag 消息标签常量
const (
	TagQuestion      = "question"       // 用户提问
	TagAnswer        = "answer"         // AI 回答
	TagFollowUp      = "follow_up"      // 追问
	TagClarification = "clarification"  // 澄清说明
	TagFact          = "fact"           // 事实陈述
	TagOpinion       = "opinion"        // 观点表达
	TagToolCall      = "tool_call"      // 工具调用
	TagToolResult    = "tool_result"    // 工具结果
	TagSummary       = "summary"        // 摘要
	TagGreeting      = "greeting"       // 问候
	TagThanks        = "thanks"         // 感谢
	TagOther         = "other"          // 其他
)

// Topic 话题结构
type Topic struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
	Count    int      `json:"count"` // 出现次数
}

// Entity 实体结构
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"` // person, event, place, time, etc.
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}

func (Message) TableName() string {
	return "messages"
}
