package models

import (
	"time"

	"gorm.io/gorm"
)

// KGNode 知识图谱节点
type KGNode struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"type:varchar(100);uniqueIndex:idx_name_type;not null" json:"name"`
	Type        string         `gorm:"type:varchar(50);uniqueIndex:idx_name_type;not null" json:"type"` // person, state, event, battle, school, concept
	Description string         `gorm:"type:text" json:"description"`
	Properties  string         `gorm:"type:json" json:"properties"` // 额外属性 JSON
	YearStart   int            `gorm:"default:0" json:"year_start"`
	YearEnd     int            `gorm:"default:0" json:"year_end"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// KGEdge 知识图谱边（关系）
type KGEdge struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	FromNodeID   uint           `gorm:"index:idx_from;not null" json:"from_node_id"`
	ToNodeID     uint           `gorm:"index:idx_to;not null" json:"to_node_id"`
	RelationType string         `gorm:"type:varchar(50);not null" json:"relation_type"`
	Description  string         `gorm:"type:text" json:"description"`
	Properties   string         `gorm:"type:json" json:"properties"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (KGNode) TableName() string {
	return "kg_nodes"
}

func (KGEdge) TableName() string {
	return "kg_edges"
}

// NodeType 节点类型常量
const (
	NodeTypePerson  = "person"  // 人物
	NodeTypeState   = "state"   // 国家
	NodeTypeEvent   = "event"   // 事件
	NodeTypeBattle  = "battle"  // 战役
	NodeTypeSchool  = "school"  // 思想流派
	NodeTypeConcept = "concept" // 概念
)

// RelationType 关系类型常量
const (
	RelationBelongsTo    = "BELONGS_TO"    // 属于（人物-国家）
	RelationFounded      = "FOUNDED"       // 创立（人物-学派）
	RelationParticipated = "PARTICIPATED"  // 参与（人物-事件/战役）
	RelationDefeated     = "DEFEATED"      // 击败（国家-国家/人物）
	RelationAllied       = "ALLIED"        // 结盟
	RelationPreceded     = "PRECEDED"      // 前序事件
	RelationCaused       = "CAUSED"        // 导致
	RelationInfluenced   = "INFLUENCED"    // 影响
	RelationOpposed      = "OPPOSED"       // 对立
	RelationTaught       = "TAUGHT"        // 教导
	RelationServed       = "SERVED"        // 仕于
)
