package models

import (
	"time"

	"gorm.io/gorm"
)

// --- 数字人视频任务状态常量 ---
const (
	VideoStatusPending    = "pending"    // 等待处理
	VideoStatusProcessing = "processing" // 生成中
	VideoStatusCompleted  = "completed"  // 生成完成
	VideoStatusFailed     = "failed"     // 生成失败
	VideoStatusCancelled  = "cancelled"  // 已取消
)

// --- API 提供商常量 ---
const (
	APIProviderHeyGen     = "heygen"    // HeyGen 商业 API
	APIProviderSadTalker  = "sadtalker" // SadTalker 开源 API
	APIProviderCustom     = "custom"    // 自定义模型 API（ML 团队）
)

// DigitalHumanVideo 数字人视频生成任务
type DigitalHumanVideo struct {
	ID          string         `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserID      uint           `gorm:"index" json:"user_id"`
	Title       string         `gorm:"type:varchar(255)" json:"title"`
	Text        string         `gorm:"type:longtext" json:"text"`            // 驱动文本（TTS 输入）
	AvatarID    string         `gorm:"type:varchar(128)" json:"avatar_id"`   // 数字人形象 ID
	VoiceID     string         `gorm:"type:varchar(128)" json:"voice_id"`    // 音色 ID（TTS）
	Language    string         `gorm:"type:varchar(20);default:'zh-CN'" json:"language"`
	Provider    string         `gorm:"type:varchar(32);default:'heygen'" json:"provider"` // API 提供商
	ExternalID  string         `gorm:"type:varchar(128);index" json:"external_id"`        // 外部 API 返回的任务 ID
	Status      string         `gorm:"type:varchar(32);default:'pending'" json:"status"`
	VideoURL    string         `gorm:"type:varchar(1024)" json:"video_url"`  // 成品视频 URL
	LocalPath   string         `gorm:"type:varchar(512)" json:"local_path"`  // 本地保存路径（可选）
	Duration    float64        `json:"duration"`                              // 视频时长（秒）
	Resolution  string         `gorm:"type:varchar(32)" json:"resolution"`   // 分辨率，如 "1280x720"
	FileSize    int64          `json:"file_size"`                             // 文件大小（字节）
	ErrorMsg    string         `gorm:"type:text" json:"error_msg,omitempty"`
	RetryCount  int            `gorm:"default:0" json:"retry_count"`
	MaxRetry    int            `gorm:"default:3" json:"max_retry"`
	CallbackURL string         `gorm:"type:varchar(512)" json:"callback_url,omitempty"`
	Metadata    string         `gorm:"type:json" json:"metadata,omitempty"`  // 额外元数据（JSON）
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (DigitalHumanVideo) TableName() string {
	return "digital_human_videos"
}

// DigitalHumanAvatar 预设数字人形象
type DigitalHumanAvatar struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AvatarID    string    `gorm:"type:varchar(128);uniqueIndex" json:"avatar_id"`
	Name        string    `gorm:"type:varchar(100)" json:"name"`
	Description string    `gorm:"type:varchar(500)" json:"description"`
	PreviewURL  string    `gorm:"type:varchar(1024)" json:"preview_url"`
	Provider    string    `gorm:"type:varchar(32)" json:"provider"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (DigitalHumanAvatar) TableName() string {
	return "digital_human_avatars"
}
