package models

import (
	"time"

	"gorm.io/gorm"
)

// --- 工作流状态常量 ---
const (
	WFStatusInit              = "init"               // 初始化
	WFStatusOutlineGenerating = "outline_generating"  // 正在生成大纲
	WFStatusOutlineReady      = "outline_ready"       // 大纲就绪（可编辑/确认）
	WFStatusOutlineEditing    = "outline_editing"     // 正在修改大纲
	WFStatusPlanGenerating    = "plan_generating"     // 正在生成教案
	WFStatusPlanReady         = "plan_ready"          // 教案就绪
	WFStatusPPTGenerating     = "ppt_generating"      // 正在生成 PPT
	WFStatusDone              = "done"                // 全部完成
	WFStatusFailed            = "failed"              // 失败
)

// LessonWorkflow 备课工作流
type LessonWorkflow struct {
	ID          string         `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserID      uint           `gorm:"index" json:"user_id"`
	Status      string         `gorm:"type:varchar(32);default:'init'" json:"status"`
	Topic       string         `gorm:"type:varchar(255)" json:"topic"`
	Subject     string         `gorm:"type:varchar(50);default:'历史'" json:"subject"`
	Grade       string         `gorm:"type:varchar(50)" json:"grade"`
	Duration    int            `gorm:"default:45" json:"duration"`           // 课时（分钟）
	OutlineJSON string         `gorm:"type:json" json:"outline_json"`       // 大纲 JSON
	LessonPlan  string         `gorm:"type:longtext" json:"lesson_plan"`    // 教案 Markdown
	PPTPath     string         `gorm:"type:varchar(500)" json:"ppt_path"`   // PPT 文件路径
	ErrorMsg    string         `gorm:"type:text" json:"error_msg,omitempty"`
	RetryCount  int            `gorm:"default:0" json:"retry_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (LessonWorkflow) TableName() string {
	return "lesson_workflows"
}

// WorkflowMessage 工作流对话消息（记录每个阶段的交互）
type WorkflowMessage struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkflowID string    `gorm:"index;type:varchar(64)" json:"workflow_id"`
	Stage      string    `gorm:"type:varchar(32)" json:"stage"` // outline, plan, ppt
	Role       string    `gorm:"type:varchar(20)" json:"role"`  // user, assistant, system
	Content    string    `gorm:"type:longtext" json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

func (WorkflowMessage) TableName() string {
	return "workflow_messages"
}

// --- 大纲结构定义（JSON 序列化用） ---

// CourseOutline 课程大纲
type CourseOutline struct {
	Title           string          `json:"title"`
	Subject         string          `json:"subject"`
	Grade           string          `json:"grade"`
	Duration        int             `json:"duration"`
	Objectives      TeachObjectives `json:"objectives"`
	KeyPoints       []string        `json:"key_points"`
	DifficultPoints []string        `json:"difficult_points"`
	Methods         []string        `json:"teaching_methods"`
	Process         []TeachProcess  `json:"process"`
	BoardDesign     string          `json:"board_design"`
}

// TeachObjectives 教学目标（三维目标）
type TeachObjectives struct {
	Knowledge string `json:"knowledge"` // 知识与技能
	Ability   string `json:"ability"`   // 过程与方法
	Emotion   string `json:"emotion"`   // 情感态度价值观
}

// TeachProcess 教学环节
type TeachProcess struct {
	Stage    string `json:"stage"`    // 导入、新授、练习、小结、作业
	Duration int    `json:"duration"` // 分钟
	Activity string `json:"activity"` // 教学活动描述
	Method   string `json:"method"`   // 教学方法
}

// --- PPT 结构定义 ---

// PPTStructure PPT 整体结构
type PPTStructure struct {
	Slides []PPTSlideData `json:"slides"`
}

// PPTSlideData 单页幻灯片数据
type PPTSlideData struct {
	Type     string   `json:"type"`      // title, content, interactive, summary
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle,omitempty"`
	Bullets  []string `json:"bullets,omitempty"`
	Notes    string   `json:"notes,omitempty"` // 演讲备注
}
