package models

import (
	"time"

	"gorm.io/gorm"
)

// KnowledgeChunk 知识块模型（用于 RAG 检索）
type KnowledgeChunk struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string         `gorm:"type:varchar(255);index" json:"title"`
	Content     string         `gorm:"type:text" json:"content"`
	Category    string         `gorm:"type:varchar(50);index" json:"category"` // person, event, battle, school, state, culture
	Keywords    string         `gorm:"type:varchar(500)" json:"keywords"`
	Source      string         `gorm:"type:varchar(255)" json:"source"`      // 来源文献
	Period      string         `gorm:"type:varchar(100)" json:"period"`      // 时期：春秋/战国
	YearStart   int            `gorm:"default:0" json:"year_start"`          // 年份范围（负数表示公元前）
	YearEnd     int            `gorm:"default:0" json:"year_end"`
	RelatedIDs  string         `gorm:"type:json" json:"related_ids"`         // 关联的其他知识块ID
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (KnowledgeChunk) TableName() string {
	return "knowledge_chunks"
}

// KnowledgeCategory 知识分类常量
const (
	CategoryPerson  = "person"  // 人物
	CategoryEvent   = "event"   // 事件
	CategoryBattle  = "battle"  // 战役
	CategorySchool  = "school"  // 思想流派
	CategoryState   = "state"   // 诸侯国
	CategoryCulture = "culture" // 制度文化
)

// Period 时期常量
const (
	PeriodSpringAutumn = "春秋" // 公元前770年-公元前476年
	PeriodWarringStates = "战国" // 公元前475年-公元前221年
)
