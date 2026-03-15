package models

import (
	"time"

	"gorm.io/gorm"
)

// Session 会话模型
type Session struct {
	ID        string         `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserID    uint           `gorm:"index" json:"user_id"`
	Title     string         `gorm:"type:varchar(255)" json:"title"`
	Mode      string         `gorm:"type:varchar(20);default:'auto'" json:"mode"` // local, online, auto
	Messages  []Message      `gorm:"foreignKey:SessionID" json:"messages"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Message 消息模型
type Message struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID  string    `gorm:"index;type:varchar(64)" json:"session_id"`
	Role       string    `gorm:"type:varchar(20)" json:"role"` // user, assistant, system
	Content    string    `gorm:"type:text" json:"content"`
	ToolCalls  string    `gorm:"type:json" json:"tool_calls,omitempty"`  // JSON 存储工具调用
	ToolResult string    `gorm:"type:text" json:"tool_result,omitempty"` // 工具返回结果
	CreatedAt  time.Time `json:"created_at"`
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}

func (Message) TableName() string {
	return "messages"
}
